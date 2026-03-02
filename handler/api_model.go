package handler

import "github.com/atopos31/llmio/handler/modelapi"

// ModelRequest represents the request body for creating/updating a model（兼容层）。
type ModelRequest = modelapi.ModelRequest

// BatchDeleteModelsRequest represents the request body for batch deleting models（兼容层）。
type BatchDeleteModelsRequest = modelapi.BatchDeleteModelsRequest

// BatchUpdateModelsRequest represents the request body for batch updating models（兼容层）。
type BatchUpdateModelsRequest = modelapi.BatchUpdateModelsRequest

// ModelTemplateItemResponse 模型模板项响应结构（兼容层）。
type ModelTemplateItemResponse = modelapi.ModelTemplateItemResponse

// ModelTemplateResponse 模型模板响应结构（兼容层）。
type ModelTemplateResponse = modelapi.ModelTemplateResponse

// ModelTemplateItemRequest 模型模板项请求结构（兼容层）。
type ModelTemplateItemRequest = modelapi.ModelTemplateItemRequest

// GetModels 获取所有模型列表（兼容层）。
var GetModels = modelapi.GetModels

// CreateModel 创建模型（兼容层）。
var CreateModel = modelapi.CreateModel

// UpdateModel 更新模型（兼容层）。
var UpdateModel = modelapi.UpdateModel

// DeleteModel 删除模型（兼容层）。
var DeleteModel = modelapi.DeleteModel

// BatchDeleteModels 批量删除模型（兼容层）。
var BatchDeleteModels = modelapi.BatchDeleteModels

// BatchUpdateModels 批量更新模型参数（兼容层）。
var BatchUpdateModels = modelapi.BatchUpdateModels

// GetModelTemplate 获取模型模板（兼容层）。
var GetModelTemplate = modelapi.GetModelTemplate

// AddModelTemplateItem 添加手动模板项（兼容层）。
var AddModelTemplateItem = modelapi.AddModelTemplateItem

// DeleteModelTemplateItem 删除手动模板项（兼容层）。
var DeleteModelTemplateItem = modelapi.DeleteModelTemplateItem
