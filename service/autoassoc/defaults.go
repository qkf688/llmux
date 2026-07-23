package autoassoc

import "github.com/atopos31/llmio/models"

// DefaultWeight 自动关联创建时的默认权重。
const DefaultWeight = 5

// DefaultPriorityFallback 设置缺失/非法时的默认优先级（与 setting schema 一致）。
const DefaultPriorityFallback = 100

// NewDefaultAssociation 构造自动关联的默认 ModelWithProvider 行。
func NewDefaultAssociation(modelID, providerID uint, providerModel string, defaultPriority int) models.ModelWithProvider {
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
		Weight:           DefaultWeight,
		Priority:         defaultPriority,
	}
}
