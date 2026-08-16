package healthcheck

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
)

// HealthChecker 健康检测服务。
type HealthChecker struct {
	// baseCtx 为 Start 传入的进程级 ctx，Restart 据此派生新的运行 ctx。
	// 保存它是为了堵住 Restart 曾用 context.Background() 派生、使 checker 脱离
	// 进程级取消的漏点：重启后仍受同一个信号 ctx 约束。
	baseCtx  context.Context
	ctx      context.Context
	cancel   context.CancelFunc
	ticker   *time.Ticker
	mu       sync.RWMutex
	running  bool
	interval time.Duration
}

var (
	healthChecker     *HealthChecker
	healthCheckerOnce sync.Once
)

// GetHealthChecker 获取健康检测单例。
func GetHealthChecker() *HealthChecker {
	healthCheckerOnce.Do(func() {
		healthChecker = &HealthChecker{}
	})
	return healthChecker
}

// Start 启动健康检测服务。ctx 为进程级 ctx，其取消即本服务的退出信号。
func (h *HealthChecker) Start(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 无论是否真正启动都先记下 baseCtx：Start 因「未启用」提前返回时，
	// 后续设置变更触发的 Restart 仍需据此派生，不能退化成 Background。
	h.baseCtx = ctx

	if h.running {
		slog.Info("health checker already running")
		return
	}

	if !h.isEnabled(ctx) {
		slog.Info("health check is disabled")
		return
	}

	h.interval = h.getInterval(ctx)
	h.ctx, h.cancel = context.WithCancel(ctx)
	h.ticker = time.NewTicker(h.interval)
	h.running = true

	// 经 bgtask 登记：进程关闭时 baseCtx 取消使 run 退出，Shutdown 等到它停干净，
	// 不会在一轮检测写库中途被硬切。
	runCtx, tick := h.ctx, h.ticker.C
	bgtask.Go(func(context.Context) { h.run(runCtx, tick) })
	slog.Info("health checker started", "interval", h.interval)
}

// Stop 停止健康检测服务。
func (h *HealthChecker) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	if h.cancel != nil {
		h.cancel()
	}
	if h.ticker != nil {
		h.ticker.Stop()
	}

	h.running = false
	slog.Info("health checker stopped")
}

// Restart 重启健康检测服务（配置变更时调用）。沿用 Start 记录的进程级 baseCtx，
// 使重启后的 checker 仍受进程级取消约束，不脱离优雅关闭。
func (h *HealthChecker) Restart() {
	h.Stop()

	h.mu.RLock()
	base := h.baseCtx
	h.mu.RUnlock()
	if base == nil {
		// 从未 Start 过就 Restart 属调用方错误：无进程级 ctx 可继承，退回不启动而非
		// 用 Background 静默脱管。
		slog.Warn("health checker restart skipped: never started")
		return
	}
	h.Start(base)
}

// IsRunning 检查是否正在运行。
func (h *HealthChecker) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

// run 为检测主循环。ctx 与 tick 由 Start 在锁内捕获后传入，避免循环期间再去读
// 会被 Restart 并发替换的 h.ctx / h.ticker 字段。
func (h *HealthChecker) run(ctx context.Context, tick <-chan time.Time) {
	h.checkAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			if !h.isEnabled(ctx) {
				slog.Info("health check disabled, stopping checker")
				h.Stop()
				return
			}

			h.updateIntervalIfNeeded(ctx)
			h.checkAll(ctx)
		}
	}
}

func (h *HealthChecker) updateIntervalIfNeeded(ctx context.Context) {
	newInterval := h.getInterval(ctx)

	h.mu.RLock()
	current := h.interval
	h.mu.RUnlock()

	if newInterval == current {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.ticker == nil {
		h.interval = newInterval
		return
	}
	if h.interval == newInterval {
		return
	}

	h.interval = newInterval
	h.ticker.Reset(newInterval)
	slog.Info("health check interval updated", "interval", newInterval)
}
