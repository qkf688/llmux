package providerapi

import (
	"context"
	"errors"
	"strconv"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/handler/autoassoc"
	"github.com/atopos31/llmio/handler/settings"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetProviders 获取所有提供商列表（支持名称搜索和类型筛选）。
func GetProviders(c *gin.Context) {
	name := c.Query("name")
	providerType := c.Query("type")

	list, err := repos().Provider.List(c.Request.Context(), repository.ProviderFilter{
		Name: name,
		Type: providerType,
	})
	if err != nil {
		common.InternalServerError(c, err.Error())
		return
	}

	common.Success(c, list)
}

// CreateProvider 创建提供商。
func CreateProvider(c *gin.Context) {
	var req ProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()
	exists, err := repos().Provider.ExistsByName(ctx, req.Name)
	if err != nil {
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if exists {
		common.BadRequest(c, "Provider already exists")
		return
	}

	modelEndpoint := true
	if req.ModelEndpoint != nil {
		modelEndpoint = *req.ModelEndpoint
	}

	modelFilterEnabled := false
	if req.ModelFilterEnabled != nil {
		modelFilterEnabled = *req.ModelFilterEnabled
	}

	// AuthType 仅 Anthropic；DRY-16 抽取留给阶段 5
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

	if err := repos().Provider.Create(ctx, &provider); err != nil {
		common.InternalServerError(c, "Failed to create provider: "+err.Error())
		return
	}

	go autoassoc.TriggerAutoAssociate(context.Background())

	common.Success(c, provider)
}

// UpdateProvider 更新提供商。
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

	ctx := c.Request.Context()
	if _, err := repos().Provider.Get(ctx, uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.NotFound(c, "Provider not found")
			return
		}
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}

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

	if err := repos().Provider.Update(ctx, uint(id), &updates); err != nil {
		common.InternalServerError(c, "Failed to update provider: "+err.Error())
		return
	}

	updatedProvider, err := repos().Provider.Get(ctx, uint(id))
	if err != nil {
		common.InternalServerError(c, "Failed to retrieve updated provider: "+err.Error())
		return
	}

	go autoassoc.TriggerAutoAssociate(context.Background())
	go autoassoc.TriggerAutoClean(context.Background())

	common.Success(c, updatedProvider)
}

// DeleteProvider 删除提供商。
func DeleteProvider(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	ctx := c.Request.Context()
	result, err := repos().Provider.Delete(ctx, uint(id))
	if err != nil {
		common.InternalServerError(c, "Failed to delete provider: "+err.Error())
		return
	}

	autoClean := settings.GetSettingBool(ctx, models.SettingKeyAutoCleanOnDelete)
	if autoClean {
		go autoassoc.TriggerAutoClean(context.Background())
	} else {
		if _, err := repos().ModelWithProvider.DeleteByProviderID(ctx, uint(id)); err != nil {
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