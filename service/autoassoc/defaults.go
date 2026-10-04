package autoassoc

import "github.com/qkf688/llmux/models"

// DefaultWeightFallback 设置缺失/非法时的默认权重（与 setting schema 一致）。
const DefaultWeightFallback = 100

// DefaultPriorityFallback 设置缺失/非法时的默认优先级（与 setting schema 一致）。
const DefaultPriorityFallback = 100

// NewDefaultAssociation 构造自动关联的默认 ModelWithProvider 行。
// defaultWeight / defaultPriority 由调用方从 setting schema 读取后传入，
// 保持本函数为纯构造（不嵌入配置读取，SRP）。
//
// ToolCall / StructuredOutput / Image 三态置 nil = 继承 Model 的能力真值：
// 绝大多数第三方与官方能力一致，无需逐条标注；确有链路弱于模型时再在关联上手工 override。
// （此前在此硬编码 true/false/false，等于把「模型固有能力」复制到每条关联上，
//
//	与前端创建弹窗的默认全 true 口径也各说各话。）
func NewDefaultAssociation(modelID, providerID uint, providerModel string, defaultWeight, defaultPriority int) models.ModelWithProvider {
	trueVal := true
	falseVal := false
	return models.ModelWithProvider{
		ModelID:          modelID,
		ProviderModel:    providerModel,
		ProviderID:       providerID,
		ToolCall:         nil, // 继承 Model.ToolCall
		StructuredOutput: nil, // 继承 Model.StructuredOutput
		Image:            nil, // 继承 Model.Image
		WithHeader:       &falseVal,
		Status:           &trueVal,
		CustomerHeaders:  map[string]string{},
		Weight:           defaultWeight,
		Priority:         defaultPriority,
	}
}
