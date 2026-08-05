package associations

// ModelWithProviderRequest represents the request body for creating/updating a model-provider association.
type ModelWithProviderRequest struct {
	ModelID          uint              `json:"model_id"`
	ProviderModel    string            `json:"provider_name"`
	ProviderID       uint              `json:"provider_id"`
	ToolCall         bool              `json:"tool_call"`
	StructuredOutput bool              `json:"structured_output"`
	Image            bool              `json:"image"`
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
