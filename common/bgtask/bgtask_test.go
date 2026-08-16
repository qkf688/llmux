package bgtask

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestManagerGoWaitsForTask 核心契约：Shutdown 必须等到任务跑完才返回。
// 这是整个包存在的理由——写库任务不排空就等于丢数据。
func TestManagerGoWaitsForTask(t *testing.T) {
	m := NewManager()

	var done atomic.Bool
	m.Go(func(context.Context) {
		time.Sleep(50 * time.Millisecond)
		done.Store(true)
	})

	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	if !done.Load() {
		t.Fatal("Shutdown returned before task finished")
	}
}

// TestManagerTaskCtxNotCanceledOnShutdown 任务 ctx 必须**不随** Shutdown 取消。
// 取消会把 SaveChatLog / 权重衰减打断，比不排空丢得更多。
func TestManagerTaskCtxNotCanceledOnShutdown(t *testing.T) {
	m := NewManager()

	started := make(chan struct{})
	var canceled atomic.Bool
	m.Go(func(ctx context.Context) {
		close(started)
		// Shutdown 已在等待本任务期间：此刻 ctx 若被取消即违反契约。
		time.Sleep(30 * time.Millisecond)
		canceled.Store(ctx.Err() != nil)
	})

	<-started
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	if canceled.Load() {
		t.Fatal("task ctx was canceled during shutdown; drain tasks must not be interrupted")
	}
}

// TestManagerWaitsForCtxDrivenLoop 长驻循环由**自身** ctx 决定何时停，
// Manager 只保证它停干净了——这是 healthcheck / modelsync 的接入方式。
func TestManagerWaitsForCtxDrivenLoop(t *testing.T) {
	m := NewManager()

	loopCtx, stopLoop := context.WithCancel(context.Background())
	exited := make(chan struct{})
	m.Go(func(context.Context) {
		<-loopCtx.Done()
		close(exited)
	})

	// 模拟 main 的关闭序：先取消信号 ctx，再等 Manager 排空。
	stopLoop()
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("Shutdown returned before ctx-driven loop exited")
	}
}

// TestManagerShutdownTimeout 卡住的任务不得让关闭无限挂起：超时须返回错误并放行，
// 让 main 能继续走完后续关闭步骤（关数据库等）。
func TestManagerShutdownTimeout(t *testing.T) {
	m := NewManager()

	release := make(chan struct{})
	defer close(release) // 让卡住的 goroutine 在测试结束时退出，避免泄漏到其它用例
	m.Go(func(context.Context) { <-release })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := m.Shutdown(ctx)
	if err == nil {
		t.Fatal("Shutdown() = nil, want timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown() = %v, want wrapped context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Shutdown blocked for %v, want to return near ctx deadline", elapsed)
	}
}

// TestManagerGoAfterShutdown Shutdown 之后再登记必须安全丢弃：此时 Wait 已过、数据库即将关闭，
// 放它跑只会撞上 "database is closed" 之类的误导性错误。丢弃且可见，不 panic。
func TestManagerGoAfterShutdown(t *testing.T) {
	m := NewManager()
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}

	var ran atomic.Bool
	m.Go(func(context.Context) { ran.Store(true) })

	// 给被错误启动的 goroutine 一个暴露自己的窗口。
	time.Sleep(30 * time.Millisecond)
	if ran.Load() {
		t.Fatal("task started after shutdown; want discarded")
	}
}

// TestManagerShutdownIdempotent 关闭序里任何一步失败都不 early-return，Shutdown 可能被重复触达。
func TestManagerShutdownIdempotent(t *testing.T) {
	m := NewManager()
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("first Shutdown() = %v, want nil", err)
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown() = %v, want nil", err)
	}
}

// TestManagerConcurrentGo 调用点分散在 handler / service 两层，并发登记是常态。
func TestManagerConcurrentGo(t *testing.T) {
	m := NewManager()

	const n = 100
	var count atomic.Int64
	for range n {
		go m.Go(func(context.Context) { count.Add(1) })
	}

	// 并发 Go 与 Shutdown 竞争时，只保证「已登记的必被等到」，
	// 故此处先给登记留出窗口，再断言无一丢失。
	time.Sleep(50 * time.Millisecond)
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	if got := count.Load(); got != n {
		t.Fatalf("completed tasks = %d, want %d", got, n)
	}
}

// TestDefaultLazyInit 业务包（service/chat 等）的既有测试不会调 SetDefault，
// 裸用包级 Go 必须能工作，否则一装配就炸一片测试。
func TestDefaultLazyInit(t *testing.T) {
	t.Cleanup(func() { SetDefault(NewManager()) })
	SetDefault(nil)

	if Default() == nil {
		t.Fatal("Default() = nil, want lazily created manager")
	}

	var ran atomic.Bool
	Go(func(context.Context) { ran.Store(true) })

	if err := Default().Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	if !ran.Load() {
		t.Fatal("package-level Go task did not run")
	}
}

// TestSetDefaultReplaces main 装配时注入的 Manager 必须是包级函数实际使用的那一个。
func TestSetDefaultReplaces(t *testing.T) {
	t.Cleanup(func() { SetDefault(NewManager()) })

	m := NewManager()
	SetDefault(m)
	if Default() != m {
		t.Fatal("Default() did not return the injected manager")
	}

	var ran atomic.Bool
	Go(func(context.Context) { ran.Store(true) })
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}
	if !ran.Load() {
		t.Fatal("package-level Go did not dispatch to the injected manager")
	}
}
