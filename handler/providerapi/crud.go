package providerapi

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/autoassoc"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/handler/settings"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
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
		httpresp.InternalServerError(c, err.Error())
		return
	}

	httpresp.Success(c, list)
}

// CreateProvider 创建提供商。
func CreateProvider(c *gin.Context) {
	var req ProviderRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()
	exists, err := repos().Provider.ExistsByName(ctx, req.Name)
	if err != nil {
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if exists {
		httpresp.BadRequest(c, "Provider already exists")
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

	// AuthType 仅 Anthropic
	authType := resolveAuthType(req.Type, req.AuthType)

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
		httpresp.InternalServerError(c, "Failed to create provider: "+err.Error())
		return
	}

	go autoassoc.TriggerAutoAssociate(context.Background())

	httpresp.Success(c, provider)
}

// UpdateProvider 更新提供商。
func UpdateProvider(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}

	var req ProviderRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()
	if _, err := repos().Provider.Get(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Provider not found")
			return
		}
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	authType := resolveAuthType(req.Type, req.AuthType)

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

	if err := repos().Provider.Update(ctx, id, &updates); err != nil {
		httpresp.InternalServerError(c, "Failed to update provider: "+err.Error())
		return
	}

	updatedProvider, err := repos().Provider.Get(ctx, id)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to retrieve updated provider: "+err.Error())
		return
	}

	go autoassoc.TriggerAutoAssociate(context.Background())
	go autoassoc.TriggerAutoClean(context.Background())

	httpresp.Success(c, updatedProvider)
}

// DeleteProvider 删除提供商。
func DeleteProvider(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	result, err := repos().Provider.Delete(ctx, id)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to delete provider: "+err.Error())
		return
	}

	autoClean := settings.GetSettingBool(ctx, models.SettingKeyAutoCleanOnDelete)
	if autoClean {
		go autoassoc.TriggerAutoClean(context.Background())
	} else {
		if _, err := repos().ModelWithProvider.DeleteByProviderID(ctx, id); err != nil {
			httpresp.InternalServerError(c, "Failed to delete provider: "+err.Error())
			return
		}
	}

	if result == 0 {
		httpresp.NotFound(c, "Provider not found")
		return
	}

	httpresp.Success(c, nil)
}
