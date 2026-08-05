package modelapi

// ModelRequest represents the request body for creating/updating a model.
type ModelRequest struct {
	Name          string `json:"name"`
	Remark        string `json:"remark"`
	MaxRetry      int    `json:"max_retry"`
	TimeOut       int    `json:"time_out"`
	IOLog         bool   `json:"io_log"`
	AutoAssociate *bool  `json:"auto_associate,omitempty"` // 是否允许自动关联触发
	// SupportsThinking 用 *bool（区别于 Model 实体的 bool）：DTO 需区分"未传=不改"与"传了 false"。
	// 与 AutoAssociate 的 *bool 条件写风格一致，避免 PUT 缺省字段时静默重置存量开启的模型。
	SupportsThinking *bool `json:"supports_thinking,omitempty"` // 是否支持 thinking；nil=不改（Create 时视为 false）
}

// BatchDeleteModelsRequest represents the request body for batch deleting models.
type BatchDeleteModelsRequest struct {
	IDs []uint `json:"ids"`
}

// BatchUpdateModelsRequest represents the request body for batch updating models.
type BatchUpdateModelsRequest struct {
	IDs           []uint `json:"ids" binding:"required,min=1"`
	MaxRetry      *int   `json:"max_retry"`      // 指针类型，nil 表示不更新
	TimeOut       *int   `json:"time_out"`       // 指针类型，nil 表示不更新
	AutoAssociate *bool  `json:"auto_associate"` // 指针类型，nil 表示不更新
}
