package healthcheck

import (
	"context"
	"log/slog"

	"github.com/qkf688/llmux/models"
)

// checkAll 执行一轮定时检测。ctx 由 run 传入（派生自进程级 ctx），使整轮检测在
// 进程关闭时可被取消——此前自建 context.Background() 让一轮检测完全没有取消感知。
func (h *HealthChecker) checkAll(ctx context.Context) {
	modelProviders, checkDisabledOnly, err := h.listModelProvidersForCheck(ctx)
	if err != nil {
		slog.Error("failed to get model providers for health check", "error", err)
		return
	}

	slog.Info("starting health check", "count", len(modelProviders), "disabled_only", checkDisabledOnly)
	for _, mp := range modelProviders {
		if ctx.Err() != nil {
			slog.Info("health check interrupted", "reason", ctx.Err())
			return
		}
		h.checkOne(ctx, &mp)
	}
	slog.Info("health check completed")
}

// CheckAllWithBatch 批量检测所有模型提供商（手动触发，带 batchID）。
func (h *HealthChecker) CheckAllWithBatch(ctx context.Context, batchID string) error {
	modelProviders, checkDisabledOnly, err := h.listModelProvidersForCheck(ctx)
	if err != nil {
		slog.Error("failed to get model providers for batch health check", "error", err, "batch_id", batchID)
		return err
	}

	slog.Info("starting batch health check", "count", len(modelProviders), "batch_id", batchID, "disabled_only", checkDisabledOnly)
	for _, mp := range modelProviders {
		// 与 checkAll 一致：ctx 取消即中止整批，不把剩余候选继续打到上游。
		if ctx.Err() != nil {
			slog.Info("batch health check interrupted", "reason", ctx.Err(), "batch_id", batchID)
			return ctx.Err()
		}
		h.checkOneWithBatch(ctx, &mp, batchID)
	}
	slog.Info("batch health check completed", "batch_id", batchID)

	return nil
}

func (h *HealthChecker) listModelProvidersForCheck(ctx context.Context) ([]models.ModelWithProvider, bool, error) {
	checkDisabledOnly := h.getCheckDisabledOnly(ctx)

	var status *bool
	if checkDisabledOnly {
		falseVal := false
		status = &falseVal
	}

	modelProviders, err := repos().ModelWithProvider.ListByStatus(ctx, status)
	return modelProviders, checkDisabledOnly, err
}
