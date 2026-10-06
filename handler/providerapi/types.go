package providerapi

import (
	"github.com/qkf688/llmux/models"
)

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

// GroupInput 凭据分组写入体（凭据统一来自引用的号池）。
// ID = 既有分组就地更新（0/缺失 = 新建）；模型白名单**不在**写入体里——KeyGroup.Models
// 是模型同步的专属数据（唯一写入方 modelsync），供应商表单保存不得触碰（清掉它 = 同步
// 快照丢失，且组织级目录缩水会连累自动关联清理误删关联）。
type GroupInput struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Weight int    `json:"weight"`
	PoolID *uint  `json:"pool_id"`
}

// ProviderRequest represents the request body for creating/updating a provider.
// S6 起全量路径必须携带 protocols/endpoints/groups；config 仅存 adapter 字段（不含 api_key/_schedule）。
// Update 支持 partial-update：Endpoints/Groups 为 nil（键缺失）表示只改顶层标量、不动子表，
// 供「开关切换」类轻量动作复用同一 PUT（表单提交则全量携带三个字段）。
// 用指针切片区分「键缺失（nil）」与「显式空数组（非 nil 空）」：普通非指针切片无法区分二者。
// 只带 endpoints/groups 其中一键（半 partial）会被 400 拒绝，禁止静默丢弃该键。
type ProviderRequest struct {
	Name               string           `json:"name"`
	Type               string           `json:"type"`
	Config             string           `json:"config"`
	Console            string           `json:"console"`
	Proxy              string           `json:"proxy"`
	ModelEndpoint      *bool            `json:"model_endpoint"`
	ModelFilterEnabled *bool            `json:"model_filter_enabled"`
	Blacklisted        *bool            `json:"blacklisted"` // 是否拉黑（拉黑后跳过自动关联/一键关联）
	Protocols          []string         `json:"protocols"`
	Endpoints          *[]EndpointInput `json:"endpoints"`
	Groups             *[]GroupInput    `json:"groups"`
}

// ProviderListItem 供应商列表项：内嵌 models.Provider（含 Protocols）+ 端点/分组计数（列表徽标）。
type ProviderListItem struct {
	models.Provider
	EndpointCount int64 `json:"EndpointCount"`
	GroupCount    int64 `json:"GroupCount"`
}

// GroupDetail 详情页分组：内嵌 KeyGroup（凭据经其 PoolID 引用号池，明文永不出响应）。
// PoolID 三态指针禁 omitempty。
type GroupDetail struct {
	models.KeyGroup
}

// ProviderDetail 供应商详情：Provider + 展开的 endpoints/groups（编辑弹窗回填用）。
type ProviderDetail struct {
	models.Provider
	Endpoints []models.Endpoint `json:"Endpoints"`
	Groups    []GroupDetail     `json:"Groups"`
}
