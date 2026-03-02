package healthcheck

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func (h *HealthChecker) checkAll() {
	ctx := context.Background()
	modelProviders, checkDisabledOnly, err := h.listModelProvidersForCheck(ctx)
	if err != nil {
		slog.Error("failed to get model providers for health check", "error", err)
		return
	}

	slog.Info("starting health check", "count", len(modelProviders), "disabled_only", checkDisabledOnly)
	for _, mp := range modelProviders {
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
		h.checkOneWithBatch(ctx, &mp, batchID)
	}
	slog.Info("batch health check completed", "batch_id", batchID)

	return nil
}

func (h *HealthChecker) listModelProvidersForCheck(ctx context.Context) ([]models.ModelWithProvider, bool, error) {
	checkDisabledOnly := h.getCheckDisabledOnly(ctx)

	var (
		modelProviders []models.ModelWithProvider
		err            error
	)

	if checkDisabledOnly {
		falseVal := false
		modelProviders, err = gorm.G[models.ModelWithProvider](models.DB).Where("status = ?", &falseVal).Find(ctx)
	} else {
		modelProviders, err = gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	}

	return modelProviders, checkDisabledOnly, err
}
