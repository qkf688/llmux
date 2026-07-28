package providerapi

// ProviderTemplate 供应商配置模板。
type ProviderTemplate struct {
	Type     string `json:"type"`
	Template string `json:"template"`
}

// ProviderRequest represents the request body for creating/updating a provider.
type ProviderRequest struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	Config             string `json:"config"`
	Console            string `json:"console"`
	Proxy              string `json:"proxy"`
	ModelEndpoint      *bool  `json:"model_endpoint"`
	ModelFilterEnabled *bool  `json:"model_filter_enabled"`
	Blacklisted        *bool  `json:"blacklisted"` // 是否拉黑（拉黑后跳过自动关联/一键关联）
	AuthType           string `json:"auth_type"`   // 认证方式：x-api-key 或 bearer，仅用于 Anthropic 类型
}
