package credwrite

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestQueue_DropOldestWhenFull(t *testing.T) {
	ResetDroppedForTest()
	block := make(chan struct{})
	var closeBlock sync.Once
	unblock := func() { closeBlock.Do(func() { close(block) }) }
	t.Cleanup(func() {
		unblock()
		ReplaceHandlersForTest(Handlers{})
		ResetDroppedForTest()
	})

	entered := make(chan struct{})
	var enteredOnce sync.Once
	var processed atomic.Int32
	ReplaceHandlersForTest(Handlers{
		Cooldown: func(ctx context.Context, credID uint, reason string) error {
			enteredOnce.Do(func() { close(entered) })
			processed.Add(1)
			<-block
			return nil
		},
	})

	q := NewQueue(2)
	go q.runWorker()
	t.Cleanup(q.requestStop)

	// 先让 worker 拿走 1 条并阻塞，再精确填满容量 2，避免时序竞态。
	q.enqueueDropOldest(Job{Kind: KindCooldown, CredID: 1, Reason: "a"})
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not pick first job")
	}

	q.enqueueDropOldest(Job{Kind: KindCooldown, CredID: 2, Reason: "b"})
	q.enqueueDropOldest(Job{Kind: KindCooldown, CredID: 3, Reason: "c"})
	// 满：再入 4 应丢最旧 2，保留 3+4；worker 持有 1 → 最终处理 1,3,4
	q.enqueueDropOldest(Job{Kind: KindCooldown, CredID: 4, Reason: "d"})

	if got := DroppedCount(); got != 1 {
		t.Fatalf("DroppedCount = %d, want 1", got)
	}

	unblock()
	deadline := time.After(2 * time.Second)
	for processed.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("processed = %d, want 3", processed.Load())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TestQueue_FlushDoesNotDropSentinel(t *testing.T) {
	ResetDroppedForTest()
	block := make(chan struct{})
	var closeBlock sync.Once
	unblock := func() { closeBlock.Do(func() { close(block) }) }
	t.Cleanup(func() {
		unblock()
		ReplaceHandlersForTest(Handlers{})
		ResetDroppedForTest()
	})

	ReplaceHandlersForTest(Handlers{
		Cooldown: func(ctx context.Context, credID uint, reason string) error {
			<-block
			return nil
		},
	})

	q := NewQueue(1)
	go q.runWorker()
	t.Cleanup(q.requestStop)

	q.enqueueDropOldest(Job{Kind: KindCooldown, CredID: 1, Reason: "a"})
	// 等 worker 卡住后再塞满缓冲
	time.Sleep(20 * time.Millisecond)
	q.enqueueDropOldest(Job{Kind: KindCooldown, CredID: 2, Reason: "b"})

	done := make(chan struct{})
	q.enqueueFlush(Job{Kind: kindFlush, flushDone: done})

	// 并发 drop 不得吃掉 flush 哨兵
	q.enqueueDropOldest(Job{Kind: KindCooldown, CredID: 3, Reason: "c"})

	unblock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Flush sentinel was dropped or never processed")
	}
}

func TestQueue_FlushDrainsJobs(t *testing.T) {
	ResetDroppedForTest()
	var seen atomic.Int32
	ReplaceHandlersForTest(Handlers{
		Cooldown: func(ctx context.Context, credID uint, reason string) error {
			seen.Add(1)
			return nil
		},
	})
	t.Cleanup(func() { ReplaceHandlersForTest(Handlers{}) })

	q := NewQueue(8)
	go q.runWorker()
	t.Cleanup(q.requestStop)

	q.enqueueDropOldest(Job{Kind: KindCooldown, CredID: 10, Reason: "http_429"})
	q.enqueueDropOldest(Job{Kind: KindCooldown, CredID: 11, Reason: "http_500"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := q.flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if seen.Load() != 2 {
		t.Fatalf("processed = %d, want 2", seen.Load())
	}
}

func TestQueue_StopAfterFlush(t *testing.T) {
	ResetDroppedForTest()
	var seen atomic.Int32
	ReplaceHandlersForTest(Handlers{
		Cooldown: func(ctx context.Context, credID uint, reason string) error {
			seen.Add(1)
			return nil
		},
	})
	t.Cleanup(func() { ReplaceHandlersForTest(Handlers{}) })

	q := NewQueue(4)
	go q.runWorker()

	q.enqueueDropOldest(Job{Kind: KindCooldown, CredID: 99, Reason: "http_429"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := q.flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	q.requestStop()
	deadline := time.After(time.Second)
	for seen.Load() < 1 {
		select {
		case <-deadline:
			t.Fatalf("processed = %d, want 1", seen.Load())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TestSetHandlers_MergesNonNilFields(t *testing.T) {
	ReplaceHandlersForTest(Handlers{})
	t.Cleanup(func() { ReplaceHandlersForTest(Handlers{}) })

	var cool, auth atomic.Int32
	SetHandlers(Handlers{
		Cooldown: func(ctx context.Context, credID uint, reason string) error {
			cool.Add(1)
			return nil
		},
	})
	SetHandlers(Handlers{
		AuthFail: func(ctx context.Context, credID uint) error {
			auth.Add(1)
			return nil
		},
	})

	processJob(context.Background(), Job{Kind: KindCooldown, CredID: 1, Reason: "x"})
	processJob(context.Background(), Job{Kind: KindAuthFail, CredID: 2})
	if cool.Load() != 1 || auth.Load() != 1 {
		t.Fatalf("cool=%d auth=%d, want both 1 (merge kept cooldown)", cool.Load(), auth.Load())
	}
}
