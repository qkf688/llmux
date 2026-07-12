package settings

import (
	"context"

	"github.com/atopos31/llmio/models"
)

// Reader 是热路径只读配置接口，语义与 models.GetSetting* 完全对齐：
//   - key 不存在 / 解析失败 → 返回调用方传入的 defaultValue
//   - Int 的 minValue=0 表示不强制最小值约束
//
// 业务包（如 chat）通过此接口获取配置，不再直接依赖 Setting 表与 models.GetSetting* 全局函数。
type Reader interface {
	Bool(ctx context.Context, key string, defaultValue bool) bool
	Int(ctx context.Context, key string, defaultValue, minValue int) int
	String(ctx context.Context, key string, defaultValue string) string
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