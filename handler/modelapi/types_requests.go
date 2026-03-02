package modelapi

// ModelRequest represents the request body for creating/updating a model.
type ModelRequest struct {
	Name          string `json:"name"`
	Remark        string `json:"remark"`
	MaxRetry      int    `json:"max_retry"`
	TimeOut       int    `json:"time_out"`
	IOLog         bool   `json:"io_log"`
	AutoAssociate *bool  `json:"auto_associate,omitempty"` // 是否允许自动关联触发
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
