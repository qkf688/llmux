package healthcheck

import (
	"context"
	"log/slog"
	"sync"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
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

// handleCheckResult 处理一次检测结果。status 取 models.HealthCheckStatus* 三态之一。
//
// 三态分开处置（Stage 36）：通过 → 调整 + 自动启用；上游失败 → 衰减 + 判停；
// 本机没测成 → 什么都不做——它不是上游的结论，不该记在供应商头上（不衰减、不判停）。
func (h *HealthChecker) handleCheckResult(ctx context.Context, mp *models.ModelWithProvider, providerName, status string) {
	switch status {
	case models.HealthCheckStatusSuccess:
		h.handleCheckSuccess(ctx, mp)
	case models.HealthCheckStatusError:
		h.handleCheckFailure(ctx, mp, providerName)
	}
}

func (h *HealthChecker) handleCheckSuccess(ctx context.Context, mp *models.ModelWithProvider) {
	if shouldCountHealthCheckSuccess(ctx) {
		applySuccessAdjustments(ctx, mp.ID)
	}

	// 开关只在成功分支读：失败路径不需要它，少一次读少一处读失败面。
	autoEnable := h.getAutoEnable(ctx)
	if autoEnable && (mp.Status == nil || !*mp.Status) {
		// 必须走 UpdateFields：结构体 Updates 会把 consecutive_failures 的 0 当零值跳过，
		// 计数不清零会让关联恢复后再失败一次就立刻被重新禁用。
		if _, err := repos().ModelWithProvider.UpdateFields(ctx, mp.ID, map[string]any{
			"status":               true,
			"consecutive_failures": 0,
		}); err != nil {
			slog.Error("failed to enable model provider after health check success", "id", mp.ID, "error", err)
		} else {
			slog.Info("model provider auto-enabled after health check success", "id", mp.ID)
		}
	}
}

func (h *HealthChecker) handleCheckFailure(ctx context.Context, mp *models.ModelWithProvider, providerName string) {
	// 判停所需的两项配置先取：读不到就跳过判停步（留痕在 disableDecision 内）。
	failureDisableEnabled, failureThreshold, decisionOK := h.disableDecision(ctx)

	failCount, err := h.getConsecutiveFailures(ctx, mp.ID)
	if err != nil {
		slog.Error("failed to get consecutive failures", "id", mp.ID, "error", err)
		return
	}

	if shouldCountHealthCheckFailure(ctx) {
		applyWeightDecay(ctx, mp.ID, providerName, mp.ProviderModel)
		applyPriorityDecay(ctx, mp.ID, providerName, mp.ProviderModel)
	}

	if !decisionOK {
		return
	}

	if failureDisableEnabled && failCount >= failureThreshold && (mp.Status == nil || *mp.Status) {
		falseVal := false
		if err := repos().ModelWithProvider.Update(ctx, mp.ID,
			models.ModelWithProvider{Status: &falseVal}); err != nil {
			slog.Error("failed to disable model provider after health check failures", "id", mp.ID, "error", err)
		} else {
			slog.Warn("model provider auto-disabled after health check failures", "id", mp.ID, "fail_count", failCount)
		}
	}
}

// disableDecision 汇总「连续失败判定停用」所需的两项配置。
//
// 任一读不到 → ok=false：本轮跳过判停步并留痕，禁止拿默认值冒充真配置执行停用写
// （Stage 36，与 Stage 33 结算侧同口径）。这两项的默认值组合（阈值 3 + 允许停用）
// 恰好是「最容易被误停用」的那一组——数据库不稳期正是故障期，读失败照默认值判
// 会把还能服务的关联停掉，且停用原因不入库、后台只看到「已停用」。
func (h *HealthChecker) disableDecision(ctx context.Context) (enabled bool, threshold int, ok bool) {
	enabled, err := getSettingBoolE(ctx, models.SettingKeyHealthCheckFailureDisableEnabled, true)
	if err != nil {
		settings.LogReadSkip("healthcheck auto-disable switch", models.SettingKeyHealthCheckFailureDisableEnabled, err)
		return false, 0, false
	}

	threshold, err = getSettingIntE(ctx, models.SettingKeyHealthCheckFailureThreshold, 3, 1)
	if err != nil {
		settings.LogReadSkip("healthcheck auto-disable threshold", models.SettingKeyHealthCheckFailureThreshold, err)
		return false, 0, false
	}

	return enabled, threshold, true
}

// getConsecutiveFailures 统计「最近连续几次拿到上游坏结论」。
//
// 窗口取**最近 10 条有结论的记录**（未完成的行不进窗口，Stage 36 Ruling 3）：本机没测成的行
// 既不是成功也不是失败，若按行数截窗，它们会把已有结论挤出窗口，使「故障期旧账、恢复后
// 照阈值结账」静默失效（3 次真实失败 + 10 轮未完成 → 恢复后从 1 重新数）。
func (h *HealthChecker) getConsecutiveFailures(ctx context.Context, mpID uint) (int, error) {
	logs, err := repos().HealthCheckLog.ListRecentDecidedByModelProviderID(ctx, mpID, 10,
		[]string{models.HealthCheckStatusSuccess, models.HealthCheckStatusError})
	if err != nil {
		return 0, err
	}

	count := 0
	for _, log := range logs {
		// 窗口已保证只含有结论的行：成功截断，其余（error）累加。
		if log.Status == models.HealthCheckStatusSuccess {
			return count, nil
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
