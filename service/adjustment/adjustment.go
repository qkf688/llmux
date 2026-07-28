package adjustment

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
)

// ApplySuccessAdjustments 在成功调用后尝试提升权重与优先级。
func ApplySuccessAdjustments(ctx context.Context, modelProviderID uint) {
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
		repos().ModelWithProvider.IncreaseWeight,
		"weight",
		func(max int) bool { return max <= 0 }, // 权重 max=0 也表示禁用
	)
}

// applyPriorityIncreaseByID 根据配置提升优先级
func applyPriorityIncreaseByID(ctx context.Context, modelProviderID uint) {
	adjustIntField(ctx, modelProviderID,
		getAutoPriorityIncreaseStep, getAutoPriorityIncreaseMax,
		repos().ModelWithProvider.IncreasePriority,
		"priority",
		func(max int) bool { return max < 0 }, // 优先级 max=0 是有效值（表示不允许增加）
	)
}

// adjustIntField 泛化权重/优先级提升逻辑（原子操作，无读改写竞态）。
// increase 是 repo 的类型化原子方法，service 不依赖列名细节（DIP）。
// maxDisabled 判断 max 是否表示禁用：权重 max<=0 禁用，优先级 max<0 禁用。
func adjustIntField(
	ctx context.Context,
	modelProviderID uint,
	getStep func(context.Context) int,
	getMax func(context.Context) int,
	increase func(context.Context, uint, int, int) (int64, error),
	label string,
	maxDisabled func(int) bool,
) {
	step := getStep(ctx)
	max := getMax(ctx)
	if step <= 0 || maxDisabled(max) {
		return
	}

	affected, err := increase(ctx, modelProviderID, step, max)
	if err != nil {
		slog.Error(label+" increase error", "error", err, "id", modelProviderID)
		return
	}
	if affected > 0 {
		slog.Info(label+" increased after success", "id", modelProviderID, "step", step, "max", max)
	}
}

func getAutoWeightIncreaseStep(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoWeightIncreaseStep, 1, 1)
}

func getAutoWeightIncreaseMax(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoWeightIncreaseMax, 100, 1)
}

func getAutoPriorityIncreaseStep(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoPriorityIncreaseStep, 1, 1)
}

func getAutoPriorityIncreaseMax(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoPriorityIncreaseMax, 100, 0)
}

func getAutoSuccessIncrease(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyAutoSuccessIncrease, true)
}

// ApplyWeightDecayByModelProviderID 根据配置对指定关联应用权重衰减（原子操作，无读改写竞态）。
func ApplyWeightDecayByModelProviderID(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	if !getAutoWeightDecay(ctx) {
		return
	}

	decayStep := getAutoWeightDecayStep(ctx)
	affected, err := repos().ModelWithProvider.DecayWeight(ctx, modelProviderID, decayStep, 1)
	if err != nil {
		slog.Error("update weight error", "error", err, "id", modelProviderID)
		return
	}
	if affected > 0 {
		slog.Info("weight decay applied", "provider", providerName, "model", providerModel, "id", modelProviderID, "step", decayStep)
	}
}

// ApplyPriorityDecayByModelProviderID 根据配置对指定关联应用优先级衰减（原子操作，无读改写竞态）。
func ApplyPriorityDecayByModelProviderID(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	if !getAutoPriorityDecay(ctx) {
		return
	}

	decayStep := getAutoPriorityDecayStep(ctx)
	threshold := getAutoPriorityDecayThreshold(ctx)
	disableEnabled := getAutoPriorityDecayDisableEnabled(ctx)

	affected, err := repos().ModelWithProvider.DecayPriority(ctx, modelProviderID, decayStep, 0)
	if err != nil {
		slog.Error("update priority error", "error", err, "id", modelProviderID)
		return
	}
	if affected > 0 {
		slog.Info("priority decay applied", "provider", providerName, "model", providerModel, "id", modelProviderID, "step", decayStep)
	}

	// 原子衰减后重读当前值，判断是否需要禁用
	if !disableEnabled {
		return
	}
	mp, err := repos().ModelWithProvider.Get(ctx, modelProviderID)
	if err != nil {
		slog.Error("re-read model provider after priority decay", "error", err, "id", modelProviderID)
		return
	}
	if mp.Priority <= threshold && (mp.Status == nil || *mp.Status) {
		if _, err := repos().ModelWithProvider.UpdateFields(ctx, modelProviderID, map[string]any{"status": false}); err != nil {
			slog.Error("auto disable model provider error", "error", err, "id", modelProviderID)
		} else {
			slog.Warn("model provider auto disabled due to low priority", "provider", providerName, "model", providerModel, "id", modelProviderID, "priority", mp.Priority, "threshold", threshold)
		}
	}
}

// ShouldCountHealthCheckSuccess 健康检测成功是否计入成功调用。
func ShouldCountHealthCheckSuccess(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyHealthCheckCountAsSuccess, true)
}

// ShouldCountHealthCheckFailure 健康检测失败是否计入失败调用。
func ShouldCountHealthCheckFailure(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyHealthCheckCountAsFailure, false)
}

func getAutoPriorityDecayDisableEnabled(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyAutoPriorityDecayDisableEnabled, true)
}

// AutoWeightDecayEnabled 返回是否开启自动权重衰减（编排层可先短路再扫候选）。
func AutoWeightDecayEnabled(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyAutoWeightDecay, false)
}

func getAutoWeightDecay(ctx context.Context) bool {
	return AutoWeightDecayEnabled(ctx)
}

func getAutoWeightDecayStep(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoWeightDecayStep, 1, 0)
}

// AutoPriorityDecayEnabled 返回是否开启自动优先级衰减（编排层可先短路再扫候选）。
func AutoPriorityDecayEnabled(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyAutoPriorityDecay, false)
}

func getAutoPriorityDecay(ctx context.Context) bool {
	return AutoPriorityDecayEnabled(ctx)
}

func getAutoPriorityDecayStep(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoPriorityDecayStep, 1, 0)
}

func getAutoPriorityDecayThreshold(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyAutoPriorityDecayThreshold, 90, 0)
}

func getConsecutiveFailureThreshold(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyConsecutiveFailureThreshold, 3, 1)
}

func getConsecutiveFailureDisableEnabled(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyConsecutiveFailureDisableEnabled, true)
}

// IncrementConsecutiveFailures 原子自增连续失败次数。达阈值时若启用自动禁用则禁用关联。
// 计数始终自增（数据列维护不寄生于策略开关）；仅 disable-check 受 ConsecutiveFailureDisableEnabled 门控。
func IncrementConsecutiveFailures(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	// 原子自增，无读改写竞态
	if _, err := repos().ModelWithProvider.IncrementConsecutiveFailures(ctx, modelProviderID); err != nil {
		slog.Error("increment consecutive failure count error", "error", err, "id", modelProviderID)
		return
	}

	if !getConsecutiveFailureDisableEnabled(ctx) {
		return
	}

	threshold := getConsecutiveFailureThreshold(ctx)

	// 重读当前值判断是否需要禁用（禁用操作幂等，竞态无害）
	mp, err := repos().ModelWithProvider.Get(ctx, modelProviderID)
	if err != nil {
		slog.Error("re-read model provider after failure increment", "error", err, "id", modelProviderID)
		return
	}
	if mp.ConsecutiveFailures >= threshold && (mp.Status == nil || *mp.Status) {
		if _, err := repos().ModelWithProvider.UpdateFields(ctx, modelProviderID, map[string]any{"status": false}); err != nil {
			slog.Error("auto disable model provider error", "error", err, "id", modelProviderID)
		} else {
			slog.Warn("model provider auto disabled due to consecutive failures", "provider", providerName, "model", providerModel, "id", modelProviderID, "fail_count", mp.ConsecutiveFailures, "threshold", threshold)
		}
	}
}

// ResetConsecutiveFailures 成功调用后将连续失败计数清零。
func ResetConsecutiveFailures(ctx context.Context, modelProviderID uint) {
	if _, err := repos().ModelWithProvider.ResetConsecutiveFailures(ctx, modelProviderID); err != nil {
		slog.Error("reset consecutive failure count error", "error", err, "id", modelProviderID)
	}
}
