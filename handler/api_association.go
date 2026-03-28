package handler

import (
	"context"
	"log/slog"
	"slices"
	"strconv"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ModelWithProviderRequest represents the request body for creating/updating a model-provider association
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
}

// ModelProviderStatusRequest represents the request body for updating provider status
type ModelProviderStatusRequest struct {
	Status bool `json:"status"`
}

// GetModelProviders 获取模型的提供商关联列表
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

	modelProviders, err := gorm.G[models.ModelWithProvider](models.DB).Where("model_id = ?", modelID).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, err.Error())
		return
	}

	common.Success(c, modelProviders)
}

// GetModelProviderStatus 获取提供商状态信息
func GetModelProviderStatus(c *gin.Context) {
	providerIDStr := c.Query("provider_id")
	modelName := c.Query("model_name")
	providerModel := c.Query("provider_model")

	if providerIDStr == "" || modelName == "" || providerModel == "" {
		common.BadRequest(c, "provider_id, model_name and provider_model query parameters are required")
		return
	}

	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid provider_id format")
		return
	}

	// 获取提供商信息
	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", providerID).First(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve provider: "+err.Error())
		return
	}

	// 获取最近10次请求状态
	logs, err := gorm.G[models.ChatLog](models.DB).
		Where("provider_name = ?", provider.Name).
		Where("provider_model = ?", providerModel).
		Where("name = ?", modelName).
		Limit(10).
		Order("created_at DESC").
		Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve chat log: "+err.Error())
		return
	}

	status := make([]bool, 0)
	for _, log := range logs {
		status = append(status, log.Status == "success")
	}
	slices.Reverse(status)
	common.Success(c, status)
}

// GetModelProviderHealthStatus 获取模型提供商的健康检测结果（最近若干次）
func GetModelProviderHealthStatus(c *gin.Context) {
	modelProviderIDStr := c.Query("model_provider_id")
	limitStr := c.Query("limit")

	if modelProviderIDStr == "" {
		common.BadRequest(c, "model_provider_id query parameter is required")
		return
	}

	modelProviderID, err := strconv.ParseUint(modelProviderIDStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid model_provider_id format")
		return
	}

	limit := 10
	if limitStr != "" {
		parsed, parseErr := strconv.Atoi(limitStr)
		if parseErr != nil || parsed < 1 || parsed > 50 {
			common.BadRequest(c, "Invalid limit parameter (must be between 1 and 50)")
			return
		}
		limit = parsed
	}

	logs, err := gorm.G[models.HealthCheckLog](models.DB).
		Where("model_provider_id = ?", modelProviderID).
		Order("checked_at DESC").
		Limit(limit).
		Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve health check logs: "+err.Error())
		return
	}

	status := make([]bool, 0, len(logs))
	for _, log := range logs {
		status = append(status, log.Status == "success")
	}
	slices.Reverse(status)
	common.Success(c, status)
}

// CreateModelProvider 创建模型提供商关联
func CreateModelProvider(c *gin.Context) {
	var req ModelWithProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// 检查模型是否允许关联（无论是自动还是手动）
	model, err := gorm.G[models.Model](models.DB).Where("id = ?", req.ModelID).First(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to get model: "+err.Error())
		return
	}
	
	// 如果模型设置了不允许自动关联，仍然允许手动关联（因为这是用户明确的操作）
	// 但如果将来需要限制手动关联，可以在这里添加检查
	_ = model // 显式忽略变量以避免未使用警告

	customerHeaders := req.CustomerHeaders
	if customerHeaders == nil {
		customerHeaders = map[string]string{}
	}

	// 如果没有指定优先级，使用默认优先级
	priority := req.Priority
	if priority == 0 {
		priority = GetAutoPriorityDecayDefault(c.Request.Context())
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

	err = gorm.G[models.ModelWithProvider](models.DB).Create(c.Request.Context(), &modelProvider)
	if err != nil {
		common.InternalServerError(c, "Failed to create model-provider association: "+err.Error())
		return
	}

	// 检查是否启用关联时自动保存到模板
	if GetSettingBool(c.Request.Context(), models.SettingKeyAutoSaveTemplateOnAssociate) {
		if err := saveProviderModelToTemplate(c.Request.Context(), modelProvider.ModelID, modelProvider.ProviderModel); err != nil {
			slog.Warn("failed to save provider model to template on create",
				"error", err,
				"model_id", modelProvider.ModelID,
				"provider_model", modelProvider.ProviderModel)
		}
	}

	common.Success(c, modelProvider)
}

// UpdateModelProvider 更新模型提供商关联
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

	// Check if model-provider association exists
	_, err = gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Model-provider association not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	// Update fields
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

	if _, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", id).Updates(c.Request.Context(), updates); err != nil {
		common.InternalServerError(c, "Failed to update model-provider association: "+err.Error())
		return
	}

	// Get updated model-provider association
	updatedModelProvider, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve updated model-provider association: "+err.Error())
		return
	}

	common.Success(c, updatedModelProvider)
}

// UpdateModelProviderStatus 切换模型提供商关联启用状态
func UpdateModelProviderStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	var req ModelProviderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	existing, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Model-provider association not found")
			return
		}
		common.InternalServerError(c, "Failed to retrieve model-provider association: "+err.Error())
		return
	}

	status := req.Status
	updates := models.ModelWithProvider{
		Status: &status,
	}
	if status {
		updates.ConsecutiveFailures = 0
	}

	if _, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", id).Updates(c.Request.Context(), updates); err != nil {
		common.InternalServerError(c, "Failed to update status: "+err.Error())
		return
	}

	existing.Status = &status
	common.Success(c, existing)
}

// saveProviderModelToTemplate 将 ProviderModel 保存到模板项
func saveProviderModelToTemplate(ctx context.Context, modelID uint, providerModel string) error {
	if providerModel == "" {
		return nil
	}

	// 检查是否已存在
	count, err := gorm.G[models.ModelTemplateItem](models.DB).
		Where("model_id = ? AND name = ?", modelID, providerModel).
		Count(ctx, "id")
	if err != nil {
		return err
	}

	// 如果不存在则创建
	if count == 0 {
		item := models.ModelTemplateItem{
			ModelID: modelID,
			Name:    providerModel,
		}
		if err := gorm.G[models.ModelTemplateItem](models.DB).Create(ctx, &item); err != nil {
			return err
		}
		slog.Info("auto-saved provider model to template",
			"model_id", modelID,
			"provider_model", providerModel)
	}

	return nil
}

// batchImportExistingAssociations 批量导入现有关联到模板
func batchImportExistingAssociations(ctx context.Context) {
	slog.Info("starting batch import of existing associations to template")

	// 获取所有现有关联
	assocs, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	if err != nil {
		slog.Error("failed to fetch existing associations", "error", err)
		return
	}

	imported := 0
	skipped := 0
	failed := 0

	for _, assoc := range assocs {
		if assoc.ProviderModel == "" {
			skipped++
			continue
		}

		if err := saveProviderModelToTemplate(ctx, assoc.ModelID, assoc.ProviderModel); err != nil {
			slog.Warn("failed to import association to template",
				"model_id", assoc.ModelID,
				"provider_model", assoc.ProviderModel,
				"error", err)
			failed++
		} else {
			imported++
		}
	}

	slog.Info("batch import completed",
		"total", len(assocs),
		"imported", imported,
		"skipped", skipped,
		"failed", failed)
}

// DeleteModelProvider 删除模型提供商关联
func DeleteModelProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	result, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id = ?", id).
		Delete(c.Request.Context())
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

// BatchDeleteModelProvidersRequest represents the request body for batch deleting model-provider associations
type BatchDeleteModelProvidersRequest struct {
	IDs []uint `json:"ids"`
}

// BatchUpdateModelProvidersStatusRequest represents the request body for batch updating model-provider associations status
type BatchUpdateModelProvidersStatusRequest struct {
	IDs    []uint `json:"ids"`
	Status bool   `json:"status"`
}

// BatchUpdateModelProvidersCapabilitiesRequest represents the request body for batch updating model-provider association capabilities.
// A nil capability field means "keep unchanged".
type BatchUpdateModelProvidersCapabilitiesRequest struct {
	IDs              []uint `json:"ids"`
	ToolCall         *bool  `json:"tool_call"`
	StructuredOutput *bool  `json:"structured_output"`
	Image            *bool  `json:"image"`
}

// BatchDeleteModelProviders 批量删除模型提供商关联
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

	ctx := c.Request.Context()

	result, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id IN ?", req.IDs).
		Delete(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to delete model-provider associations: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result,
	})
}

// BatchUpdateModelProvidersStatus 批量更新模型提供商关联状态
func BatchUpdateModelProvidersStatus(c *gin.Context) {
	var req BatchUpdateModelProvidersStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	ctx := c.Request.Context()
	status := req.Status
	updates := models.ModelWithProvider{
		Status: &status,
	}

	// 如果是启用操作，同时重置连续失败次数
	if status {
		updates.ConsecutiveFailures = 0
	}

	result, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id IN ?", req.IDs).
		Updates(ctx, updates)

	if err != nil {
		common.InternalServerError(c, "Failed to update status: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"updated": result,
	})
}

// BatchUpdateModelProvidersCapabilities 批量更新模型提供商关联能力字段（tool_call/structured_output/image）。
func BatchUpdateModelProvidersCapabilities(c *gin.Context) {
	var req BatchUpdateModelProvidersCapabilitiesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	if req.ToolCall == nil && req.StructuredOutput == nil && req.Image == nil {
		common.BadRequest(c, "No capability fields provided")
		return
	}

	updates := models.ModelWithProvider{
		ToolCall:         req.ToolCall,
		StructuredOutput: req.StructuredOutput,
		Image:            req.Image,
	}

	result, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("id IN ?", req.IDs).
		Updates(c.Request.Context(), updates)
	if err != nil {
		common.InternalServerError(c, "Failed to update capabilities: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"updated": result,
	})
}
