package settings

import (
	"context"
	"log/slog"

	"github.com/qkf688/llmux/models"
)

// Reader 是热路径只读配置接口，语义与 models.GetSetting* 完全对齐：
//   - key 不存在 / 解析失败 → 返回调用方传入的 defaultValue
//   - Int 的 minValue=0 表示不强制最小值约束
//
// 业务包（如 chat）通过此接口获取配置，不再直接依赖 Setting 表与 models.GetSetting* 全局函数。
//
// BoolE / IntE 是同取值语义的「读失败可辨」变体（Stage 33）：key 不存在 → (default, nil)
// （未设置是有效状态）；底层查询失败或存值非法 → (default, err)。**「读失败会引发写动作」
// 的决策点必须用 E 变体**并按 err 显式降级（跳过该写动作 + 留痕），禁止拿 default 冒充
// 真配置去执行写——结算序列中途预算失效退默认值曾会误停用可用关联。
type Reader interface {
	Bool(ctx context.Context, key string, defaultValue bool) bool
	Int(ctx context.Context, key string, defaultValue, minValue int) int
	String(ctx context.Context, key string, defaultValue string) string
	BoolE(ctx context.Context, key string, defaultValue bool) (bool, error)
	IntE(ctx context.Context, key string, defaultValue, minValue int) (int, error)
}

// modelsReader 委托 models.GetSetting* 实现 Reader。
// 保留与旧热路径完全一致的默认值 / minValue 策略，零行为漂移。
type modelsReader struct{}

// ModelsReader 是基于 models.GetSetting* 的默认单例 Reader。
//   - models.DB == nil 时 models.GetSetting* 会返回 default，无需额外处理
//   - 测试如需 mock，通过业务包的 SetSettingsReader 注入自定义实现
var ModelsReader Reader = modelsReader{}

func (modelsReader) Bool(ctx context.Context, key string, defaultValue bool) bool {
	return models.GetSettingBool(ctx, key, defaultValue)
}

func (modelsReader) Int(ctx context.Context, key string, defaultValue, minValue int) int {
	return models.GetSettingInt(ctx, key, defaultValue, minValue)
}

func (modelsReader) String(ctx context.Context, key string, defaultValue string) string {
	return models.GetSettingString(ctx, key, defaultValue)
}

func (modelsReader) BoolE(ctx context.Context, key string, defaultValue bool) (bool, error) {
	return models.GetSettingBoolE(ctx, key, defaultValue)
}

func (modelsReader) IntE(ctx context.Context, key string, defaultValue, minValue int) (int, error) {
	return models.GetSettingIntE(ctx, key, defaultValue, minValue)
}

// LogReadSkip 记录「设置读取失败 → 依赖该配置的写动作被跳过」（Stage 33 显式降级口径）。
// 跳过与写库失败都要留痕：否则事后无法回答「漏了多少次账、漏的哪一步」。
// 消息形状单源在此，各结算域（adjustment / chat 凭据结算）共用，禁散落自造文案。
func LogReadSkip(step, key string, err error) {
	slog.Warn("settings read failed: dependent write step skipped", "step", step, "key", key, "error", err)
}
