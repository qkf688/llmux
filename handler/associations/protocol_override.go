package associations

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/channel"
	"gorm.io/gorm"
)

// protocolOverrideError 表示**客户端可修复**的覆盖配置错误（400）。与读库失败分开：
// 后者是基础设施故障，走 InternalErrorMessage（500 收口），原始 err 只进日志。
// 把两者混成 400 + err.Error() 会把表名/锁信息回给浏览器，违反 AGENTS §5 的 500 收口。
type protocolOverrideError struct{ msg string }

func (e *protocolOverrideError) Error() string { return e.msg }

func invalidProtocolOverride(format string, args ...any) error {
	return &protocolOverrideError{msg: fmt.Sprintf(format, args...)}
}

// writeProtocolOverrideError 把覆盖校验错误映射到响应：校验类 400（回本包自己的静态文案），
// 其余 500 收口。调用方禁止各自 BadRequest —— 那会重新把 DB 错误当客户端错误。
func writeProtocolOverrideError(c *gin.Context, err error) {
	var ve *protocolOverrideError
	if errors.As(err, &ve) {
		httpresp.BadRequest(c, ve.msg)
		return
	}
	httpresp.InternalErrorMessage(c, "Failed to validate protocol override", err)
}

// validateProtocolOverride 校验并归一化关联的协议覆盖（两项三态字段）。
//
// 判据与运行时**同一份**（channel.ResolveProtocolScope）：保存时能存进去的组合，
// 运行时必须解析得出来；解析会失败的组合在保存时就拒掉，避免「配置看着正常、
// 请求恒 ErrEndpointUnavailable」。规则：
//   - 未知协议 → 拒绝（与 consts.protocolMetaTable 对齐，不静默忽略）；
//   - 重复协议去重；空切片归一为 nil（= 继承）——「一条协议都不许走」没有业务含义，
//     要停用整个关联请用 Status；
//   - 主格式必须落在生效集合内：集合已设置时比集合，未设置时比「供应商当前已启用的协议」；
//   - 没有供应商（provider_id=0 的悬空关联）时不允许单独设主格式——无法校验落点。
func validateProtocolOverride(
	ctx context.Context,
	repos *repository.Repositories,
	providerID uint,
	protocols *[]string,
	primary *string,
) (*[]string, *string, error) {
	if protocols != nil {
		clean := make([]string, 0, len(*protocols))
		seen := make(map[consts.Protocol]bool, len(*protocols))
		for _, raw := range *protocols {
			p := consts.Protocol(raw)
			if _, ok := consts.TypeOfProtocol(p); !ok {
				return nil, nil, invalidProtocolOverride("unknown protocol %q", raw)
			}
			if seen[p] {
				continue
			}
			seen[p] = true
			clean = append(clean, raw)
		}
		if len(clean) == 0 {
			protocols = nil
		} else {
			protocols = &clean
		}
	}

	if primary == nil {
		return protocols, nil, nil
	}

	p := consts.Protocol(*primary)
	if _, ok := consts.TypeOfProtocol(p); !ok {
		return nil, nil, invalidProtocolOverride("unknown primary_protocol %q", *primary)
	}
	if protocols != nil {
		if !slices.Contains(*protocols, *primary) {
			return nil, nil, invalidProtocolOverride("primary_protocol %q is not in protocols", *primary)
		}
		return protocols, primary, nil
	}

	// 集合继承供应商 → 主格式必须落在供应商已启用的协议里，否则转换没有落点。
	if providerID == 0 {
		return nil, nil, invalidProtocolOverride("provider_id is required when primary_protocol is set")
	}
	provider, err := repos.Provider.Get(ctx, providerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, invalidProtocolOverride("provider %d not found", providerID)
		}
		return nil, nil, fmt.Errorf("load provider %d: %w", providerID, err)
	}
	endpoints, err := repos.Endpoint.ListByProvider(ctx, providerID)
	if err != nil {
		return nil, nil, fmt.Errorf("load endpoints of provider %d: %w", providerID, err)
	}
	if _, err := channel.ResolveProtocolScope(provider.Type, endpoints,
		&models.ModelWithProvider{PrimaryProtocol: primary}); err != nil {
		// 解析错误文本只进日志：对外文案是给管理员的动作指引，不是解析器内部措辞。
		slog.Debug("protocol override rejected", "provider_id", providerID, "primary_protocol", *primary, "error", err)
		return nil, nil, invalidProtocolOverride("primary_protocol %q is not enabled on this provider", *primary)
	}
	return protocols, primary, nil
}

// UpdateModelProviderProtocols 只写协议覆盖两列（`PATCH /model-providers/:id/protocols`）。
//
// 为什么不能复用全量 PUT：关联 Update 的契约是「五项三态字段缺省 = 显式清回继承」+
// `WithHeader` 二态无条件写指针，只为改协议而发一次 PUT 会把能力、thinking、请求头透传
// 一起改掉。协议检测的「一键设为只走 X」正是这种调用方——窄端点只碰协议两列，
// 从结构上消除这一类误写，而不是要求每个新调用方记得把整行字段补全。
// 两项都按「显式提交」处理：nil = 写 NULL（继承），与 PUT 的三态契约一致。
func UpdateModelProviderProtocols(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}
	var req ModelWithProviderProtocolsRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	ctx := c.Request.Context()
	existing, err := repos().ModelWithProvider.Get(ctx, uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Model-provider association not found")
			return
		}
		httpresp.InternalErrorMessage(c, "Database error", err)
		return
	}

	protocols, primaryProtocol, err := validateProtocolOverride(ctx, repos(), existing.ProviderID, req.Protocols, req.PrimaryProtocol)
	if err != nil {
		writeProtocolOverrideError(c, err)
		return
	}
	protocolsVal, err := models.SerializeStringSlicePtrForUpdate(protocols)
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to serialize protocols", err)
		return
	}
	if _, err := repos().ModelWithProvider.UpdateFields(ctx, uint(id), map[string]any{
		"protocols":        protocolsVal,
		"primary_protocol": primaryProtocol,
	}); err != nil {
		httpresp.InternalErrorMessage(c, "Failed to update protocol override", err)
		return
	}

	updated, err := repos().ModelWithProvider.Get(ctx, uint(id))
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to retrieve updated model-provider association", err)
		return
	}
	httpresp.Success(c, updated)
}
