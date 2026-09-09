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

// InlineKeyInput 内联凭据写入项：ID 引用既有凭据（key 空 = 原样保留；key 非空 =
// 替换该条内容），ID 为 0 表示以 key 命名新增。掩码回填的条目提交时 key 传空串
// 由 ID 保留——避免掩码文本被当成明文落库（恶意/误提交的占位符顶替死 key）。
type InlineKeyInput struct {
	ID  uint   `json:"id"`
	Key string `json:"key"`
}

// GroupInput 凭据分组写入体（凭据来源二选一：inline / pool）。
type GroupInput struct {
	Name       string           `json:"name"`
	Weight     int              `json:"weight"`
	Models     string           `json:"models"`
	Source     string           `json:"source"` // "inline" | "pool"
	InlineKeys []InlineKeyInput `json:"inline_keys"`
	PoolID     *uint            `json:"pool_id"`
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
	AuthType           string           `json:"auth_type"`   // 认证方式：x-api-key 或 bearer，仅用于 Anthropic 类型
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

// InlineKeyItem 内联凭据详情项：掩码展示 + 凭据 ID（编辑弹窗引用该条做受控明文拉取）。
// KeyMasked 由服务端掩码（credentialcrypto.MaskKey），短 key/解密失败退化为 ****，
// 详情响应不含明文——明文仅经受控端点 GET …/inline-keys/raw 显式拉取。
type InlineKeyItem struct {
	ID        uint   `json:"ID"`
	KeyMasked string `json:"KeyMasked"`
}

// InlineKeyRawItem 受控明文端点返回项：按凭据 id 归位，前端整组替换掩码行时不依赖
// 数组顺序——删掩码行未保存、并发保存改动同组，都不会把明文错位归到别的行上。
type InlineKeyRawItem struct {
	ID  uint   `json:"id"`
	Key string `json:"key"`
}

// InlineKeysRawData 内联组明文查看响应（受控端点 GET …/inline-keys/raw）。
type InlineKeysRawData struct {
	Keys []InlineKeyRawItem `json:"keys"`
}

// GroupDetail 详情页分组：内嵌 KeyGroup + 派生 Source + inline 凭据掩码项
// （管理端 JWT 域，安全对齐号池 KeyMasked 体系；明文不出常规响应）。
// PoolID 三态指针禁 omitempty；InlineKeys 空数组表示 inline 无 key 或 pool 来源。
type GroupDetail struct {
	models.KeyGroup
	Source     string          `json:"Source"`
	InlineKeys []InlineKeyItem `json:"InlineKeys"`
}

// ProviderDetail 供应商详情：Provider + 展开的 endpoints/groups（编辑弹窗回填用）。
type ProviderDetail struct {
	models.Provider
	Endpoints []models.Endpoint `json:"Endpoints"`
	Groups    []GroupDetail     `json:"Groups"`
}
