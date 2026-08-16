// Package bgtask 登记后台 goroutine，使进程关闭时能统一等待它们退出。
//
// 解决的问题：请求路径里 fire-and-forget 的写库 goroutine（重试日志、权重衰减、
// 日志保留清理、自动关联等）此前一律 `go f(context.Background())` 启动，进程被
// 终止时既无法排空也无从感知，落库与关联调整会被硬切。
package bgtask

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Manager 登记后台 goroutine 并在 Shutdown 时等待它们退出。
//
// 职责**只有**登记与等待，不负责取消：
//
//   - 收尾任务（fire-and-forget 写库）本就不该被取消——取消会把落库与权重衰减
//     打断，比不排空丢得更多，所以传给任务的 ctx 永不取消。
//   - 长驻任务（ticker 循环）的取消由其自身的 ctx 负责（由 main 的信号 ctx 派生）。
//     若在这里再提供一个取消源，同一个 goroutine 就有两个互不知情的取消源。
//
// 因此「何时停」由调用方的 ctx 决定，「停没停干净」由本类型的 Shutdown 保证。
type Manager struct {
	// taskCtx 交给任务，生命周期独立于关闭信号（见类型注释）。
	taskCtx context.Context

	wg sync.WaitGroup

	mu sync.Mutex
	// shutdown 置位后拒绝新任务：此时 Wait 已开始、数据库即将关闭，放新任务跑只会
	// 撞上「数据库已关闭」这类误导性错误。丢弃并记 warn，让问题可见而非隐形。
	shutdown bool
}

// NewManager 创建独立的 Manager。测试直接构造本类型即可，无需触碰包级默认实例。
func NewManager() *Manager {
	return &Manager{taskCtx: context.Background()}
}

// Go 登记一个后台任务：Shutdown 会等它退出，且不会取消传入的 ctx。
//
// 长驻循环类任务应忽略此 ctx，改用自身由信号 ctx 派生的 ctx 作为退出条件，
// 否则 Shutdown 只能等到超时。
func (m *Manager) Go(fn func(ctx context.Context)) {
	m.mu.Lock()
	if m.shutdown {
		m.mu.Unlock()
		slog.Warn("background task discarded: manager already shut down")
		return
	}
	// Add 必须在锁内：shutdown 置位后不再 Add，从而保证 Add 与 Shutdown 里的
	// Wait 不会并发（WaitGroup 不允许计数归零时的 Add 与 Wait 竞争）。
	m.wg.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.wg.Done()
		fn(m.taskCtx)
	}()
}

// Shutdown 等待所有已登记任务退出，并拒绝后续登记。
//
// ctx 到期即返回错误而不继续等待：关闭序的后续步骤（关闭数据库）必须有机会执行，
// 任何一个卡住的任务都不得让进程无限挂起。可重复调用。
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.shutdown = true
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("background tasks not drained before deadline: %w", ctx.Err())
	}
}

var (
	defaultMu      sync.Mutex
	defaultManager *Manager
)

// SetDefault 注入包级 Manager，由 main 在启动装配时调用（与 repository.SetDefault 同构）。
func SetDefault(m *Manager) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultManager = m
}

// Default 返回包级 Manager，未注入时懒建一个。
//
// 懒建而非 panic：业务包的单元测试不走 main 的装配流程，但仍会执行到调用
// bgtask.Go 的代码路径，缺了懒建会让一大片既有测试因空指针失败。
func Default() *Manager {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultManager == nil {
		defaultManager = NewManager()
	}
	return defaultManager
}

// Go 用包级 Manager 登记后台任务。业务侧调用点用这个，不必持有 Manager 句柄。
func Go(fn func(ctx context.Context)) {
	Default().Go(fn)
}
