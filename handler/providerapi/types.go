package providerapi

// ProviderTemplate 供应商配置模板。
type ProviderTemplate struct {
	Type     string `json:"type"`
	Template string `json:"template"`
}

// 阶段 5 OCP-6 前仍用静态切片；勿在此做自注册。
var template = []ProviderTemplate{
	{
		Type: "openai",
		Template: `{
			"base_url": "https://api.openai.com/v1",
			"api_key": "YOUR_API_KEY"
		}`,
	},
	{
		Type: "openai-res",
		Template: `{
			"base_url": "https://api.openai.com/v1",
			"api_key": "YOUR_API_KEY"
		}`,
	},
	{
		Type: "anthropic",
		Template: `{
			"base_url": "https://api.anthropic.com/v1",
			"api_key": "YOUR_API_KEY",
			"beta": "",
			"version": "2023-06-01",
			"auth_type": "x-api-key"
		}`,
	},
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