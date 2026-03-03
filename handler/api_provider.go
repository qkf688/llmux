package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
			"version": "2023-06-01",
			"auth_type": "x-api-key"
		}`,
	},
}

// ProviderRequest represents the request body for creating/updating a provider
type ProviderRequest struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	Config             string `json:"config"`
	Console            string `json:"console"`
	Proxy              string `json:"proxy"`
	ModelEndpoint      *bool  `json:"model_endpoint"`
	ModelFilterEnabled *bool  `json:"model_filter_enabled"`
	Blacklisted        *bool  `json:"blacklisted"` // 是否拉黑（拉黑后跳过自动关联/一键关联）
	AuthType           string `json:"auth_type"`   // 认证方式：x-api-key 或 bearer，仅用于 Anthropic 类型
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

	if provider.ModelEndpoint != nil && !*provider.ModelEndpoint {
		common.BadRequest(c, "model_endpoint disabled")
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

	// 如果没有指定ModelEndpoint，默认为true
	modelEndpoint := true
	if req.ModelEndpoint != nil {
		modelEndpoint = *req.ModelEndpoint
	}

	// 如果没有指定ModelFilterEnabled，默认为false
	modelFilterEnabled := false
	if req.ModelFilterEnabled != nil {
		modelFilterEnabled = *req.ModelFilterEnabled
	}

	// 处理 AuthType（仅用于 Anthropic 类型）
	var authType *string
	if req.Type == "anthropic" && req.AuthType != "" {
		authType = &req.AuthType
	}

	provider := models.Provider{
		Name:               req.Name,
		Type:               req.Type,
		Config:             req.Config,
		Console:            req.Console,
		Proxy:              req.Proxy,
		ModelEndpoint:      &modelEndpoint,
		ModelFilterEnabled: &modelFilterEnabled,
		AuthType:           authType,
	}

	if err := gorm.G[models.Provider](models.DB).Create(c.Request.Context(), &provider); err != nil {
		common.InternalServerError(c, "Failed to create provider: "+err.Error())
		return
	}

	// 触发自动关联
	go triggerAutoAssociate(context.Background())

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
	var authType *string
	if req.Type == "anthropic" && req.AuthType != "" {
		authType = &req.AuthType
	}

	updates := models.Provider{
		Name:               req.Name,
		Type:               req.Type,
		Config:             req.Config,
		Console:            req.Console,
		Proxy:              req.Proxy,
		ModelEndpoint:      req.ModelEndpoint,
		ModelFilterEnabled: req.ModelFilterEnabled,
		Blacklisted:        req.Blacklisted,
		AuthType:           authType,
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

	// 触发自动关联和清理
	go triggerAutoAssociate(context.Background())
	go triggerAutoClean(context.Background())

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

	// 检查是否启用自动清理
	ctx := c.Request.Context()
	autoClean := GetSettingBool(ctx, models.SettingKeyAutoCleanOnDelete)
	if autoClean {
		go triggerAutoClean(context.Background())
	} else {
		// 手动删除关联
		if _, err := gorm.G[models.ModelWithProvider](models.DB).Where("provider_id = ?", id).Delete(ctx); err != nil {
			common.InternalServerError(c, "Failed to delete provider: "+err.Error())
			return
		}
	}

	if result == 0 {
		common.NotFound(c, "Provider not found")
		return
	}

	common.Success(c, nil)
}

// GetProviderTemplates 获取提供商模板
func GetProviderTemplates(c *gin.Context) {
	common.Success(c, template)
}

// GetProviderBlacklist 获取拉黑的供应商ID列表
func GetProviderBlacklist(c *gin.Context) {
	ctx := c.Request.Context()
	var providers []models.Provider
	if err := models.DB.WithContext(ctx).Where("blacklisted = ?", true).Select("id").Find(&providers).Error; err != nil {
		common.InternalServerError(c, "Failed to get blacklist: "+err.Error())
		return
	}
	ids := make([]uint, 0, len(providers))
	for _, p := range providers {
		ids = append(ids, p.ID)
	}
	common.Success(c, map[string]interface{}{"blacklisted_ids": ids})
}

// UpdateProviderBlacklist 更新供应商黑名单（整体替换）
func UpdateProviderBlacklist(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		ProviderIDs []uint `json:"provider_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	var rowsAffected int64
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		falseVal := false
		if err := tx.Model(&models.Provider{}).Where("blacklisted = ?", true).Update("blacklisted", &falseVal).Error; err != nil {
			return err
		}
		if len(req.ProviderIDs) > 0 {
			trueVal := true
			result := tx.Model(&models.Provider{}).Where("id IN ?", req.ProviderIDs).Update("blacklisted", &trueVal)
			if result.Error != nil {
				return result.Error
			}
			rowsAffected = result.RowsAffected
		}
		return nil
	})
	if err != nil {
		common.InternalServerError(c, "Failed to update blacklist: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{"updated": rowsAffected})
}

// ClearProviderAssociations 清除指定提供商的所有模型关联
func ClearProviderAssociations(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid provider ID format")
		return
	}

	ctx := c.Request.Context()

	// 首先检查提供商是否存在
	provider, err := gorm.G[models.Provider](models.DB).Where("id = ?", id).First(ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.NotFound(c, "Provider not found")
		} else {
			common.InternalServerError(c, "Failed to get provider: "+err.Error())
		}
		return
	}

	// 删除该提供商的所有模型关联
	result, err := gorm.G[models.ModelWithProvider](models.DB).
		Where("provider_id = ?", id).
		Delete(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to clear associations: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"provider_id":   id,
		"provider_name": provider.Name,
		"deleted_count": result,
	})
}
