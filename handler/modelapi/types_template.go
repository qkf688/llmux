package modelapi

// ModelTemplateItemResponse 模型模板项响应结构。
type ModelTemplateItemResponse struct {
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
}

// ModelTemplateResponse 模型模板响应结构。
type ModelTemplateResponse struct {
	ModelID   uint                        `json:"model_id"`
	ModelName string                      `json:"model_name"`
	Items     []ModelTemplateItemResponse `json:"items"`
}

// ModelTemplateItemRequest 模型模板项请求结构。
type ModelTemplateItemRequest struct {
	Name string `json:"name"`
}
