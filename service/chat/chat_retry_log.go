package chat

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/adjustment"
	"gorm.io/gorm"
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

	// 查找对应的 ModelWithProvider
	for id, mwp := range modelWithProviderMap {
		// 获取供应商信息以匹配日志
		provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", mwp.ProviderID).First(ctx)
		if err != nil {
			continue
		}
		if provider.Name == log.ProviderName && mwp.ProviderModel == log.ProviderModel {
			adjustment.ApplyWeightDecayByModelProviderID(ctx, id, log.ProviderName, log.ProviderModel)
			break
		}
	}
}

// applyPriorityDecay 应用优先级衰减
func applyPriorityDecay(ctx context.Context, log models.ChatLog, modelWithProviderMap map[uint]models.ModelWithProvider) {
	// 关闭时直接返回，避免扫 map / 查 Provider 的多余开销。
	if !adjustment.AutoPriorityDecayEnabled(ctx) {
		return
	}

	// 查找对应的 ModelWithProvider
	for id, mwp := range modelWithProviderMap {
		// 获取供应商信息以匹配日志
		provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", mwp.ProviderID).First(ctx)
		if err != nil {
			continue
		}
		if provider.Name == log.ProviderName && mwp.ProviderModel == log.ProviderModel {
			adjustment.ApplyPriorityDecayByModelProviderID(ctx, id, log.ProviderName, log.ProviderModel)
			break
		}
	}
}
