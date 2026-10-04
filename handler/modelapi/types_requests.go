package modelapi

// ModelRequest represents the request body for creating/updating a model.
type ModelRequest struct {
	Name          string `json:"name"`
	Remark        string `json:"remark"`
	IOLog         bool   `json:"io_log"`
	AutoAssociate *bool  `json:"auto_associate,omitempty"` // 是否允许自动关联触发
	// SupportsThinking 用 *bool（区别于 Model 实体的 bool）：DTO 需区分"未传=不改"与"传了 false"。
	// 与 AutoAssociate 的 *bool 条件写风格一致，避免 PUT 缺省字段时静默重置存量开启的模型。
	SupportsThinking *bool `json:"supports_thinking,omitempty"` // 是否支持 thinking；nil=不改（Create 时视为 false）
	// ThinkingLevels 思考档位白名单。空切片/nil=不约束（任意档位透传）；非空=只允许白名单内档位。
	// SupportsThinking=false 时此字段被忽略（thinking 整体剥离）。
	// 6 档有序 [minimal, low, medium, high, xhigh, max] + 2 特殊 [none, auto]。
	ThinkingLevels []string `json:"thinking_levels,omitempty"`
	// 能力真值三项（ToolCall 工具调用 / StructuredOutput 结构化输出 / Image 视觉）。
	// 与 SupportsThinking 同范式用 *bool：nil=不改（Create 时视为 false），
	// 供 ModelWithProvider 的对应 override 字段继承（nil=继承本字段）。
	// 绝大多数第三方与官方能力一致，关联侧默认继承，无需逐条标注。
	ToolCall         *bool `json:"tool_call,omitempty"`
	StructuredOutput *bool `json:"structured_output,omitempty"`
	Image            *bool `json:"image,omitempty"`
}

// BatchDeleteModelsRequest represents the request body for batch deleting models.
type BatchDeleteModelsRequest struct {
	IDs []uint `json:"ids"`
}

// BatchUpdateModelsRequest represents the request body for batch updating models.
type BatchUpdateModelsRequest struct {
	IDs           []uint `json:"ids" binding:"required,min=1"`
	AutoAssociate *bool  `json:"auto_associate"` // 指针类型，nil 表示不更新
}
