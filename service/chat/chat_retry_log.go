package chat

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/atopos31/llmio/models"
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
	// 检查是否开启自动权重衰减
	if !getAutoWeightDecay(ctx) {
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
			applyWeightDecayByModelProviderID(ctx, id, log.ProviderName, log.ProviderModel)
			break
		}
	}
}

// getAutoWeightDecay 获取自动权重衰减开关
func getAutoWeightDecay(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecay).
		First(ctx)
	if err != nil {
		return false // 默认关闭
	}
	return setting.Value == "true"
}

// getAutoWeightDecayStep 获取自动权重衰减步长
func getAutoWeightDecayStep(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecayStep).
		First(ctx)
	if err != nil {
		return 1 // 默认步长1
	}
	step, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 1
	}
	return step
}

// applyPriorityDecay 应用优先级衰减
func applyPriorityDecay(ctx context.Context, log models.ChatLog, modelWithProviderMap map[uint]models.ModelWithProvider) {
	// 检查是否开启自动优先级衰减
	if !getAutoPriorityDecay(ctx) {
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
			applyPriorityDecayByModelProviderID(ctx, id, log.ProviderName, log.ProviderModel)
			break
		}
	}
}

// getAutoPriorityDecay 获取自动优先级衰减开关
func getAutoPriorityDecay(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecay).
		First(ctx)
	if err != nil {
		return false // 默认关闭
	}
	return setting.Value == "true"
}

// getAutoPriorityDecayStep 获取自动优先级衰减步长
func getAutoPriorityDecayStep(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayStep).
		First(ctx)
	if err != nil {
		return 1 // 默认步长1
	}
	step, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 1
	}
	return step
}

// getAutoPriorityDecayThreshold 获取自动优先级衰减阈值
func getAutoPriorityDecayThreshold(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayThreshold).
		First(ctx)
	if err != nil {
		return 90 // 默认阈值90
	}
	threshold, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 90
	}
	return threshold
}
