package providerapi

import (
	"encoding/json"
	"errors"
	"fmt"

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

	// partial-update：endpoints/groups 两键均缺失时只改顶层标量（开关类切换复用同一 PUT），
	// 跳过结构化校验与子表重写；表单提交全量携带 children，走完整校验 + 四表同步。
	// 半 partial（只带一键）同样进入全量路径，由 validateStructuredRequest 显式 400 拒绝。
	childrenProvided := req.hasChildren()
	if childrenProvided {
		if err := validateStructuredRequest(ctx, repos(), &req); err != nil {
			httpresp.BadRequest(c, err.Error())
			return
		}
	}

	// 键缺失判定必须先于 sanitize：sanitizeConfig("") 会把空串加工成 "{}"（非零值），
	// 之后 struct Updates 的零值跳过不再保护原值——partial 不带 config 键时会把原
	// Config 覆盖成 "{}" 丢失 base_url 等。空串在此约定为「键缺失 = 不动」。
	configProvided := req.Config != ""
	sanitized, err := sanitizeConfig(req.Config)
	if err != nil {
		httpresp.BadRequest(c, "Invalid config JSON: "+err.Error())
		return
	}
	req.Config = sanitized

	authType := resolveAuthType(req.Type, req.AuthType)

	err = repos().RunInTx(ctx, func(txRepos *repository.Repositories) error {
		if childrenProvided {
			// 全量路径走 UpdateFields(map)：空串/NULL 是合法的「显式清空」语义，
			// struct Updates 的零值跳过会静默保留旧值（console/proxy 清空失效）。
			// protocols 带 serializer:json——map 更新不走 serializer，必须传
			// json.Marshal 后的字符串，直接传 []string 会写坏列（读回解析失败）。
			protoJSON, err := json.Marshal(req.Protocols)
			if err != nil {
				return fmt.Errorf("marshal protocols: %w", err)
			}
			fields := map[string]any{
				"config":    req.Config,
				"console":   req.Console,
				"proxy":     req.Proxy,
				"auth_type": authType, // *string：nil 写 NULL（切回默认认证是合法清空）
				"protocols": string(protoJSON),
			}
			// name/type 是必填身份字段：空串视为客户端缺省，保留现值
			// （与旧 struct 零值跳过行为等价，防畸形请求抹掉身份字段）。
			if req.Name != "" {
				fields["name"] = req.Name
			}
			if req.Type != "" {
				fields["type"] = req.Type
			}
			// 三态开关：指针非 nil 才写，nil = 未提供（保留现值）。
			if req.ModelEndpoint != nil {
				fields["model_endpoint"] = *req.ModelEndpoint
			}
			if req.ModelFilterEnabled != nil {
				fields["model_filter_enabled"] = *req.ModelFilterEnabled
			}
			if req.Blacklisted != nil {
				fields["blacklisted"] = *req.Blacklisted
			}
			if _, err := txRepos.Provider.UpdateFields(ctx, id, fields); err != nil {
				return err
			}
			endpoints := []EndpointInput{}
			if req.Endpoints != nil {
				endpoints = *req.Endpoints
			}
			groups := []GroupInput{}
			if req.Groups != nil {
				groups = *req.Groups
			}
			return syncProviderChildren(ctx, txRepos, id, endpoints, groups)
		}
		// partial：struct Updates 的零值跳过恰是「未提供 = 不动」语义（nil 指针/
		// 空串跳过）；Config 是唯一例外——经 sanitize 加工空串会变 "{}"，须显式守卫。
		updates := models.Provider{
			Name:               req.Name,
			Type:               req.Type,
			Console:            req.Console,
			Proxy:              req.Proxy,
			ModelEndpoint:      req.ModelEndpoint,
			ModelFilterEnabled: req.ModelFilterEnabled,
			Blacklisted:        req.Blacklisted,
			AuthType:           authType,
		}
		if configProvided {
			updates.Config = req.Config
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
