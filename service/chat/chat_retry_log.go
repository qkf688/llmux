package chat

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/adjustment"
	"github.com/samber/lo"
)

func RecordRetryLog(ctx context.Context, retryLog chan models.ChatLog, modelWithProviderMap map[uint]models.ModelWithProvider) {
	for log := range retryLog {
		if _, err := SaveChatLog(ctx, log); err != nil {
			slog.Error("save chat log error", "error", err)
		}
		// 当调用失败时，检查并应用权重衰减和优先级衰减
		if log.Status == "error" {
			applyWeightDecay(ctx, log, modelWithProviderMap)
			applyPriorityDecay(ctx, log, modelWithProviderMap)
		}
	}
}

// applyWeightDecay 应用权重衰减
func applyWeightDecay(ctx context.Context, log models.ChatLog, modelWithProviderMap map[uint]models.ModelWithProvider) {
	// 关闭时直接返回，避免扫 map / 查 Provider 的多余开销。
	if !adjustment.AutoWeightDecayEnabled(ctx) {
		return
	}

	if id, ok := findModelProviderIDByLog(ctx, log, modelWithProviderMap); ok {
		adjustment.ApplyWeightDecayByModelProviderID(ctx, id, log.ProviderName, log.ProviderModel)
	}
}

// applyPriorityDecay 应用优先级衰减
func applyPriorityDecay(ctx context.Context, log models.ChatLog, modelWithProviderMap map[uint]models.ModelWithProvider) {
	// 关闭时直接返回，避免扫 map / 查 Provider 的多余开销。
	if !adjustment.AutoPriorityDecayEnabled(ctx) {
		return
	}

	if id, ok := findModelProviderIDByLog(ctx, log, modelWithProviderMap); ok {
		adjustment.ApplyPriorityDecayByModelProviderID(ctx, id, log.ProviderName, log.ProviderModel)
	}
}

// findModelProviderIDByLog 用日志中的 (provider_name, provider_model) 反查候选集里对应的关联 ID。
// 关联只记录 provider_id，需一次性批量取回 Provider 名称后比对，避免逐条查库。
func findModelProviderIDByLog(ctx context.Context, log models.ChatLog, modelWithProviderMap map[uint]models.ModelWithProvider) (uint, bool) {
	providerIDs := lo.Uniq(lo.Map(lo.Values(modelWithProviderMap), func(mp models.ModelWithProvider, _ int) uint {
		return mp.ProviderID
	}))
	providers, err := repos().Provider.ListByIDs(ctx, providerIDs)
	if err != nil {
		slog.Error("failed to load providers for decay matching", "error", err)
		return 0, false
	}
	providerNameByID := lo.SliceToMap(providers, func(p models.Provider) (uint, string) { return p.ID, p.Name })

	for id, mwp := range modelWithProviderMap {
		if providerNameByID[mwp.ProviderID] == log.ProviderName && mwp.ProviderModel == log.ProviderModel {
			return id, true
		}
	}
	return 0, false
}
