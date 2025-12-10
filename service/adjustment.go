package service

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// applySuccessAdjustments 在成功调用后尝试提升权重与优先级
func applySuccessAdjustments(ctx context.Context, modelProviderID uint) {
	applyWeightIncreaseByID(ctx, modelProviderID)
	applyPriorityIncreaseByID(ctx, modelProviderID)
}

// applyWeightIncreaseByID 根据配置提升权重
func applyWeightIncreaseByID(ctx context.Context, modelProviderID uint) {
	step := getAutoWeightIncreaseStep(ctx)
	max := getAutoWeightIncreaseMax(ctx)
	if step <= 0 || max <= 0 {
		return
	}

	mp, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", modelProviderID).First(ctx)
	if err != nil {
		return
	}

	if max < mp.Weight {
		max = mp.Weight
	}

	newWeight := mp.Weight + step
	if newWeight > max {
		newWeight = max
	}

	if newWeight == mp.Weight {
		return
	}

	if _, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id = ?", modelProviderID).
		Update(ctx, "weight", newWeight); err != nil {
		slog.Error("update weight increase error", "error", err, "id", modelProviderID)
		return
	}

	slog.Info("weight increased after success", "id", modelProviderID, "old_weight", mp.Weight, "new_weight", newWeight)
}

// applyPriorityIncreaseByID 根据配置提升优先级
func applyPriorityIncreaseByID(ctx context.Context, modelProviderID uint) {
	step := getAutoPriorityIncreaseStep(ctx)
	max := getAutoPriorityIncreaseMax(ctx)
	if step <= 0 || max < 0 {
		return
	}

	mp, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", modelProviderID).First(ctx)
	if err != nil {
		return
	}

	if max < mp.Priority {
		max = mp.Priority
	}

	newPriority := mp.Priority + step
	if newPriority > max {
		newPriority = max
	}

	if newPriority == mp.Priority {
		return
	}

	if _, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id = ?", modelProviderID).
		Update(ctx, "priority", newPriority); err != nil {
		slog.Error("update priority increase error", "error", err, "id", modelProviderID)
		return
	}

	slog.Info("priority increased after success", "id", modelProviderID, "old_priority", mp.Priority, "new_priority", newPriority)
}

// getAutoWeightIncreaseStep 获取自动权重增加步长
func getAutoWeightIncreaseStep(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightIncreaseStep).
		First(ctx)
	if err != nil {
		return 1
	}
	step, err := strconv.Atoi(setting.Value)
	if err != nil || step < 1 {
		return 1
	}
	return step
}

// getAutoWeightIncreaseMax 获取自动权重增加上限
func getAutoWeightIncreaseMax(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightIncreaseMax).
		First(ctx)
	if err != nil {
		return 100
	}
	max, err := strconv.Atoi(setting.Value)
	if err != nil || max < 1 {
		return 100
	}
	return max
}

// getAutoPriorityIncreaseStep 获取自动优先级增加步长
func getAutoPriorityIncreaseStep(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityIncreaseStep).
		First(ctx)
	if err != nil {
		return 1
	}
	step, err := strconv.Atoi(setting.Value)
	if err != nil || step < 1 {
		return 1
	}
	return step
}

// getAutoPriorityIncreaseMax 获取自动优先级增加上限
func getAutoPriorityIncreaseMax(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityIncreaseMax).
		First(ctx)
	if err != nil {
		return 100
	}
	max, err := strconv.Atoi(setting.Value)
	if err != nil || max < 0 {
		return 100
	}
	return max
}

// shouldCountHealthCheckSuccess 健康检测成功是否计入成功调用
func shouldCountHealthCheckSuccess(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsSuccess).
		First(ctx)
	if err != nil {
		return true
	}
	return setting.Value == "true"
}
