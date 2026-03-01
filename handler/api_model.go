package handler

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ModelRequest represents the request body for creating/updating a model
type ModelRequest struct {
	Name          string `json:"name"`
	Remark        string `json:"remark"`
	MaxRetry      int    `json:"max_retry"`
	TimeOut       int    `json:"time_out"`
	IOLog         bool   `json:"io_log"`
	AutoAssociate *bool  `json:"auto_associate,omitempty"` // 是否允许自动关联触发
}

// GetModels 获取所有模型列表
func GetModels(c *gin.Context) {
	modelsList, err := gorm.G[models.Model](models.DB).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, err.Error())
		return
	}

	common.Success(c, modelsList)
}

// CreateModel 创建模型
func CreateModel(c *gin.Context) {
	var req ModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// Check if model exists
	count, err := gorm.G[models.Model](models.DB).Where("name = ?", req.Name).Count(c.Request.Context(), "id")
	if err != nil {
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if count > 0 {
		common.BadRequest(c, fmt.Sprintf("Model: %s already exists", req.Name))
		return
	}

	model := models.Model{
		Name:          req.Name,
		Remark:        req.Remark,
		MaxRetry:      req.MaxRetry,
		TimeOut:       req.TimeOut,
		IOLog:         &req.IOLog,
		AutoAssociate: req.AutoAssociate,
	}

	if err := gorm.G[models.Model](models.DB).Create(c.Request.Context(), &model); err != nil {
		common.InternalServerError(c, "Failed to create model: "+err.Error())
		return
	}

	common.Success(c, model)
}

// UpdateModel 更新模型
func UpdateModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	var req ModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// Check if model exists
	_, err = gorm.G[models.Model](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Model not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	// Update fields
	updates := models.Model{
		Name:          req.Name,
		Remark:        req.Remark,
		MaxRetry:      req.MaxRetry,
		TimeOut:       req.TimeOut,
		IOLog:         &req.IOLog,
		AutoAssociate: req.AutoAssociate,
	}

	if _, err := gorm.G[models.Model](models.DB).Where("id = ?", id).Updates(c.Request.Context(), updates); err != nil {
		common.InternalServerError(c, "Failed to update model: "+err.Error())
		return
	}

	// Get updated model
	updatedModel, err := gorm.G[models.Model](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve updated model: "+err.Error())
		return
	}

	common.Success(c, updatedModel)
}

// DeleteModel 删除模型
func DeleteModel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	result, err := gorm.G[models.Model](models.DB).Where("id = ?", id).Delete(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to delete model: "+err.Error())
		return
	}

	if result == 0 {
		common.NotFound(c, "Model not found")
		return
	}

	common.Success(c, nil)
}

// BatchDeleteModelsRequest represents the request body for batch deleting models
type BatchDeleteModelsRequest struct {
	IDs []uint `json:"ids"`
}

// BatchDeleteModels 批量删除模型
func BatchDeleteModels(c *gin.Context) {
	var req BatchDeleteModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	result, err := gorm.G[models.Model](models.DB).Where("id IN ?", req.IDs).Delete(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to delete models: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result,
	})
}

// BatchUpdateModelsRequest represents the request body for batch updating models
type BatchUpdateModelsRequest struct {
	IDs          []uint  `json:"ids" binding:"required,min=1"`
	MaxRetry     *int    `json:"max_retry"`     // 指针类型，nil 表示不更新
	TimeOut      *int    `json:"time_out"`      // 指针类型，nil 表示不更新
	AutoAssociate *bool  `json:"auto_associate"` // 指针类型，nil 表示不更新
}

// BatchUpdateModels 批量更新模型参数
func BatchUpdateModels(c *gin.Context) {
	var req BatchUpdateModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "参数验证失败: "+err.Error())
		return
	}

	// 验证至少有一个字段需要更新
	if req.MaxRetry == nil && req.TimeOut == nil && req.AutoAssociate == nil {
		common.BadRequest(c, "至少需要更新一个字段")
		return
	}

	// 验证值的合法性
	if req.MaxRetry != nil && *req.MaxRetry < 0 {
		common.BadRequest(c, "重试次数不能为负数")
		return
	}
	if req.TimeOut != nil && *req.TimeOut < 0 {
		common.BadRequest(c, "超时时间不能为负数")
		return
	}

	// 构建更新 map
	updates := make(map[string]interface{})
	if req.MaxRetry != nil {
		updates["max_retry"] = *req.MaxRetry
	}
	if req.TimeOut != nil {
		updates["time_out"] = *req.TimeOut
	}
	if req.AutoAssociate != nil {
		updates["auto_associate"] = req.AutoAssociate
	}

	// 执行批量更新
	result := models.DB.Model(&models.Model{}).
		Where("id IN ?", req.IDs).
		Updates(updates)

	if result.Error != nil {
		common.InternalServerError(c, "更新失败: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"updated": result.RowsAffected,
	})
}

// ModelTemplateItemResponse 模型模板项响应结构
type ModelTemplateItemResponse struct {
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
}

// ModelTemplateResponse 模型模板响应结构
type ModelTemplateResponse struct {
	ModelID   uint                        `json:"model_id"`
	ModelName string                      `json:"model_name"`
	Items     []ModelTemplateItemResponse `json:"items"`
}

// ModelTemplateItemRequest 模型模板项请求结构
type ModelTemplateItemRequest struct {
	Name string `json:"name"`
}

// isUniqueConstraintError 检查是否为唯一约束错误
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "duplicate key")
}

// buildModelTemplateResponse 构建模型模板响应
func buildModelTemplateResponse(
	model models.Model,
	associations []models.ModelWithProvider,
	manualItems []models.ModelTemplateItem,
) ModelTemplateResponse {
	sourceSetByName := make(map[string]map[string]struct{})
	addSource := func(name string, source string) {
		if name == "" || source == "" {
			return
		}
		set, ok := sourceSetByName[name]
		if !ok {
			set = make(map[string]struct{})
			sourceSetByName[name] = set
		}
		set[source] = struct{}{}
	}

	addSource(model.Name, "model_name")
	for _, assoc := range associations {
		addSource(assoc.ProviderModel, "association")
	}
	for _, item := range manualItems {
		addSource(item.Name, "manual")
	}

	items := make([]ModelTemplateItemResponse, 0, len(sourceSetByName))
	for name, sources := range sourceSetByName {
		sourceList := make([]string, 0, len(sources))
		for src := range sources {
			sourceList = append(sourceList, src)
		}
		slices.Sort(sourceList)
		items = append(items, ModelTemplateItemResponse{
			Name:    name,
			Sources: sourceList,
		})
	}
	slices.SortFunc(items, func(a, b ModelTemplateItemResponse) int {
		return strings.Compare(a.Name, b.Name)
	})

	return ModelTemplateResponse{
		ModelID:   model.ID,
		ModelName: model.Name,
		Items:     items,
	}
}

// GetModelTemplate 获取某个 ModelID 的模板（Model.Name + 既有关联 ProviderModel + 手动模板项）
func GetModelTemplate(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	model, err := gorm.G[models.Model](models.DB).Where("id = ?", id).First(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Model not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	associations, err := gorm.G[models.ModelWithProvider](models.DB).Where("model_id = ?", id).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get associations: "+err.Error())
		return
	}

	manualItems, err := gorm.G[models.ModelTemplateItem](models.DB).Where("model_id = ?", id).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get template items: "+err.Error())
		return
	}

	common.Success(c, buildModelTemplateResponse(model, associations, manualItems))
}

// AddModelTemplateItem 添加手动模板项（区分大小写、去重）
func AddModelTemplateItem(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	var req ModelTemplateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		common.BadRequest(c, "Name is required")
		return
	}

	model, err := gorm.G[models.Model](models.DB).Where("id = ?", id).First(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Model not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	count, err := gorm.G[models.ModelTemplateItem](models.DB).
		Where("model_id = ? AND name = ?", id, name).
		Count(ctx, "id")
	if err != nil {
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if count == 0 {
		item := models.ModelTemplateItem{
			ModelID: uint(id),
			Name:    name,
		}
		if err := gorm.G[models.ModelTemplateItem](models.DB).Create(ctx, &item); err != nil {
			if isUniqueConstraintError(err) {
				var existing models.ModelTemplateItem
				restoreErr := models.DB.WithContext(ctx).
					Unscoped().
					Where("model_id = ? AND name = ?", id, name).
					First(&existing).Error
				if restoreErr == nil && existing.DeletedAt.Valid {
					if err := models.DB.WithContext(ctx).
						Unscoped().
						Model(&models.ModelTemplateItem{}).
						Where("id = ?", existing.ID).
						Update("deleted_at", nil).Error; err != nil {
						common.InternalServerError(c, "Failed to restore template item: "+err.Error())
						return
					}
				}
			} else {
				common.InternalServerError(c, "Failed to create template item: "+err.Error())
				return
			}
		}
	}

	associations, err := gorm.G[models.ModelWithProvider](models.DB).Where("model_id = ?", id).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get associations: "+err.Error())
		return
	}
	manualItems, err := gorm.G[models.ModelTemplateItem](models.DB).Where("model_id = ?", id).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get template items: "+err.Error())
		return
	}

	common.Success(c, buildModelTemplateResponse(model, associations, manualItems))
}

// DeleteModelTemplateItem 删除手动模板项
func DeleteModelTemplateItem(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	var req ModelTemplateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		common.BadRequest(c, "Name is required")
		return
	}

	model, err := gorm.G[models.Model](models.DB).Where("id = ?", id).First(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Model not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	if err := models.DB.WithContext(ctx).
		Unscoped().
		Where("model_id = ? AND name = ?", id, name).
		Delete(&models.ModelTemplateItem{}).Error; err != nil {
		common.InternalServerError(c, "Failed to delete template item: "+err.Error())
		return
	}

	associations, err := gorm.G[models.ModelWithProvider](models.DB).Where("model_id = ?", id).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get associations: "+err.Error())
		return
	}
	manualItems, err := gorm.G[models.ModelTemplateItem](models.DB).Where("model_id = ?", id).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get template items: "+err.Error())
		return
	}

	common.Success(c, buildModelTemplateResponse(model, associations, manualItems))
}
