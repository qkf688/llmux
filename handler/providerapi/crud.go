package providerapi

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/handler/autoassoc"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/handler/settings"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// respondTxError 统一处理 RunInTx 返回的错误：命中 errValidation 哨兵（请求数据/配置
// 不合法）回 400，其余（DB 故障等）回 500。区分校验错与服务器错，避免把配置缺失/引用
// 失效误报成 500。
func respondTxError(c *gin.Context, action string, err error) {
	if errors.Is(err, errValidation) {
		httpresp.BadRequest(c, err.Error())
		return
	}
	httpresp.InternalServerError(c, action+": "+err.Error())
}

// GetProviders 获取所有提供商列表（支持名称搜索和类型筛选），附端点/分组计数。
func GetProviders(c *gin.Context) {
	name := c.Query("name")
	providerType := c.Query("type")
	ctx := c.Request.Context()

	list, err := repos().Provider.List(ctx, repository.ProviderFilter{
		Name: name,
		Type: providerType,
	})
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	items, err := buildProviderListItems(ctx, repos(), list)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	httpresp.Success(c, items)
}

// GetProvider 获取单个提供商详情（展开 endpoints / groups）。
func GetProvider(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	provider, err := repos().Provider.Get(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Provider not found")
			return
		}
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	detail, err := loadProviderDetail(ctx, repos(), *provider)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}
	httpresp.Success(c, detail)
}

// CreateProvider 创建提供商（事务写 providers + endpoints + key_groups + credentials）。
func CreateProvider(c *gin.Context) {
	var req ProviderRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()
	if err := validateStructuredRequest(ctx, repos(), &req); err != nil {
		httpresp.BadRequest(c, err.Error())
		return
	}

	sanitized, err := sanitizeConfig(req.Config)
	if err != nil {
		httpresp.BadRequest(c, "Invalid config JSON: "+err.Error())
		return
	}
	req.Config = sanitized

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

	authType := resolveAuthType(req.Type, req.AuthType)

	var createdID uint
	err = repos().RunInTx(ctx, func(txRepos *repository.Repositories) error {
		provider := models.Provider{
			Name:               req.Name,
			Type:               req.Type,
			Config:             req.Config,
			Console:            req.Console,
			Proxy:              req.Proxy,
			ModelEndpoint:      &modelEndpoint,
			ModelFilterEnabled: &modelFilterEnabled,
			AuthType:           authType,
			Protocols:          req.Protocols,
		}
		if err := txRepos.Provider.Create(ctx, &provider); err != nil {
			return err
		}
		createdID = provider.ID
		endpoints := []EndpointInput{}
		if req.Endpoints != nil {
			endpoints = *req.Endpoints
		}
		groups := []GroupInput{}
		if req.Groups != nil {
			groups = *req.Groups
		}
		return syncProviderChildren(ctx, txRepos, provider.ID, endpoints, groups)
	})
	if err != nil {
		respondTxError(c, "Failed to create provider", err)
		return
	}

	provider, err := repos().Provider.Get(ctx, createdID)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to retrieve created provider: "+err.Error())
		return
	}
	detail, err := loadProviderDetail(ctx, repos(), *provider)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	bgtask.Go(autoassoc.TriggerAutoAssociate)

	httpresp.Success(c, detail)
}

// UpdateProvider 更新提供商（事务同步四表）。
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

	// partial-update：endpoints/groups 键缺失时只改顶层标量（开关类切换复用同一 PUT），
	// 跳过结构化校验与子表重写；表单提交全量携带 children，走完整校验 + 四表同步。
	childrenProvided := req.hasChildren()
	if childrenProvided {
		if err := validateStructuredRequest(ctx, repos(), &req); err != nil {
			httpresp.BadRequest(c, err.Error())
			return
		}
	}

	sanitized, err := sanitizeConfig(req.Config)
	if err != nil {
		httpresp.BadRequest(c, "Invalid config JSON: "+err.Error())
		return
	}
	req.Config = sanitized

	authType := resolveAuthType(req.Type, req.AuthType)

	err = repos().RunInTx(ctx, func(txRepos *repository.Repositories) error {
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
		// 仅全量路径写 Protocols/children；partial 走 struct Updates 零值跳过语义，
		// 保持 Protocols 与子表现有值不变（nil 切片/指针不会被 Updates 覆盖）。
		if childrenProvided {
			endpoints := []EndpointInput{}
			if req.Endpoints != nil {
				endpoints = *req.Endpoints
			}
			groups := []GroupInput{}
			if req.Groups != nil {
				groups = *req.Groups
			}
			updates.Protocols = req.Protocols
			if err := txRepos.Provider.Update(ctx, id, &updates); err != nil {
				return err
			}
			return syncProviderChildren(ctx, txRepos, id, endpoints, groups)
		}
		return txRepos.Provider.Update(ctx, id, &updates)
	})
	if err != nil {
		respondTxError(c, "Failed to update provider", err)
		return
	}

	updatedProvider, err := repos().Provider.Get(ctx, id)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to retrieve updated provider: "+err.Error())
		return
	}
	detail, err := loadProviderDetail(ctx, repos(), *updatedProvider)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	bgtask.Go(autoassoc.TriggerAutoAssociate)
	bgtask.Go(autoassoc.TriggerAutoClean)

	httpresp.Success(c, detail)
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
		bgtask.Go(autoassoc.TriggerAutoClean)
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
