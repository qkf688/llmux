package associations

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetModelProviders 获取模型的提供商关联列表。
func GetModelProviders(c *gin.Context) {
	modelIDStr := c.Query("model_id")
	if modelIDStr == "" {
		common.BadRequest(c, "model_id query parameter is required")
		return
	}

	modelID, err := strconv.ParseUint(modelIDStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid model_id format")
		return
	}

	modelProviders, err := repos().ModelWithProvider.ListByModelID(c.Request.Context(), uint(modelID))
	if err != nil {
		common.InternalServerError(c, err.Error())
		return
	}

	common.Success(c, modelProviders)
}

// CreateModelProvider 创建模型提供商关联。
func CreateModelProvider(c *gin.Context) {
	var req ModelWithProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// 检查模型是否存在（手动关联始终允许，与现网一致）
	if _, err := repos().Model.Get(ctx, req.ModelID); err != nil {
		common.InternalServerError(c, "Failed to get model: "+err.Error())
		return
	}

	customerHeaders := req.CustomerHeaders
	if customerHeaders == nil {
		customerHeaders = map[string]string{}
	}

	priority := req.Priority
	if priority == 0 {
		// 与 settings.GetAutoPriorityDecayDefault 默认值一致
		priority = models.GetSettingInt(ctx, models.SettingKeyAutoPriorityDecayDefault, 100, 0)
	}

	modelProvider := models.ModelWithProvider{
		ModelID:          req.ModelID,
		ProviderModel:    req.ProviderModel,
		ProviderID:       req.ProviderID,
		ToolCall:         &req.ToolCall,
		StructuredOutput: &req.StructuredOutput,
		Image:            &req.Image,
		WithHeader:       &req.WithHeader,
		CustomerHeaders:  customerHeaders,
		Weight:           req.Weight,
		Priority:         priority,
	}

	defaultStatus := true
	modelProvider.Status = &defaultStatus

	if err := repos().ModelWithProvider.Create(ctx, &modelProvider); err != nil {
		common.InternalServerError(c, "Failed to create model-provider association: "+err.Error())
		return
	}

	if models.GetSettingBool(ctx, models.SettingKeyAutoSaveTemplateOnAssociate, false) {
		if err := SaveProviderModelToTemplate(ctx, modelProvider.ModelID, modelProvider.ProviderModel); err != nil {
			slog.Warn("failed to save provider model to template on create",
				"error", err,
				"model_id", modelProvider.ModelID,
				"provider_model", modelProvider.ProviderModel)
		}
	}

	common.Success(c, modelProvider)
}

// UpdateModelProvider 更新模型提供商关联。
func UpdateModelProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	var req ModelWithProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}
	slog.Info("UpdateModelProvider", "req", req)

	customerHeaders := req.CustomerHeaders
	if customerHeaders == nil {
		customerHeaders = map[string]string{}
	}

	ctx := c.Request.Context()
	if _, err := repos().ModelWithProvider.Get(ctx, uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.NotFound(c, "Model-provider association not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	updates := models.ModelWithProvider{
		ModelID:          req.ModelID,
		ProviderID:       req.ProviderID,
		ProviderModel:    req.ProviderModel,
		ToolCall:         &req.ToolCall,
		StructuredOutput: &req.StructuredOutput,
		Image:            &req.Image,
		WithHeader:       &req.WithHeader,
		CustomerHeaders:  customerHeaders,
		Weight:           req.Weight,
		Priority:         req.Priority,
	}

	if err := repos().ModelWithProvider.Update(ctx, uint(id), updates); err != nil {
		common.InternalServerError(c, "Failed to update model-provider association: "+err.Error())
		return
	}

	updatedModelProvider, err := repos().ModelWithProvider.Get(ctx, uint(id))
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve updated model-provider association: "+err.Error())
		return
	}

	common.Success(c, updatedModelProvider)
}

// DeleteModelProvider 删除模型提供商关联。
func DeleteModelProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	result, err := repos().ModelWithProvider.Delete(c.Request.Context(), uint(id))
	if err != nil {
		common.InternalServerError(c, "Failed to delete model-provider association: "+err.Error())
		return
	}

	if result == 0 {
		common.NotFound(c, "Model-provider association not found")
		return
	}

	common.Success(c, nil)
}

// BatchDeleteModelProviders 批量删除模型提供商关联。
func BatchDeleteModelProviders(c *gin.Context) {
	var req BatchDeleteModelProvidersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	result, err := repos().ModelWithProvider.DeleteByIDs(c.Request.Context(), req.IDs)
	if err != nil {
		common.InternalServerError(c, "Failed to delete model-provider associations: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result,
	})
}