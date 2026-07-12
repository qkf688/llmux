package chat

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// applySuccessAdjustments 在成功调用后尝试提升权重与优先级
func applySuccessAdjustments(ctx context.Context, modelProviderID uint) {
	if !getAutoSuccessIncrease(ctx) {
		return
	}

	applyWeightIncreaseByID(ctx, modelProviderID)
	applyPriorityIncreaseByID(ctx, modelProviderID)
}

// applyWeightIncreaseByID 根据配置提升权重
func applyWeightIncreaseByID(ctx context.Context, modelProviderID uint) {
	adjustIntField(ctx, modelProviderID,
		getAutoWeightIncreaseStep, getAutoWeightIncreaseMax,
		"weight", "old_weight",
		func(max int) bool { return max <= 0 }, // 权重 max=0 也表示禁用
	)
}

// applyPriorityIncreaseByID 根据配置提升优先级
func applyPriorityIncreaseByID(ctx context.Context, modelProviderID uint) {
	adjustIntField(ctx, modelProviderID,
		getAutoPriorityIncreaseStep, getAutoPriorityIncreaseMax,
		"priority", "old_priority",
		func(max int) bool { return max < 0 }, // 优先级 max=0 是有效值（表示不允许增加）
	)
}

// adjustIntField 泛化权重/优先级提升逻辑。
// maxDisabled 判断 max 是否表示禁用：权重 max<=0 禁用，优先级 max<0 禁用。
func adjustIntField(
	ctx context.Context,
	modelProviderID uint,
	getStep func(context.Context) int,
	getMax func(context.Context) int,
	fieldName string,   // "weight" 或 "priority"
	logFieldName string, // "old_weight" 或 "old_priority"
	maxDisabled func(int) bool,
) {
	step := getStep(ctx)
	max := getMax(ctx)
	if step <= 0 || maxDisabled(max) {
		return
	}

	mp, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", modelProviderID).First(ctx)
	if err != nil {
		return
	}

	currentValue := mp.Weight
	if fieldName == "priority" {
		currentValue = mp.Priority
	}

	if max < currentValue {
		max = currentValue
	}

	newValue := currentValue + step
	if newValue > max {
		newValue = max
	}

	if newValue == currentValue {
		return
	}

	if _, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id = ?", modelProviderID).
		Update(ctx, fieldName, newValue); err != nil {
		slog.Error("update "+fieldName+" increase error", "error", err, "id", modelProviderID)
		return
	}

	slog.Info(fieldName+" increased after success", "id", modelProviderID, logFieldName, currentValue, "new_"+fieldName, newValue)
}

// getAutoWeightIncreaseStep 获取自动权重增加步长
func getAutoWeightIncreaseStep(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoWeightIncreaseStep, 1, 1)
}

// getAutoWeightIncreaseMax 获取自动权重增加上限
func getAutoWeightIncreaseMax(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoWeightIncreaseMax, 100, 1)
}

// getAutoPriorityIncreaseStep 获取自动优先级增加步长
func getAutoPriorityIncreaseStep(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoPriorityIncreaseStep, 1, 1)
}

// getAutoPriorityIncreaseMax 获取自动优先级增加上限
func getAutoPriorityIncreaseMax(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoPriorityIncreaseMax, 100, 0)
}

// getAutoSuccessIncrease 获取成功自增开关
func getAutoSuccessIncrease(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyAutoSuccessIncrease, true)
}

// applyWeightDecayByModelProviderID 根据配置对指定关联应用权重衰减
func applyWeightDecayByModelProviderID(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	if !getAutoWeightDecay(ctx) {
		return
	}

	decayStep := getAutoWeightDecayStep(ctx)
	mp, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", modelProviderID).First(ctx)
	if err != nil {
		return
	}

	newWeight := mp.Weight - decayStep
	if newWeight < 1 {
		newWeight = 1
	}
	if newWeight == mp.Weight {
		return
	}

	if _, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id = ?", modelProviderID).
		Update(ctx, "weight", newWeight); err != nil {
		slog.Error("update weight error", "error", err, "id", modelProviderID)
		return
	}

	slog.Info("weight decay applied", "provider", providerName, "model", providerModel, "id", modelProviderID, "old_weight", mp.Weight, "new_weight", newWeight)
}

// applyPriorityDecayByModelProviderID 根据配置对指定关联应用优先级衰减
func applyPriorityDecayByModelProviderID(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	if !getAutoPriorityDecay(ctx) {
		return
	}

	decayStep := getAutoPriorityDecayStep(ctx)
	threshold := getAutoPriorityDecayThreshold(ctx)
	disableEnabled := getAutoPriorityDecayDisableEnabled(ctx)

	mp, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", modelProviderID).First(ctx)
	if err != nil {
		return
	}

	newPriority := mp.Priority - decayStep
	if newPriority < 0 {
		newPriority = 0
	}
	if newPriority == mp.Priority {
		return
	}

	if _, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id = ?", modelProviderID).
		Update(ctx, "priority", newPriority); err != nil {
		slog.Error("update priority error", "error", err, "id", modelProviderID)
		return
	}

	slog.Info("priority decay applied", "provider", providerName, "model", providerModel, "id", modelProviderID, "old_priority", mp.Priority, "new_priority", newPriority)

	// 只有在启用自动禁用功能时才执行禁用操作
	if disableEnabled && newPriority <= threshold {
		falseVal := false
		if _, err := gorm.G[models.ModelWithProvider](models.DB).
			Where("id = ?", modelProviderID).
			Updates(ctx, models.ModelWithProvider{Status: &falseVal}); err != nil {
			slog.Error("auto disable model provider error", "error", err, "id", modelProviderID)
		} else {
			slog.Warn("model provider auto disabled due to low priority", "provider", providerName, "model", providerModel, "priority", newPriority, "threshold", threshold)
		}
	}
}

// shouldCountHealthCheckSuccess 健康检测成功是否计入成功调用
func shouldCountHealthCheckSuccess(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyHealthCheckCountAsSuccess, true)
}

// shouldCountHealthCheckFailure 健康检测失败是否计入失败调用
func shouldCountHealthCheckFailure(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyHealthCheckCountAsFailure, false)
}

// getAutoPriorityDecayDisableEnabled 获取自动优先级衰减禁用开关
func getAutoPriorityDecayDisableEnabled(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyAutoPriorityDecayDisableEnabled, true)
}

func getConsecutiveFailureThreshold(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyConsecutiveFailureThreshold, 3, 1)
}

func getConsecutiveFailureDisableEnabled(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyConsecutiveFailureDisableEnabled, true)
}

func incrementConsecutiveFailures(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	if !getConsecutiveFailureDisableEnabled(ctx) {
		return
	}

	threshold := getConsecutiveFailureThreshold(ctx)
	mp, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", modelProviderID).First(ctx)
	if err != nil {
		return
	}

	newCount := mp.ConsecutiveFailures + 1
	updates := models.ModelWithProvider{ConsecutiveFailures: newCount}
	shouldDisable := newCount >= threshold && (mp.Status == nil || *mp.Status)
	if shouldDisable {
		falseVal := false
		updates.Status = &falseVal
	}

	if _, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id = ?", modelProviderID).
		Updates(ctx, updates); err != nil {
		slog.Error("update consecutive failure count error", "error", err, "id", modelProviderID)
		return
	}

	if shouldDisable {
		slog.Warn("model provider auto disabled due to consecutive failures", "provider", providerName, "model", providerModel, "id", modelProviderID, "fail_count", newCount, "threshold", threshold)
	}
}

func resetConsecutiveFailures(ctx context.Context, modelProviderID uint) {
	if _, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id = ? AND consecutive_failures != 0", modelProviderID).
		Update(ctx, "consecutive_failures", 0); err != nil {
		slog.Error("reset consecutive failure count error", "error", err, "id", modelProviderID)
	}
}
