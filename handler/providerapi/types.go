package providerapi

import "github.com/qkf688/llmux/models"

// ProviderTemplate 供应商配置模板。
type ProviderTemplate struct {
	Type     string `json:"type"`
	Template string `json:"template"`
}

// EndpointInput 协议端点写入体（设计定案：一协议一行；URL 空 = 继承 base_url）。
type EndpointInput struct {
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
	Enabled  bool   `json:"enabled"`
}

// GroupInput 凭据分组写入体（凭据来源二选一：inline / pool）。
type GroupInput struct {
	Name       string   `json:"name"`
	Weight     int      `json:"weight"`
	Models     string   `json:"models"`
	Source     string   `json:"source"` // "inline" | "pool"
	InlineKeys []string `json:"inline_keys"`
	PoolID     *uint    `json:"pool_id"`
}

// ProviderRequest represents the request body for creating/updating a provider.
// S6 起必须携带 protocols/endpoints/groups；config 仅存 adapter 字段（不含 api_key/_schedule）。
type ProviderRequest struct {
	Name               string          `json:"name"`
	Type               string          `json:"type"`
	Config             string          `json:"config"`
	Console            string          `json:"console"`
	Proxy              string          `json:"proxy"`
	ModelEndpoint      *bool           `json:"model_endpoint"`
	ModelFilterEnabled *bool           `json:"model_filter_enabled"`
	Blacklisted        *bool           `json:"blacklisted"` // 是否拉黑（拉黑后跳过自动关联/一键关联）
	AuthType           string          `json:"auth_type"`   // 认证方式：x-api-key 或 bearer，仅用于 Anthropic 类型
	Protocols          []string        `json:"protocols"`
	Endpoints          []EndpointInput `json:"endpoints"`
	Groups             []GroupInput    `json:"groups"`
}

// ProviderListItem 供应商列表项：内嵌 models.Provider（含 Protocols）+ 端点/分组计数（列表徽标）。
type ProviderListItem struct {
	models.Provider
	EndpointCount int64 `json:"EndpointCount"`
	GroupCount    int64 `json:"GroupCount"`
}

// GroupDetail 详情页分组：内嵌 KeyGroup + 派生 Source + inline 明文 keys（管理端 JWT 域）。
// PoolID 三态指针禁 omitempty；InlineKeys 空数组表示 inline 无 key 或 pool 来源。
type GroupDetail struct {
	models.KeyGroup
	Source     string   `json:"Source"`
	InlineKeys []string `json:"InlineKeys"`
}

// ProviderDetail 供应商详情：Provider + 展开的 endpoints/groups（编辑弹窗回填用）。
type ProviderDetail struct {
	models.Provider
	Endpoints []models.Endpoint `json:"Endpoints"`
	Groups    []GroupDetail     `json:"Groups"`
}
