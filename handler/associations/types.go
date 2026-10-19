package associations

// ModelWithProviderRequest represents the request body for creating/updating a model-provider association.
type ModelWithProviderRequest struct {
	ModelID uint `json:"model_id"`
	// ProviderModel 是上游模型名（供应商侧的 model 标识），不是供应商名称。
	// json 键与字段名对齐为 provider_model：旧键名 provider_name 与字段语义错位，
	// 极易被误读成 Provider.Name（本仓其余域的 provider_name 确实都指供应商名称）。
	//
	// 为什么是 *string 三态而非 string：Create 与 Update 共享本 DTO，且缺省语义相反——
	// Create 必填（建关联没有上游名等于建一条注定 400 的关联），Update 缺省=不改（部分更新）。
	// 二态 string 分不出「键缺失」与「显式传空串」，会在 Update 上把「不改」误判成「清了」。
	// 判空一律判 TrimSpace 后为空：Update 走 struct Updates，空串被 GORM 当零值跳过、
	// 只有全空格能被真写进去，只判空串会漏掉这条真实可达路径。
	ProviderModel *string `json:"provider_model"` // 三态：nil 由 handler 按 Create/Update 分别解释
	ProviderID    uint    `json:"provider_id"`
	// 能力三态（与 supports_thinking 同范式）：nil=继承 Model 的能力真值，true/false=override。
	// 用 *bool 而非 bool：bool 无法区分「显式关闭」与「未传」，会把继承态静默改写成 override。
	ToolCall         *bool             `json:"tool_call"`
	StructuredOutput *bool             `json:"structured_output"`
	Image            *bool             `json:"image"`
	WithHeader       bool              `json:"with_header"`
	CustomerHeaders  map[string]string `json:"customer_headers"`
	Weight           int               `json:"weight"`
	Priority         int               `json:"priority"`
	MaxTokens        *int              `json:"max_tokens"`        // max_tokens 上限，nil/0=不限
	SupportsThinking *bool             `json:"supports_thinking"` // 三态：nil=继承 model，true/false=override
	// ThinkingLevels 三态 JSON 契约（json:"thinking_levels,omitempty" + *[]string）：
	// 继承 → 前端不发字段（JSON null）→ 后端 *[]string 为 nil
	// 自定义 → 前端发 ["low","medium"] → 后端 *[]string 指向非空切片
	// 不约束 → 前端发 [] → 后端 *[]string 指向空切片
	ThinkingLevels *[]string `json:"thinking_levels,omitempty"`
	// Protocols / PrimaryProtocol 协议覆盖（三态），语义见 models.ModelWithProvider 同名字段：
	// nil=继承供应商那层；非 nil=覆盖。**缺省（nil）=显式清回继承**（与 supports_thinking 同族，
	// 走 UpdateFields 写 NULL）——前端总是显式提交这两项，不会出现「不想改却被清」的场景。
	// 空切片归一为 nil（一条协议都不许走没有业务含义）。
	Protocols       *[]string `json:"protocols,omitempty"`
	PrimaryProtocol *string   `json:"primary_protocol,omitempty"`
}

// ModelWithProviderProtocolsRequest 是窄更新端点（`PATCH /model-providers/:id/protocols`）
// 的请求体。只承载协议两项；两项都按**显式提交**解释（nil = 写 NULL = 继承），不做部分更新。
type ModelWithProviderProtocolsRequest struct {
	Protocols       *[]string `json:"protocols"`
	PrimaryProtocol *string   `json:"primary_protocol"`
}

// ModelProviderStatusRequest represents the request body for updating provider status.
type ModelProviderStatusRequest struct {
	Status bool `json:"status"`
}

// BatchDeleteModelProvidersRequest represents the request body for batch deleting model-provider associations.
type BatchDeleteModelProvidersRequest struct {
	IDs []uint `json:"ids"`
}

// BatchUpdateModelProvidersStatusRequest represents the request body for batch updating status.
type BatchUpdateModelProvidersStatusRequest struct {
	IDs    []uint `json:"ids"`
	Status bool   `json:"status"`
}

// BatchUpdateModelProvidersCapabilitiesRequest represents the request body for batch updating capabilities.
// A nil capability field means "keep unchanged".
type BatchUpdateModelProvidersCapabilitiesRequest struct {
	IDs              []uint `json:"ids"`
	ToolCall         *bool  `json:"tool_call"`
	StructuredOutput *bool  `json:"structured_output"`
	Image            *bool  `json:"image"`
}
