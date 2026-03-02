package autoassoc

// AssociationPreview 关联预览信息。
type AssociationPreview struct {
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
