package autoassoc

import "github.com/atopos31/llmio/models"

// DefaultWeightFallback 设置缺失/非法时的默认权重（与 setting schema 一致）。
const DefaultWeightFallback = 100

// DefaultPriorityFallback 设置缺失/非法时的默认优先级（与 setting schema 一致）。
const DefaultPriorityFallback = 100

// NewDefaultAssociation 构造自动关联的默认 ModelWithProvider 行。
// defaultWeight / defaultPriority 由调用方从 setting schema 读取后传入，
// 保持本函数为纯构造（不嵌入配置读取，SRP）。
func NewDefaultAssociation(modelID, providerID uint, providerModel string, defaultWeight, defaultPriority int) models.ModelWithProvider {
	trueVal := true
	falseVal := false
	return models.ModelWithProvider{
		ModelID:          modelID,
		ProviderModel:    providerModel,
		ProviderID:       providerID,
		ToolCall:         &trueVal,
		StructuredOutput: &falseVal,
		Image:            &falseVal,
		WithHeader:       &falseVal,
		Status:           &trueVal,
		CustomerHeaders:  map[string]string{},
		Weight:           defaultWeight,
		Priority:         defaultPriority,
	}
}
