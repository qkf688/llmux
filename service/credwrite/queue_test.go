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

	var cool, auth, touch, recover atomic.Int32
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
	SetHandlers(Handlers{
		TouchProbe: func(ctx context.Context, credID uint, at time.Time) error {
			touch.Add(1)
			return nil
		},
		Recover: func(ctx context.Context, credID uint, at time.Time) (bool, error) {
			recover.Add(1)
			return true, nil
		},
	})

	now := time.Now()
	processJob(context.Background(), Job{Kind: KindCooldown, CredID: 1, Reason: "x"})
	processJob(context.Background(), Job{Kind: KindAuthFail, CredID: 2})
	processJob(context.Background(), Job{Kind: KindTouchProbe, CredID: 3, At: now})
	processJob(context.Background(), Job{Kind: KindRecover, CredID: 4, At: now})
	if cool.Load() != 1 || auth.Load() != 1 || touch.Load() != 1 || recover.Load() != 1 {
		t.Fatalf("cool=%d auth=%d touch=%d recover=%d, want all 1 (merge kept cooldown/auth)",
			cool.Load(), auth.Load(), touch.Load(), recover.Load())
	}
}

func TestEnqueueTouchProbeAndRecover(t *testing.T) {
	ReplaceHandlersForTest(Handlers{})
	t.Cleanup(func() {
		ReplaceHandlersForTest(Handlers{})
		EnableTestSyncMode(true)
		ResetDroppedForTest()
	})

	var touchID, recoverID uint
	var touchAt, recoverAt time.Time
	var recoverFalse atomic.Bool
	SetHandlers(Handlers{
		TouchProbe: func(ctx context.Context, credID uint, at time.Time) error {
			touchID = credID
			touchAt = at
			return nil
		},
		Recover: func(ctx context.Context, credID uint, at time.Time) (bool, error) {
			recoverID = credID
			recoverAt = at
			if recoverFalse.Load() {
				return false, nil
			}
			return true, nil
		},
	})

	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	// Recover 始终同步：关 syncMode 也立刻拿到真实结果。
	EnableTestSyncMode(false)
	if !EnqueueRecover(10, now.Add(time.Second)) {
		t.Fatal("EnqueueRecover = false, want true（同步落库）")
	}
	if recoverID != 10 || !recoverAt.Equal(now.Add(time.Second)) {
		t.Fatalf("recover = %d %v, want 10 %v", recoverID, recoverAt, now.Add(time.Second))
	}
	recoverFalse.Store(true)
	if EnqueueRecover(11, now) {
		t.Fatal("EnqueueRecover = true, want false（handler 返回未恢复）")
	}

	// Touch 仍走异步队列。
	EnableTestSyncMode(false)
	Start()
	EnqueueTouchProbe(9, now)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if touchID != 9 || !touchAt.Equal(now) {
		t.Fatalf("touch = %d %v, want 9 %v", touchID, touchAt, now)
	}
}
