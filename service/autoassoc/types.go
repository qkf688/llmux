package autoassoc

import "github.com/atopos31/llmio/models"

// Preview 关联预览信息（创建或清理候选）。
type Preview struct {
	ModelID       uint   `json:"model_id"`
	ModelName     string `json:"model_name"`
	ProviderID    uint   `json:"provider_id"`
	ProviderName  string `json:"provider_name"`
	ProviderModel string `json:"provider_model"`
}

type associationCandidate struct {
	ModelID       uint
	ModelName     string
	ProviderID    uint
	ProviderName  string
	ProviderModel string
}

// NameMatcher 模板名称匹配器（由调用方注入 TemplateIndex，避免 service 循环依赖）。
type NameMatcher interface {
	Match(name string) []uint
}

// BuildIndexFunc 根据全量数据构建模板匹配索引。
type BuildIndexFunc func(
	allModels []models.Model,
	allAssociations []models.ModelWithProvider,
	manualItems []models.ModelTemplateItem,
) NameMatcher
