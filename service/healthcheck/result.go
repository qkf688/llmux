package healthcheck

import (
	"context"
	"log/slog"
	"sync"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// AdjustmentHooks 注入健康检测后对权重/优先级的调整逻辑。
type AdjustmentHooks struct {
	ShouldCountHealthCheckSuccess func(ctx context.Context) bool
	ShouldCountHealthCheckFailure func(ctx context.Context) bool
	ApplySuccessAdjustments       func(ctx context.Context, modelProviderID uint)
	ApplyWeightDecay              func(ctx context.Context, modelProviderID uint, providerName, providerModel string)
	ApplyPriorityDecay            func(ctx context.Context, modelProviderID uint, providerName, providerModel string)
}

var (
	adjustmentHooks   AdjustmentHooks
	adjustmentHooksMu sync.RWMutex
)

// SetAdjustmentHooks 配置健康检测结果处理的扩展 hook。
func SetAdjustmentHooks(hooks AdjustmentHooks) {
	adjustmentHooksMu.Lock()
	adjustmentHooks = hooks
	adjustmentHooksMu.Unlock()
}

func (h *HealthChecker) handleCheckResult(ctx context.Context, mp *models.ModelWithProvider, providerName string, success bool) {
	failureThreshold := h.getFailureThreshold(ctx)
	autoEnable := h.getAutoEnable(ctx)
	failureDisableEnabled := h.getFailureDisableEnabled(ctx)

	if success {
		if shouldCountHealthCheckSuccess(ctx) {
			applySuccessAdjustments(ctx, mp.ID)
		}

		if autoEnable && (mp.Status == nil || !*mp.Status) {
			trueVal := true
			if _, err := gorm.G[models.ModelWithProvider](models.DB).
				Where("id = ?", mp.ID).
				Updates(ctx, models.ModelWithProvider{Status: &trueVal, ConsecutiveFailures: 0}); err != nil {
				slog.Error("failed to enable model provider after health check success", "id", mp.ID, "error", err)
			} else {
				slog.Info("model provider auto-enabled after health check success", "id", mp.ID)
			}
		}
		return
	}

	failCount, err := h.getConsecutiveFailures(ctx, mp.ID)
	if err != nil {
		slog.Error("failed to get consecutive failures", "id", mp.ID, "error", err)
		return
	}

	if shouldCountHealthCheckFailure(ctx) {
		applyWeightDecay(ctx, mp.ID, providerName, mp.ProviderModel)
		applyPriorityDecay(ctx, mp.ID, providerName, mp.ProviderModel)
	}

	if failureDisableEnabled && failCount >= failureThreshold && (mp.Status == nil || *mp.Status) {
		falseVal := false
		if _, err := gorm.G[models.ModelWithProvider](models.DB).
			Where("id = ?", mp.ID).
			Updates(ctx, models.ModelWithProvider{Status: &falseVal}); err != nil {
			slog.Error("failed to disable model provider after health check failures", "id", mp.ID, "error", err)
		} else {
			slog.Warn("model provider auto-disabled after health check failures", "id", mp.ID, "fail_count", failCount)
		}
	}
}

func (h *HealthChecker) getConsecutiveFailures(ctx context.Context, mpID uint) (int, error) {
	logs, err := gorm.G[models.HealthCheckLog](models.DB).
		Where("model_provider_id = ?", mpID).
		Order("checked_at DESC").
		Limit(10).
		Find(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, log := range logs {
		if log.Status != "error" {
			break
		}
		count++
	}
	return count, nil
}

func shouldCountHealthCheckSuccess(ctx context.Context) bool {
	hooks := getAdjustmentHooks()
	if hooks.ShouldCountHealthCheckSuccess == nil {
		return true
	}
	return hooks.ShouldCountHealthCheckSuccess(ctx)
}

func shouldCountHealthCheckFailure(ctx context.Context) bool {
	hooks := getAdjustmentHooks()
	if hooks.ShouldCountHealthCheckFailure == nil {
		return false
	}
	return hooks.ShouldCountHealthCheckFailure(ctx)
}

func applySuccessAdjustments(ctx context.Context, modelProviderID uint) {
	hooks := getAdjustmentHooks()
	if hooks.ApplySuccessAdjustments == nil {
		return
	}
	hooks.ApplySuccessAdjustments(ctx, modelProviderID)
}

func applyWeightDecay(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	hooks := getAdjustmentHooks()
	if hooks.ApplyWeightDecay == nil {
		return
	}
	hooks.ApplyWeightDecay(ctx, modelProviderID, providerName, providerModel)
}

func applyPriorityDecay(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	hooks := getAdjustmentHooks()
	if hooks.ApplyPriorityDecay == nil {
		return
	}
	hooks.ApplyPriorityDecay(ctx, modelProviderID, providerName, providerModel)
}

func getAdjustmentHooks() AdjustmentHooks {
	adjustmentHooksMu.RLock()
	defer adjustmentHooksMu.RUnlock()
	return adjustmentHooks
}
