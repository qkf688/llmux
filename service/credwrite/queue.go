package credwrite

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
)

const defaultQueueCapacity = 4096

// Kind 凭据健康异步写任务类型（#6-4-2 扩展 probe/recovery）。
type Kind int

const (
	KindCooldown Kind = iota
	KindAuthFail
	KindTouchProbe
	KindRecover
	kindFlush // 内部：Flush 哨兵，不落库、不可被 drop-oldest 丢掉
)

// Job 单条凭据写任务；payload 尽量小，窗口/阈值在 worker 侧读设置。
type Job struct {
	Kind      Kind
	CredID    uint
	Reason    string    // KindCooldown 时：冷却 reason 机器码
	At        time.Time // KindTouchProbe / KindRecover：探活记账时间
	flushDone chan struct{}
}

// Handlers worker 落库回调，由 chat / credprobe 注入（避免 credwrite import 业务包）。
// SetHandlers 按非 nil 字段合并，追加 Probe 时不会清掉已有 Cooldown/AuthFail。
type Handlers struct {
	Cooldown   func(ctx context.Context, credID uint, reason string) error
	AuthFail   func(ctx context.Context, credID uint) error
	TouchProbe func(ctx context.Context, credID uint, at time.Time) error
	Recover    func(ctx context.Context, credID uint, at time.Time) (recovered bool, err error)
}

var (
	handlersMu sync.RWMutex
	handlers   Handlers

	syncMode atomic.Bool
	dropped  atomic.Uint64

	defaultQueue *Queue
)

// Queue 有界凭据写队列；mutex + slice 实现 drop-oldest，单 worker 串行落库。
type Queue struct {
	capacity int

	mu       sync.Mutex
	cond     *sync.Cond
	buf      []Job
	stopping bool

	workerOnce sync.Once
	stopOnce   sync.Once
}

// NewQueue 创建指定容量的队列（测试用小容量触发 drop）。
func NewQueue(capacity int) *Queue {
	if capacity < 1 {
		capacity = 1
	}
	q := &Queue{
		capacity: capacity,
		buf:      make([]Job, 0, capacity),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func init() {
	defaultQueue = NewQueue(defaultQueueCapacity)
}

// SetHandlers 按非 nil 字段合并注入落库实现。
func SetHandlers(h Handlers) {
	handlersMu.Lock()
	defer handlersMu.Unlock()
	if h.Cooldown != nil {
		handlers.Cooldown = h.Cooldown
	}
	if h.AuthFail != nil {
		handlers.AuthFail = h.AuthFail
	}
	if h.TouchProbe != nil {
		handlers.TouchProbe = h.TouchProbe
	}
	if h.Recover != nil {
		handlers.Recover = h.Recover
	}
}

// ReplaceHandlersForTest 测试用整表替换（可清空）；生产路径用 SetHandlers 合并。
func ReplaceHandlersForTest(h Handlers) {
	handlersMu.Lock()
	handlers = h
	handlersMu.Unlock()
}

// EnableTestSyncMode 测试同步直写：Enqueue 不经队列，便于既有单测断言 DB。
func EnableTestSyncMode(on bool) {
	syncMode.Store(on)
}

// DroppedCount 队列满丢弃计数（可观察降级，AC-3）。
func DroppedCount() uint64 {
	return dropped.Load()
}

// ResetDroppedForTest 重置 drop 计数。
func ResetDroppedForTest() {
	dropped.Store(0)
}

// Start 登记长驻 worker（经 bgtask）。关闭时须先 Stop（Flush + 停收），再 bgtask.Shutdown。
func Start() {
	defaultQueue.startWorker()
}

// Stop Flush 后停止 worker：关库前调用，避免 bgtask 等到超时或写已关库。
func Stop(ctx context.Context) error {
	err := Flush(ctx)
	defaultQueue.requestStop()
	return err
}

func (q *Queue) startWorker() {
	q.workerOnce.Do(func() {
		// 忽略 bgtask 的永不取消 taskCtx；退出由 requestStop 驱动（与 healthcheck 同构）。
		bgtask.Go(func(context.Context) { q.runWorker() })
	})
}

// EnqueueCooldown 非阻塞投递冷却写。
func EnqueueCooldown(credID uint, reason string) {
	if credID == 0 {
		return
	}
	defaultQueue.enqueue(Job{Kind: KindCooldown, CredID: credID, Reason: reason})
}

// EnqueueAuthFail 非阻塞投递鉴权失败计数/判停写。
func EnqueueAuthFail(credID uint) {
	if credID == 0 {
		return
	}
	defaultQueue.enqueue(Job{Kind: KindAuthFail, CredID: credID})
}

// EnqueueTouchProbe 非阻塞投递探活失败/恢复未落地时的 LastProbeAt 记账。
func EnqueueTouchProbe(credID uint, at time.Time) {
	if credID == 0 {
		return
	}
	defaultQueue.enqueue(Job{Kind: KindTouchProbe, CredID: credID, At: at})
}

// EnqueueRecover 探活成功后的恢复写。
// 必须与调用方随后的 Assemble/Select happens-before：探活低频，同步落库并返回
// 真实 recovered（条件更新 0 行 → false）。TouchProbe 仍走异步队列。
func EnqueueRecover(credID uint, at time.Time) bool {
	if credID == 0 {
		return false
	}
	return processRecoverJob(context.Background(), credID, at)
}

func processRecoverJob(ctx context.Context, credID uint, at time.Time) bool {
	handlersMu.RLock()
	h := handlers.Recover
	handlersMu.RUnlock()
	if h == nil {
		slog.Warn("credential write queue: recover handler nil", "credential_id", credID)
		return false
	}
	recovered, err := h(ctx, credID, at)
	if err != nil {
		slog.Warn("credential write queue: persist failed",
			"credential_id", credID, "kind", KindRecover, "error", err)
		return false
	}
	return recovered
}

func (q *Queue) enqueue(job Job) {
	if syncMode.Load() {
		processJob(context.Background(), job)
		return
	}
	q.startWorker()
	q.enqueueDropOldest(job)
}

func (q *Queue) enqueueDropOldest(job Job) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.stopping {
		dropped.Add(1)
		slog.Warn("credential write queue stopped, dropped incoming",
			"credential_id", job.CredID, "kind", job.Kind, "dropped_total", dropped.Load())
		return
	}

	if len(q.buf) >= q.capacity {
		if !q.dropOldestDataLocked(job) {
			return
		}
	}
	q.buf = append(q.buf, job)
	q.cond.Signal()
}

// dropOldestDataLocked 丢掉最旧的非 flush 条目；若无法腾位则丢弃 incoming 并计 drop。
// 调用方须持有 q.mu。
func (q *Queue) dropOldestDataLocked(incoming Job) bool {
	idx := -1
	var old Job
	for i, j := range q.buf {
		if j.Kind != kindFlush {
			idx = i
			old = j
			break
		}
	}
	if idx < 0 {
		dropped.Add(1)
		slog.Warn("credential write queue full (flush sentinels only), dropped incoming",
			"credential_id", incoming.CredID, "kind", incoming.Kind, "dropped_total", dropped.Load())
		return false
	}
	q.buf = append(q.buf[:idx], q.buf[idx+1:]...)
	dropped.Add(1)
	slog.Warn("credential write queue full, dropped oldest",
		"dropped_credential_id", old.CredID, "dropped_kind", old.Kind,
		"incoming_credential_id", incoming.CredID, "incoming_kind", incoming.Kind,
		"dropped_total", dropped.Load())
	return true
}

func (q *Queue) enqueueFlush(job Job) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.stopping {
		if job.flushDone != nil {
			close(job.flushDone)
		}
		return
	}

	// Flush 哨兵不可被丢掉：满时优先踢数据 job；若全是哨兵则允许短暂超容量。
	for len(q.buf) >= q.capacity {
		if !q.dropOldestDataLocked(job) {
			break
		}
	}
	q.buf = append(q.buf, job)
	q.cond.Signal()
}

func (q *Queue) runWorker() {
	for {
		job, ok := q.takeJob()
		if !ok {
			return
		}
		processJob(context.Background(), job)
	}
}

func (q *Queue) takeJob() (Job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for {
		if len(q.buf) > 0 {
			job := q.buf[0]
			q.buf = q.buf[1:]
			return job, true
		}
		if q.stopping {
			return Job{}, false
		}
		q.cond.Wait()
	}
}

func (q *Queue) requestStop() {
	q.mu.Lock()
	q.stopping = true
	q.mu.Unlock()
	q.cond.Broadcast()
}

func processJob(ctx context.Context, job Job) {
	if job.Kind == kindFlush {
		if job.flushDone != nil {
			close(job.flushDone)
		}
		return
	}

	handlersMu.RLock()
	h := handlers
	handlersMu.RUnlock()

	var err error
	switch job.Kind {
	case KindCooldown:
		if h.Cooldown == nil {
			slog.Warn("credential write queue: cooldown handler nil", "credential_id", job.CredID)
			return
		}
		err = h.Cooldown(ctx, job.CredID, job.Reason)
	case KindAuthFail:
		if h.AuthFail == nil {
			slog.Warn("credential write queue: auth-fail handler nil", "credential_id", job.CredID)
			return
		}
		err = h.AuthFail(ctx, job.CredID)
	case KindTouchProbe:
		if h.TouchProbe == nil {
			slog.Warn("credential write queue: touch-probe handler nil", "credential_id", job.CredID)
			return
		}
		err = h.TouchProbe(ctx, job.CredID, job.At)
	case KindRecover:
		if h.Recover == nil {
			slog.Warn("credential write queue: recover handler nil", "credential_id", job.CredID)
			return
		}
		_, err = h.Recover(ctx, job.CredID, job.At)
	default:
		slog.Warn("credential write queue: unknown job kind", "kind", job.Kind)
		return
	}
	if err != nil {
		slog.Warn("credential write queue: persist failed",
			"credential_id", job.CredID, "kind", job.Kind, "error", err)
	}
}

// Flush 等待默认队列排空到哨兵（与此前 DB 写 happens-before）。
func Flush(ctx context.Context) error {
	return defaultQueue.flush(ctx)
}

func (q *Queue) flush(ctx context.Context) error {
	if syncMode.Load() {
		return nil
	}
	q.startWorker()
	done := make(chan struct{})
	q.enqueueFlush(Job{Kind: kindFlush, flushDone: done})
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
