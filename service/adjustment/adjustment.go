package adjustment

import (
	"context"
	"log/slog"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
)

// 本包的结算写决策（成功提升 / 失败衰减 / 连败判停）都是「读设置 → 写库」步骤。
// 设置读取一律走 E 变体（Stage 33 显式降级）：读失败（DB 争用 / 预算到期 / 存值非法）
// 时跳过该写步骤并留痕，禁止拿默认值冒充真配置执行写动作——结算序列中途预算失效
// 退默认值曾会用默认阈值（3）误停用可用关联，且停用原因不入库。
// 粒度口径「只跳过读不到配置的那一步」：同一结算里配置可读的步骤照常执行
// （计数照记、可读配置驱动的写照做）；计数类纯数据维护不寄生于配置读取。

// intSetting / boolSetting 把「键 + 默认值 + 约束」与读取绑成一处：跳步留痕要带键名，
// 散在 getter 里会在调用点复制魔法字符串，键改动时漏改即静默失效（DRY）。
type intSetting struct {
	key string
	def int
	min int
}

func (s intSetting) read(ctx context.Context) (int, error) {
	return settingsReader.IntE(ctx, s.key, s.def, s.min)
}

type boolSetting struct {
	key string
	def bool
}

func (s boolSetting) read(ctx context.Context) (bool, error) {
	return settingsReader.BoolE(ctx, s.key, s.def)
}

// 结算写决策的配置单元（键名 / 默认值 / 约束与 models/setting_schema.go 对齐）。
var (
	autoSuccessIncrease         = boolSetting{models.SettingKeyAutoSuccessIncrease, true}
	autoWeightIncreaseStep      = intSetting{models.SettingKeyAutoWeightIncreaseStep, 1, 1}
	autoWeightIncreaseMax       = intSetting{models.SettingKeyAutoWeightIncreaseMax, 100, 1}
	autoPriorityIncreaseStep    = intSetting{models.SettingKeyAutoPriorityIncreaseStep, 1, 1}
	autoPriorityIncreaseMax     = intSetting{models.SettingKeyAutoPriorityIncreaseMax, 100, 0}
	autoWeightDecay             = boolSetting{models.SettingKeyAutoWeightDecay, false}
	autoWeightDecayStep         = intSetting{models.SettingKeyAutoWeightDecayStep, 1, 0}
	autoPriorityDecay           = boolSetting{models.SettingKeyAutoPriorityDecay, false}
	autoPriorityDecayStep       = intSetting{models.SettingKeyAutoPriorityDecayStep, 1, 0}
	autoPriorityDecayThreshold  = intSetting{models.SettingKeyAutoPriorityDecayThreshold, 90, 0}
	autoPriorityDecayDisable    = boolSetting{models.SettingKeyAutoPriorityDecayDisableEnabled, true}
	consecutiveFailureThreshold = intSetting{models.SettingKeyConsecutiveFailureThreshold, 3, 1}
	consecutiveFailureDisable   = boolSetting{models.SettingKeyConsecutiveFailureDisableEnabled, true}
)

// ApplySuccessAdjustments 在成功调用后尝试提升权重与优先级。
func ApplySuccessAdjustments(ctx context.Context, modelProviderID uint) {
	enabled, err := autoSuccessIncrease.read(ctx)
	if err != nil {
		settings.LogReadSkip("success increase gate", autoSuccessIncrease.key, err)
		return
	}
	if !enabled {
		return
	}

	applyWeightIncreaseByID(ctx, modelProviderID)
	applyPriorityIncreaseByID(ctx, modelProviderID)
}

// applyWeightIncreaseByID 根据配置提升权重
func applyWeightIncreaseByID(ctx context.Context, modelProviderID uint) {
	adjustIntField(ctx, modelProviderID,
		autoWeightIncreaseStep, autoWeightIncreaseMax,
		repos().ModelWithProvider.IncreaseWeight,
		"success weight increase",
		func(max int) bool { return max <= 0 }, // 权重 max=0 也表示禁用
	)
}

// applyPriorityIncreaseByID 根据配置提升优先级
func applyPriorityIncreaseByID(ctx context.Context, modelProviderID uint) {
	adjustIntField(ctx, modelProviderID,
		autoPriorityIncreaseStep, autoPriorityIncreaseMax,
		repos().ModelWithProvider.IncreasePriority,
		"success priority increase",
		func(max int) bool { return max < 0 }, // 优先级 max=0 是有效值（表示不允许增加）
	)
}

// adjustIntField 泛化权重/优先级提升逻辑（原子操作，无读改写竞态）。
// increase 是 repo 的类型化原子方法，service 不依赖列名细节（DIP）。
// maxDisabled 判断 max 是否表示禁用：权重 max<=0 禁用，优先级 max<0 禁用。
// step / max 任一读失败 → 整个提升写跳过并留痕（幅度不可信就不写）。
func adjustIntField(
	ctx context.Context,
	modelProviderID uint,
	step, max intSetting,
	increase func(context.Context, uint, int, int) (int64, error),
	label string,
	maxDisabled func(int) bool,
) {
	stepVal, err := step.read(ctx)
	if err != nil {
		settings.LogReadSkip(label, step.key, err)
		return
	}
	maxVal, err := max.read(ctx)
	if err != nil {
		settings.LogReadSkip(label, max.key, err)
		return
	}
	if stepVal <= 0 || maxDisabled(maxVal) {
		return
	}

	affected, err := increase(ctx, modelProviderID, stepVal, maxVal)
	if err != nil {
		slog.Error(label+" error", "error", err, "id", modelProviderID)
		return
	}
	if affected > 0 {
		slog.Info(label+" applied", "id", modelProviderID, "step", stepVal, "max", maxVal)
	}
}

// ApplyWeightDecayByModelProviderID 根据配置对指定关联应用权重衰减（原子操作，无读改写竞态）。
func ApplyWeightDecayByModelProviderID(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	enabled, err := autoWeightDecay.read(ctx)
	if err != nil {
		settings.LogReadSkip("weight decay", autoWeightDecay.key, err)
		return
	}
	if !enabled {
		return
	}

	decayStep, err := autoWeightDecayStep.read(ctx)
	if err != nil {
		settings.LogReadSkip("weight decay", autoWeightDecayStep.key, err)
		return
	}
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
// 衰减写与「低优先级判停」是两个独立步骤，各自有自己的配置读取：任一步的配置读不到
// 只跳过该步（衰减幅度读不到仍可判停；判停阈值读不到仍可衰减）。
func ApplyPriorityDecayByModelProviderID(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	enabled, err := autoPriorityDecay.read(ctx)
	if err != nil {
		settings.LogReadSkip("priority decay", autoPriorityDecay.key, err)
		return
	}
	if !enabled {
		return
	}

	decayStep, err := autoPriorityDecayStep.read(ctx)
	if err != nil {
		settings.LogReadSkip("priority decay", autoPriorityDecayStep.key, err)
	} else {
		affected, err := repos().ModelWithProvider.DecayPriority(ctx, modelProviderID, decayStep, 0)
		if err != nil {
			slog.Error("update priority error", "error", err, "id", modelProviderID)
		} else if affected > 0 {
			slog.Info("priority decay applied", "provider", providerName, "model", providerModel, "id", modelProviderID, "step", decayStep)
		}
	}

	// 判停步骤：配置读不到即跳过（禁止拿编造阈值停用）。衰减写成功与否不影响它
	// ——判停重读当前值，以库里的真实优先级为准。
	disableEnabled, err := autoPriorityDecayDisable.read(ctx)
	if err != nil {
		settings.LogReadSkip("priority auto-disable", autoPriorityDecayDisable.key, err)
		return
	}
	if !disableEnabled {
		return
	}
	threshold, err := autoPriorityDecayThreshold.read(ctx)
	if err != nil {
		settings.LogReadSkip("priority auto-disable", autoPriorityDecayThreshold.key, err)
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

// AutoWeightDecayEnabled 返回是否开启自动权重衰减（编排层可先短路再扫候选）。
// 编排层短路是「省扫描」不是写决策，读失败按未开启处理即可，不走 E 变体。
func AutoWeightDecayEnabled(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyAutoWeightDecay, false)
}

// AutoPriorityDecayEnabled 返回是否开启自动优先级衰减（编排层可先短路再扫候选）。
// 读失败处置同 AutoWeightDecayEnabled。
func AutoPriorityDecayEnabled(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyAutoPriorityDecay, false)
}

// IncrementConsecutiveFailures 原子自增连续失败次数。达阈值时若启用自动禁用则禁用关联。
// 计数始终自增（数据列维护不寄生于策略开关）；判停是独立步骤，配置读不到即跳过
// （Stage 33：不拿编造阈值停用可用关联），跳过与写失败均留痕。
func IncrementConsecutiveFailures(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	// 原子自增，无读改写竞态
	if _, err := repos().ModelWithProvider.IncrementConsecutiveFailures(ctx, modelProviderID); err != nil {
		slog.Error("increment consecutive failure count error", "error", err, "id", modelProviderID)
		return
	}

	enabled, err := consecutiveFailureDisable.read(ctx)
	if err != nil {
		settings.LogReadSkip("consecutive-failure auto-disable", consecutiveFailureDisable.key, err)
		return
	}
	if !enabled {
		return
	}

	threshold, err := consecutiveFailureThreshold.read(ctx)
	if err != nil {
		settings.LogReadSkip("consecutive-failure auto-disable", consecutiveFailureThreshold.key, err)
		return
	}

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
