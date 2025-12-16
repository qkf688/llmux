package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ProviderRequest represents the request body for creating/updating a provider
type ProviderRequest struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Config  string `json:"config"`
	Console string `json:"console"`
	Proxy   string `json:"proxy"`
}

// ModelRequest represents the request body for creating/updating a model
type ModelRequest struct {
	Name     string `json:"name"`
	Remark   string `json:"remark"`
	MaxRetry int    `json:"max_retry"`
	TimeOut  int    `json:"time_out"`
	IOLog    bool   `json:"io_log"`
}

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

// SystemConfigRequest represents the request body for updating system configuration
type SystemConfigRequest struct {
	EnableSmartRouting  bool    `json:"enable_smart_routing"`
	SuccessRateWeight   float64 `json:"success_rate_weight"`
	ResponseTimeWeight  float64 `json:"response_time_weight"`
	DecayThresholdHours int     `json:"decay_threshold_hours"`
	MinWeight           int     `json:"min_weight"`
}

// GetProviders 获取所有提供商列表（支持名称搜索和类型筛选）
func GetProviders(c *gin.Context) {
	// 筛选参数
	name := c.Query("name")
	providerType := c.Query("type")

	// 构建查询条件
	query := models.DB.Model(&models.Provider{}).WithContext(c.Request.Context())

	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	if providerType != "" {
		query = query.Where("type = ?", providerType)
	}
	var providers []models.Provider
	if err := query.Find(&providers).Error; err != nil {
		common.InternalServerError(c, err.Error())
		return
	}

	common.Success(c, providers)
}

func GetProviderModels(c *gin.Context) {
	id := c.Param("id")
	source := c.Query("source")
	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, err.Error())
		return
	}

	config := provider.Config
	if source == "upstream" {
		if cleanedConfig, err := dropCustomModels(config); err == nil {
			config = cleanedConfig
		} else {
			slog.Warn("failed to strip custom models for upstream refresh", "provider_id", id, "error", err)
		}
	}

	chatModel, err := providers.New(provider.Type, config, provider.Proxy)
	if err != nil {
		common.InternalServerError(c, "Failed to get models: "+err.Error())
		return
	}
	models, err := chatModel.Models(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Failed to get models: "+err.Error())
		return
	}
	// 确保返回的是数组而不是 nil，避免前端白屏
	if models == nil {
		models = []providers.Model{}
	}
	common.Success(c, models)
}

func dropCustomModels(config string) (string, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return "", err
	}

	delete(parsed, "custom_models")

	updated, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(updated), nil
}

// CreateProvider 创建提供商
func CreateProvider(c *gin.Context) {
	var req ProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// Check if provider exists
	count, err := gorm.G[models.Provider](models.DB).Where("name = ?", req.Name).Count(c.Request.Context(), "id")
	if err != nil {
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	if count > 0 {
		common.BadRequest(c, "Provider already exists")
		return
	}

	provider := models.Provider{
		Name:    req.Name,
		Type:    req.Type,
		Config:  req.Config,
		Console: req.Console,
		Proxy:   req.Proxy,
	}

	if err := gorm.G[models.Provider](models.DB).Create(c.Request.Context(), &provider); err != nil {
		common.InternalServerError(c, "Failed to create provider: "+err.Error())
		return
	}

	common.Success(c, provider)
}

// UpdateProvider 更新提供商
func UpdateProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	var req ProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// Check if provider exists
	if _, err := gorm.G[models.Provider](models.DB).Where("id = ?", id).First(c.Request.Context()); err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Provider not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	// Update fields
	updates := models.Provider{
		Name:    req.Name,
		Type:    req.Type,
		Config:  req.Config,
		Console: req.Console,
		Proxy:   req.Proxy,
	}

	if _, err := gorm.G[models.Provider](models.DB).Where("id = ?", id).Updates(c.Request.Context(), updates); err != nil {
		common.InternalServerError(c, "Failed to update provider: "+err.Error())
		return
	}

	// Get updated provider
	updatedProvider, err := gorm.G[models.Provider](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve updated provider: "+err.Error())
		return
	}

	common.Success(c, updatedProvider)
}

// DeleteProvider 删除提供商
func DeleteProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	result, err := gorm.G[models.Provider](models.DB).Where("id = ?", id).Delete(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to delete provider: "+err.Error())
		return
	}

	//删除关联
	if _, err := gorm.G[models.ModelWithProvider](models.DB).Where("provider_id = ?", id).Delete(c.Request.Context()); err != nil {
		common.InternalServerError(c, "Failed to delete provider: "+err.Error())
		return
	}

	if result == 0 {
		common.NotFound(c, "Provider not found")
		return
	}

	common.Success(c, nil)
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
		Name:     req.Name,
		Remark:   req.Remark,
		MaxRetry: req.MaxRetry,
		TimeOut:  req.TimeOut,
		IOLog:    &req.IOLog,
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
		Name:     req.Name,
		Remark:   req.Remark,
		MaxRetry: req.MaxRetry,
		TimeOut:  req.TimeOut,
		IOLog:    &req.IOLog,
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

type ProviderTemplate struct {
	Type     string `json:"type"`
	Template string `json:"template"`
}

var template = []ProviderTemplate{
	{
		Type: "openai",
		Template: `{
			"base_url": "https://api.openai.com/v1",
			"api_key": "YOUR_API_KEY"
		}`,
	},
	{
		Type: "openai-res",
		Template: `{
			"base_url": "https://api.openai.com/v1",
			"api_key": "YOUR_API_KEY"
		}`,
	},
	{
		Type: "anthropic",
		Template: `{
			"base_url": "https://api.anthropic.com/v1",
			"api_key": "YOUR_API_KEY",
			"beta": "",
			"version": "2023-06-01"
		}`,
	},
}

func GetProviderTemplates(c *gin.Context) {
	common.Success(c, template)
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

	customerHeaders := req.CustomerHeaders
	if customerHeaders == nil {
		customerHeaders = map[string]string{}
	}

	// 如果没有指定优先级，使用默认优先级
	priority := req.Priority
	if priority == 0 {
		priority = getAutoPriorityDecayDefault(c.Request.Context())
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

	err := gorm.G[models.ModelWithProvider](models.DB).Create(c.Request.Context(), &modelProvider)
	if err != nil {
		common.InternalServerError(c, "Failed to create model-provider association: "+err.Error())
		return
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

	if _, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", id).Updates(c.Request.Context(), updates); err != nil {
		common.InternalServerError(c, "Failed to update status: "+err.Error())
		return
	}

	existing.Status = &status
	common.Success(c, existing)
}

// DeleteModelProvider 删除模型提供商关联
func DeleteModelProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	result, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", id).Delete(c.Request.Context())
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

	result, err := gorm.G[models.ModelWithProvider](models.DB).Where("id IN ?", req.IDs).Delete(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to delete model-provider associations: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result,
	})
}

// GetRequestLogs 获取最近的请求日志（支持分页和筛选）
func GetRequestLogs(c *gin.Context) {
	// 分页参数
	pageStr := c.Query("page")
	page := 1
	if pageStr != "" {
		parsedPage, err := strconv.Atoi(pageStr)
		if err != nil || parsedPage < 1 {
			common.BadRequest(c, "Invalid page parameter")
			return
		}
		page = parsedPage
	}

	pageSizeStr := c.Query("page_size")
	pageSize := 20 // Default page size
	if pageSizeStr != "" {
		parsedPageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || parsedPageSize < 1 || parsedPageSize > 100 {
			common.BadRequest(c, "Invalid page_size parameter (must be between 1 and 100)")
			return
		}
		pageSize = parsedPageSize
	}

	// 筛选参数
	providerName := c.Query("provider_name")
	name := c.Query("name")
	status := c.Query("status")
	style := c.Query("style")
	userAgent := c.Query("user_agent")

	// 构建查询条件
	query := models.DB.Model(&models.ChatLog{})

	if providerName != "" {
		query = query.Where("provider_name = ?", providerName)
	}

	if name != "" {
		query = query.Where("name = ?", name)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if style != "" {
		query = query.Where("style = ?", style)
	}

	if userAgent != "" {
		query = query.Where("user_agent = ?", userAgent)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.InternalServerError(c, "Failed to count logs: "+err.Error())
		return
	}

	// 获取分页数据
	var logs []models.ChatLog
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		common.InternalServerError(c, "Failed to query logs: "+err.Error())
		return
	}

	result := map[string]any{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (total + int64(pageSize) - 1) / int64(pageSize),
	}

	common.Success(c, result)
}

// GetChatIO 查询指定日志的输入输出记录
func GetChatIO(c *gin.Context) {
	id := c.Param("id")

	chatIO, err := gorm.G[models.ChatIO](models.DB).Where("log_id = ?", id).First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "ChatIO not found")
		return
	}

	common.Success(c, chatIO)
}

// GetSystemConfig 获取系统配置
func GetSystemConfig(c *gin.Context) {
	config := map[string]interface{}{
		"enable_smart_routing":  true,
		"success_rate_weight":   0.7,
		"response_time_weight":  0.3,
		"decay_threshold_hours": 24,
		"min_weight":            1,
	}

	common.Success(c, config)
}

// UpdateSystemConfig 更新系统配置
func UpdateSystemConfig(c *gin.Context) {
	var req SystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	config := map[string]interface{}{
		"enable_smart_routing":  req.EnableSmartRouting,
		"success_rate_weight":   req.SuccessRateWeight,
		"response_time_weight":  req.ResponseTimeWeight,
		"decay_threshold_hours": req.DecayThresholdHours,
		"min_weight":            req.MinWeight,
	}

	common.Success(c, config)
}

// GetUserAgents 获取所有不重复的用户代理种类
func GetUserAgents(c *gin.Context) {
	var userAgents []string

	// 查询所有不重复的非空用户代理
	if err := models.DB.Model(&models.ChatLog{}).
		Where("user_agent IS NOT NULL AND user_agent != ''").
		Distinct("user_agent").
		Pluck("user_agent", &userAgents).
		Error; err != nil {
		common.InternalServerError(c, "Failed to query user agents: "+err.Error())
		return
	}

	common.Success(c, userAgents)
}

// SettingsResponse 设置响应结构
type SettingsResponse struct {
	StrictCapabilityMatch           bool `json:"strict_capability_match"`
	AutoWeightDecay                 bool `json:"auto_weight_decay"`
	AutoWeightDecayDefault          int  `json:"auto_weight_decay_default"`
	AutoWeightDecayStep             int  `json:"auto_weight_decay_step"`
	AutoSuccessIncrease             bool `json:"auto_success_increase"`
	AutoWeightIncreaseStep          int  `json:"auto_weight_increase_step"`
	AutoWeightIncreaseMax           int  `json:"auto_weight_increase_max"`
	AutoPriorityDecay               bool `json:"auto_priority_decay"`
	AutoPriorityDecayDefault        int  `json:"auto_priority_decay_default"`
	AutoPriorityDecayStep           int  `json:"auto_priority_decay_step"`
	AutoPriorityDecayThreshold      int  `json:"auto_priority_decay_threshold"`
	AutoPriorityDecayDisableEnabled bool `json:"auto_priority_decay_disable_enabled"`
	AutoPriorityIncreaseStep        int  `json:"auto_priority_increase_step"`
	AutoPriorityIncreaseMax         int  `json:"auto_priority_increase_max"`
	LogRetentionCount               int  `json:"log_retention_count"`
	LogRawRequestResponse           bool `json:"log_raw_request_response"`
	DisableAllLogs                  bool `json:"disable_all_logs"`
	CountHealthCheckAsSuccess       bool `json:"count_health_check_as_success"`
	CountHealthCheckAsFailure       bool `json:"count_health_check_as_failure"`
	// 性能优化相关设置
	DisablePerformanceTracking bool `json:"disable_performance_tracking"`
	DisableTokenCounting       bool `json:"disable_token_counting"`
	EnableRequestTrace         bool `json:"enable_request_trace"`
}

// UpdateSettingsRequest 更新设置请求结构
type UpdateSettingsRequest struct {
	StrictCapabilityMatch           bool `json:"strict_capability_match"`
	AutoWeightDecay                 bool `json:"auto_weight_decay"`
	AutoWeightDecayDefault          int  `json:"auto_weight_decay_default"`
	AutoWeightDecayStep             int  `json:"auto_weight_decay_step"`
	AutoSuccessIncrease             bool `json:"auto_success_increase"`
	AutoWeightIncreaseStep          int  `json:"auto_weight_increase_step"`
	AutoWeightIncreaseMax           int  `json:"auto_weight_increase_max"`
	AutoPriorityDecay               bool `json:"auto_priority_decay"`
	AutoPriorityDecayDefault        int  `json:"auto_priority_decay_default"`
	AutoPriorityDecayStep           int  `json:"auto_priority_decay_step"`
	AutoPriorityDecayThreshold      int  `json:"auto_priority_decay_threshold"`
	AutoPriorityDecayDisableEnabled bool `json:"auto_priority_decay_disable_enabled"`
	AutoPriorityIncreaseStep        int  `json:"auto_priority_increase_step"`
	AutoPriorityIncreaseMax         int  `json:"auto_priority_increase_max"`
	LogRetentionCount               int  `json:"log_retention_count"`
	LogRawRequestResponse           bool `json:"log_raw_request_response"`
	DisableAllLogs                  bool `json:"disable_all_logs"`
	CountHealthCheckAsSuccess       bool `json:"count_health_check_as_success"`
	CountHealthCheckAsFailure       bool `json:"count_health_check_as_failure"`
	// 性能优化相关设置
	DisablePerformanceTracking bool `json:"disable_performance_tracking"`
	DisableTokenCounting       bool `json:"disable_token_counting"`
	EnableRequestTrace         bool `json:"enable_request_trace"`
}

// GetSettings 获取所有设置
func GetSettings(c *gin.Context) {
	settings, err := gorm.G[models.Setting](models.DB).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to get settings: "+err.Error())
		return
	}

	// 构建响应
	response := SettingsResponse{
		StrictCapabilityMatch:           true, // 默认值
		AutoWeightDecay:                 false,
		AutoWeightDecayDefault:          100,
		AutoWeightDecayStep:             1,
		AutoSuccessIncrease:             true,
		AutoWeightIncreaseStep:          1,
		AutoWeightIncreaseMax:           100,
		AutoPriorityDecay:               false,
		AutoPriorityDecayDefault:        100,
		AutoPriorityDecayStep:           1,
		AutoPriorityDecayThreshold:      90,
		AutoPriorityDecayDisableEnabled: true, // 默认启用自动禁用功能
		LogRetentionCount:               100,  // 默认保留100条
		AutoPriorityIncreaseStep:        1,
		AutoPriorityIncreaseMax:         100,
		CountHealthCheckAsSuccess:       true,
		CountHealthCheckAsFailure:       false,
		// 性能优化相关默认值
		DisablePerformanceTracking: false,
		DisableTokenCounting:       false,
		EnableRequestTrace:         true, // 默认启用
	}

	for _, setting := range settings {
		switch setting.Key {
		case models.SettingKeyStrictCapabilityMatch:
			response.StrictCapabilityMatch = setting.Value == "true"
		case models.SettingKeyAutoWeightDecay:
			response.AutoWeightDecay = setting.Value == "true"
		case models.SettingKeyAutoWeightDecayDefault:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoWeightDecayDefault = val
			}
		case models.SettingKeyAutoWeightDecayStep:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoWeightDecayStep = val
			}
		case models.SettingKeyAutoSuccessIncrease:
			response.AutoSuccessIncrease = setting.Value == "true"
		case models.SettingKeyAutoWeightIncreaseStep:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoWeightIncreaseStep = val
			}
		case models.SettingKeyAutoWeightIncreaseMax:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoWeightIncreaseMax = val
			}
		case models.SettingKeyAutoPriorityDecay:
			response.AutoPriorityDecay = setting.Value == "true"
		case models.SettingKeyAutoPriorityDecayDefault:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoPriorityDecayDefault = val
			}
		case models.SettingKeyAutoPriorityDecayStep:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoPriorityDecayStep = val
			}
		case models.SettingKeyAutoPriorityDecayThreshold:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoPriorityDecayThreshold = val
			}
		case models.SettingKeyAutoPriorityDecayDisableEnabled:
			response.AutoPriorityDecayDisableEnabled = setting.Value == "true"
		case models.SettingKeyAutoPriorityIncreaseStep:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoPriorityIncreaseStep = val
			}
		case models.SettingKeyAutoPriorityIncreaseMax:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoPriorityIncreaseMax = val
			}
		case models.SettingKeyLogRetentionCount:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.LogRetentionCount = val
			}
		case models.SettingKeyLogRawRequestResponse:
			response.LogRawRequestResponse = setting.Value == "true"
		case models.SettingKeyDisableAllLogs:
			response.DisableAllLogs = setting.Value == "true"
		case models.SettingKeyHealthCheckCountAsSuccess:
			response.CountHealthCheckAsSuccess = setting.Value == "true"
		case models.SettingKeyHealthCheckCountAsFailure:
			response.CountHealthCheckAsFailure = setting.Value == "true"
		case models.SettingKeyDisablePerformanceTracking:
			response.DisablePerformanceTracking = setting.Value == "true"
		case models.SettingKeyDisableTokenCounting:
			response.DisableTokenCounting = setting.Value == "true"
		case models.SettingKeyEnableRequestTrace:
			response.EnableRequestTrace = setting.Value == "true"
		}
	}

	common.Success(c, response)
}

// UpdateSettings 更新设置
func UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// 更新严格能力匹配设置
	strictValue := "false"
	if req.StrictCapabilityMatch {
		strictValue = "true"
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyStrictCapabilityMatch).
		Update(ctx, "value", strictValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动权重衰减开关
	autoWeightDecayValue := "false"
	if req.AutoWeightDecay {
		autoWeightDecayValue = "true"
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecay).
		Update(ctx, "value", autoWeightDecayValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动权重衰减默认值
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecayDefault).
		Update(ctx, "value", strconv.Itoa(req.AutoWeightDecayDefault)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动权重衰减步长
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecayStep).
		Update(ctx, "value", strconv.Itoa(req.AutoWeightDecayStep)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if req.AutoSuccessIncrease {
		if req.AutoWeightIncreaseStep < 1 {
			req.AutoWeightIncreaseStep = 1
		}
		if req.AutoWeightIncreaseMax < 1 {
			req.AutoWeightIncreaseMax = 100
		}
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoSuccessIncrease).
		Update(ctx, "value", strconv.FormatBool(req.AutoSuccessIncrease)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightIncreaseStep).
		Update(ctx, "value", strconv.Itoa(req.AutoWeightIncreaseStep)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightIncreaseMax).
		Update(ctx, "value", strconv.Itoa(req.AutoWeightIncreaseMax)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动优先级衰减开关
	autoPriorityDecayValue := "false"
	if req.AutoPriorityDecay {
		autoPriorityDecayValue = "true"
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecay).
		Update(ctx, "value", autoPriorityDecayValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动优先级衰减默认值
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayDefault).
		Update(ctx, "value", strconv.Itoa(req.AutoPriorityDecayDefault)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动优先级衰减步长
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayStep).
		Update(ctx, "value", strconv.Itoa(req.AutoPriorityDecayStep)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动优先级衰减阈值
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayThreshold).
		Update(ctx, "value", strconv.Itoa(req.AutoPriorityDecayThreshold)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动优先级衰减禁用开关
	autoPriorityDecayDisableEnabledValue := "false"
	if req.AutoPriorityDecayDisableEnabled {
		autoPriorityDecayDisableEnabledValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayDisableEnabled).
		Update(ctx, "value", autoPriorityDecayDisableEnabledValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if req.AutoPriorityIncreaseStep < 1 {
		req.AutoPriorityIncreaseStep = 1
	}
	if req.AutoPriorityIncreaseMax < 0 {
		req.AutoPriorityIncreaseMax = 100
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityIncreaseStep).
		Update(ctx, "value", strconv.Itoa(req.AutoPriorityIncreaseStep)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityIncreaseMax).
		Update(ctx, "value", strconv.Itoa(req.AutoPriorityIncreaseMax)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	countHealthCheckValue := "false"
	if req.CountHealthCheckAsSuccess {
		countHealthCheckValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsSuccess).
		Update(ctx, "value", countHealthCheckValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	countHealthCheckFailureValue := "false"
	if req.CountHealthCheckAsFailure {
		countHealthCheckFailureValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsFailure).
		Update(ctx, "value", countHealthCheckFailureValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新日志保留条数设置
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyLogRetentionCount).
		Update(ctx, "value", strconv.Itoa(req.LogRetentionCount)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新原始请求响应记录开关
	logRawValue := "false"
	if req.LogRawRequestResponse {
		logRawValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyLogRawRequestResponse).
		Update(ctx, "value", logRawValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新完全关闭日志记录开关
	disableAllLogsValue := "false"
	if req.DisableAllLogs {
		disableAllLogsValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyDisableAllLogs).
		Update(ctx, "value", disableAllLogsValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新性能优化相关设置
	disablePerformanceTrackingValue := "false"
	if req.DisablePerformanceTracking {
		disablePerformanceTrackingValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyDisablePerformanceTracking).
		Update(ctx, "value", disablePerformanceTrackingValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	disableTokenCountingValue := "false"
	if req.DisableTokenCounting {
		disableTokenCountingValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyDisableTokenCounting).
		Update(ctx, "value", disableTokenCountingValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	enableRequestTraceValue := "false"
	if req.EnableRequestTrace {
		enableRequestTraceValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyEnableRequestTrace).
		Update(ctx, "value", enableRequestTraceValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 如果设置了保留条数限制，立即执行清理
	if req.LogRetentionCount > 0 {
		go cleanupExcessLogs(req.LogRetentionCount)
	}

	// 返回更新后的设置
	GetSettings(c)
}

// cleanupExcessLogs 清理超出保留条数的日志
func cleanupExcessLogs(retentionCount int) {
	ctx := context.Background()

	// 获取总日志数
	var total int64
	if err := models.DB.Model(&models.ChatLog{}).Count(&total).Error; err != nil {
		slog.Error("failed to count logs for cleanup", "error", err)
		return
	}

	// 如果日志数超过保留条数，删除多余的
	if int(total) > retentionCount {
		deleteCount := int(total) - retentionCount

		// 获取需要删除的日志ID（最旧的）
		var logsToDelete []models.ChatLog
		if err := models.DB.Model(&models.ChatLog{}).
			Order("id ASC").
			Limit(deleteCount).
			Find(&logsToDelete).Error; err != nil {
			slog.Error("failed to find logs to delete", "error", err)
			return
		}

		// 提取ID列表
		ids := make([]uint, len(logsToDelete))
		for i, log := range logsToDelete {
			ids[i] = log.ID
		}

		// 删除对应的ChatIO记录
		if _, err := gorm.G[models.ChatIO](models.DB).
			Where("log_id IN ?", ids).
			Delete(ctx); err != nil {
			slog.Error("failed to delete chat io records", "error", err)
		}

		// 删除日志记录
		if _, err := gorm.G[models.ChatLog](models.DB).
			Where("id IN ?", ids).
			Delete(ctx); err != nil {
			slog.Error("failed to delete logs", "error", err)
			return
		}

		slog.Info("cleaned up excess logs", "deleted", deleteCount, "retention", retentionCount)
	}
}

// ResetModelWeightsRequest 重置模型权重请求结构
type ResetModelWeightsRequest struct {
	ModelID *uint `json:"model_id"` // 可选，为空时重置所有模型的权重
}

// ResetModelWeights 重置模型关联的权重到默认值
func ResetModelWeights(c *gin.Context) {
	var req ResetModelWeightsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// 获取默认权重值
	defaultWeight := 100
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecayDefault).
		First(ctx)
	if err == nil {
		if val, err := strconv.Atoi(setting.Value); err == nil {
			defaultWeight = val
		}
	}

	// 更新权重
	var result int
	if req.ModelID != nil {
		result, err = gorm.G[models.ModelWithProvider](models.DB).
			Where("model_id = ?", *req.ModelID).
			Update(ctx, "weight", defaultWeight)
	} else {
		// 使用 Where("1 = 1") 来允许全表更新
		result, err = gorm.G[models.ModelWithProvider](models.DB).
			Where("1 = 1").
			Update(ctx, "weight", defaultWeight)
	}

	if err != nil {
		common.InternalServerError(c, "Failed to reset weights: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"updated":        result,
		"default_weight": defaultWeight,
	})
}

// ResetModelPrioritiesRequest 重置模型优先级请求结构
type ResetModelPrioritiesRequest struct {
	ModelID *uint `json:"model_id"` // 可选，为空时重置所有模型的优先级
}

// ResetModelPriorities 重置模型关联的优先级到默认值
func ResetModelPriorities(c *gin.Context) {
	var req ResetModelPrioritiesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// 获取默认优先级值
	defaultPriority := getAutoPriorityDecayDefault(ctx)

	// 更新优先级
	var result int
	var err error
	if req.ModelID != nil {
		result, err = gorm.G[models.ModelWithProvider](models.DB).
			Where("model_id = ?", *req.ModelID).
			Update(ctx, "priority", defaultPriority)
	} else {
		// 使用 Where("1 = 1") 来允许全表更新
		result, err = gorm.G[models.ModelWithProvider](models.DB).
			Where("1 = 1").
			Update(ctx, "priority", defaultPriority)
	}

	if err != nil {
		common.InternalServerError(c, "Failed to reset priorities: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"updated":          result,
		"default_priority": defaultPriority,
	})
}

// EnableAllAssociationsRequest 启用所有关联请求结构
type EnableAllAssociationsRequest struct {
	ModelID *uint `json:"model_id"` // 可选，为空时启用所有模型的关联
}

// EnableAllAssociations 启用所有模型关联
func EnableAllAssociations(c *gin.Context) {
	var req EnableAllAssociationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// 启用所有关联模型
	trueVal := true
	var result int
	var err error
	if req.ModelID != nil {
		result, err = gorm.G[models.ModelWithProvider](models.DB).
			Where("model_id = ?", *req.ModelID).
			Updates(ctx, models.ModelWithProvider{Status: &trueVal})
	} else {
		result, err = gorm.G[models.ModelWithProvider](models.DB).
			Where("1 = 1").
			Updates(ctx, models.ModelWithProvider{Status: &trueVal})
	}

	if err != nil {
		common.InternalServerError(c, "Failed to enable associations: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"updated": result,
	})
}

// GetStrictCapabilityMatch 获取严格能力匹配设置
func GetStrictCapabilityMatch(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyStrictCapabilityMatch).
		First(ctx)
	if err != nil {
		return true // 默认开启
	}
	return setting.Value == "true"
}

// getAutoPriorityDecayDefault 获取自动优先级衰减默认值
func getAutoPriorityDecayDefault(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayDefault).
		First(ctx)
	if err != nil {
		return 100 // 默认优先级100
	}
	val, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 100
	}
	return val
}

// DeleteLog 删除单条日志
func DeleteLog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	ctx := c.Request.Context()

	// 删除对应的ChatIO记录
	if _, err := gorm.G[models.ChatIO](models.DB).
		Where("log_id = ?", id).
		Delete(ctx); err != nil {
		slog.Warn("failed to delete chat io record", "log_id", id, "error", err)
	}

	// 删除日志记录
	result, err := gorm.G[models.ChatLog](models.DB).
		Where("id = ?", id).
		Delete(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to delete log: "+err.Error())
		return
	}

	if result == 0 {
		common.NotFound(c, "Log not found")
		return
	}

	common.Success(c, nil)
}

// BatchDeleteLogsRequest 批量删除日志请求结构
type BatchDeleteLogsRequest struct {
	IDs []uint `json:"ids"`
}

// BatchDeleteLogs 批量删除日志
func BatchDeleteLogs(c *gin.Context) {
	var req BatchDeleteLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	ctx := c.Request.Context()

	// 删除对应的ChatIO记录
	if _, err := gorm.G[models.ChatIO](models.DB).
		Where("log_id IN ?", req.IDs).
		Delete(ctx); err != nil {
		slog.Warn("failed to delete chat io records", "error", err)
	}

	// 删除日志记录
	result, err := gorm.G[models.ChatLog](models.DB).
		Where("id IN ?", req.IDs).
		Delete(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to delete logs: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result,
	})
}

// HealthCheckSettingsResponse 健康检测设置响应结构
type HealthCheckSettingsResponse struct {
	Enabled                 bool `json:"enabled"`
	Interval                int  `json:"interval"`
	FailureThreshold        int  `json:"failure_threshold"`
	FailureDisableEnabled   bool `json:"failure_disable_enabled"`
	AutoEnable              bool `json:"auto_enable"`
	LogRetentionCount       int  `json:"log_retention_count"`
	CountHealthCheckSuccess bool `json:"count_health_check_as_success"`
	CountHealthCheckFailure bool `json:"count_health_check_as_failure"`
}

// UpdateHealthCheckSettingsRequest 更新健康检测设置请求结构
type UpdateHealthCheckSettingsRequest struct {
	Enabled                 bool `json:"enabled"`
	Interval                int  `json:"interval"`
	FailureThreshold        int  `json:"failure_threshold"`
	FailureDisableEnabled   bool `json:"failure_disable_enabled"`
	AutoEnable              bool `json:"auto_enable"`
	LogRetentionCount       int  `json:"log_retention_count"`
	CountHealthCheckSuccess bool `json:"count_health_check_as_success"`
	CountHealthCheckFailure bool `json:"count_health_check_as_failure"`
}

// GetHealthCheckSettings 获取健康检测设置
func GetHealthCheckSettings(c *gin.Context) {
	ctx := c.Request.Context()
	enabled, interval, failureThreshold, failureDisableEnabled, autoEnable, logRetentionCount, countAsSuccess, countAsFailure := service.GetHealthCheckSettings(ctx)

	response := HealthCheckSettingsResponse{
		Enabled:                 enabled,
		Interval:                interval,
		FailureThreshold:        failureThreshold,
		FailureDisableEnabled:   failureDisableEnabled,
		AutoEnable:              autoEnable,
		LogRetentionCount:       logRetentionCount,
		CountHealthCheckSuccess: countAsSuccess,
		CountHealthCheckFailure: countAsFailure,
	}

	common.Success(c, response)
}

// UpdateHealthCheckSettings 更新健康检测设置
func UpdateHealthCheckSettings(c *gin.Context) {
	var req UpdateHealthCheckSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// 更新启用状态
	enabledValue := "false"
	if req.Enabled {
		enabledValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckEnabled).
		Update(ctx, "value", enabledValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新检测间隔
	if req.Interval < 1 {
		req.Interval = 60
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckInterval).
		Update(ctx, "value", strconv.Itoa(req.Interval)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新失败次数阈值
	if req.FailureThreshold < 1 {
		req.FailureThreshold = 3
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckFailureThreshold).
		Update(ctx, "value", strconv.Itoa(req.FailureThreshold)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新失败自动禁用开关
	failureDisableEnabledValue := "false"
	if req.FailureDisableEnabled {
		failureDisableEnabledValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckFailureDisableEnabled).
		Update(ctx, "value", failureDisableEnabledValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动启用
	autoEnableValue := "false"
	if req.AutoEnable {
		autoEnableValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckAutoEnable).
		Update(ctx, "value", autoEnableValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新健康检测日志保留条数
	if req.LogRetentionCount < 0 {
		req.LogRetentionCount = 0
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckLogRetentionCount).
		Update(ctx, "value", strconv.Itoa(req.LogRetentionCount)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	countHealthCheckSuccess := "false"
	if req.CountHealthCheckSuccess {
		countHealthCheckSuccess = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsSuccess).
		Update(ctx, "value", countHealthCheckSuccess); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	countHealthCheckFailure := "false"
	if req.CountHealthCheckFailure {
		countHealthCheckFailure = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsFailure).
		Update(ctx, "value", countHealthCheckFailure); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 重启健康检测服务
	go service.GetHealthChecker().Restart(context.Background())

	// 执行日志清理以满足新的保留策略
	go service.EnforceHealthCheckLogRetention(context.Background())

	// 返回更新后的设置
	GetHealthCheckSettings(c)
}

// GetHealthCheckLogs 获取健康检测日志（支持分页和筛选）
func GetHealthCheckLogs(c *gin.Context) {
	// 分页参数
	pageStr := c.Query("page")
	page := 1
	if pageStr != "" {
		parsedPage, err := strconv.Atoi(pageStr)
		if err != nil || parsedPage < 1 {
			common.BadRequest(c, "Invalid page parameter")
			return
		}
		page = parsedPage
	}

	pageSizeStr := c.Query("page_size")
	pageSize := 20 // Default page size
	if pageSizeStr != "" {
		parsedPageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || parsedPageSize < 1 || parsedPageSize > 100 {
			common.BadRequest(c, "Invalid page_size parameter (must be between 1 and 100)")
			return
		}
		pageSize = parsedPageSize
	}

	// 筛选参数
	modelProviderID := c.Query("model_provider_id")
	modelName := c.Query("model_name")
	providerName := c.Query("provider_name")
	status := c.Query("status")

	// 构建查询条件
	query := models.DB.Model(&models.HealthCheckLog{})

	if modelProviderID != "" {
		query = query.Where("model_provider_id = ?", modelProviderID)
	}

	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}

	if providerName != "" {
		query = query.Where("provider_name = ?", providerName)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.InternalServerError(c, "Failed to count health check logs: "+err.Error())
		return
	}

	// 获取分页数据
	var logs []models.HealthCheckLog
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		common.InternalServerError(c, "Failed to query health check logs: "+err.Error())
		return
	}

	result := map[string]any{
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (total + int64(pageSize) - 1) / int64(pageSize),
	}

	common.Success(c, result)
}

// ClearHealthCheckLogs 清空健康检测日志
func ClearHealthCheckLogs(c *gin.Context) {
	ctx := c.Request.Context()

	result, err := gorm.G[models.HealthCheckLog](models.DB).
		Where("1 = 1").
		Delete(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to clear health check logs: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result,
	})
}

// RunHealthCheck 手动运行单个模型提供商的健康检测
func RunHealthCheck(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	ctx := c.Request.Context()

	log, err := service.GetHealthChecker().CheckSingle(ctx, uint(id))
	if err != nil {
		common.InternalServerError(c, "Failed to run health check: "+err.Error())
		return
	}

	common.Success(c, log)
}

// RunHealthCheckAll 手动运行所有模型提供商的健康检测
func RunHealthCheckAll(c *gin.Context) {
	go func() {
		checker := service.GetHealthChecker()
		ctx := context.Background()

		// 获取所有模型提供商关联
		modelProviders, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
		if err != nil {
			slog.Error("failed to get model providers for health check", "error", err)
			return
		}

		for _, mp := range modelProviders {
			checker.CheckSingle(ctx, mp.ID)
		}
	}()

	common.Success(c, map[string]string{
		"message": "Health check started for all model providers",
	})
}

// ClearAllLogs 清空所有日志
func ClearAllLogs(c *gin.Context) {
	ctx := c.Request.Context()

	// 删除所有ChatIO记录
	if _, err := gorm.G[models.ChatIO](models.DB).
		Where("1 = 1").
		Delete(ctx); err != nil {
		slog.Warn("failed to delete all chat io records", "error", err)
	}

	// 删除所有日志记录
	result, err := gorm.G[models.ChatLog](models.DB).
		Where("1 = 1").
		Delete(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to clear logs: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result,
	})
}
