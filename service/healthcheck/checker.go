package healthcheck

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// HealthChecker 健康检测服务。
type HealthChecker struct {
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

// Start 启动健康检测服务。
func (h *HealthChecker) Start(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

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

	go h.run()
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

// Restart 重启健康检测服务（配置变更时调用）。
func (h *HealthChecker) Restart(ctx context.Context) {
	h.Stop()
	h.Start(ctx)
}

// IsRunning 检查是否正在运行。
func (h *HealthChecker) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

func (h *HealthChecker) run() {
	h.checkAll()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.ticker.C:
			if !h.isEnabled(h.ctx) {
				slog.Info("health check disabled, stopping checker")
				h.Stop()
				return
			}

			h.updateIntervalIfNeeded(h.ctx)
			h.checkAll()
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
