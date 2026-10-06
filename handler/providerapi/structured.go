package providerapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/repository"
)

// errValidation 标记「请求数据/配置不合法」这类非 DB 故障（客户端可修复、重发即可）
// 的错误哨兵。事务内 syncProviderChildren 的业务校验错用 %w 包装它，RunInTx 返回后
// handler 凭 errors.Is 区分 BadRequest（400） 与 InternalServerError（500），
// 避免把「加密未配置 / 引用失效」这类语义错误误报为服务器故障。
var errValidation = errors.New("provider input validation")

// hasChildren 判断请求体是否携带结构化 children（endpoints/groups 任一非 nil）。
// true = 全量更新意图；false = partial-update（两键均缺失），只改顶层标量、不动子表。
// 任一键单独出现（半 partial）同样判 true，交由 validateStructuredRequest 显式拒绝
// 缺失的另一半——禁止静默丢弃已提供的键。
func (req *ProviderRequest) hasChildren() bool {
	return req.Endpoints != nil || req.Groups != nil
}

// validateStructuredRequest 校验 S6 结构化字段；失败返回可直接给 BadRequest 的文案。
// 仅全量路径（hasChildren）调用；partial 请求（开关类）跳过本校验。
func validateStructuredRequest(ctx context.Context, repos *repository.Repositories, req *ProviderRequest) error {
	if len(req.Protocols) == 0 {
		return fmt.Errorf("至少勾选一个出站协议")
	}
	protoSet := make(map[string]struct{}, len(req.Protocols))
	for _, p := range req.Protocols {
		if _, ok := consts.TypeOfProtocol(consts.Protocol(p)); !ok {
			return fmt.Errorf("invalid protocol %q", p)
		}
		if _, dup := protoSet[p]; dup {
			return fmt.Errorf("duplicate protocol %q in protocols", p)
		}
		protoSet[p] = struct{}{}
	}

	// 全量路径由调用方保证 Endpoints/Groups 非 nil（hasChildren），此处防御性解引用。
	endpoints := []EndpointInput{}
	if req.Endpoints != nil {
		endpoints = *req.Endpoints
	}
	if len(endpoints) == 0 {
		return fmt.Errorf("至少保留一个协议端点")
	}
	epSeen := make(map[string]struct{}, len(endpoints))
	for _, ep := range endpoints {
		if _, ok := consts.TypeOfProtocol(consts.Protocol(ep.Protocol)); !ok {
			return fmt.Errorf("invalid endpoint protocol %q", ep.Protocol)
		}
		if _, dup := epSeen[ep.Protocol]; dup {
			return fmt.Errorf("duplicate endpoint protocol %q (one per protocol)", ep.Protocol)
		}
		epSeen[ep.Protocol] = struct{}{}
		if _, in := protoSet[ep.Protocol]; !in {
			return fmt.Errorf("endpoint protocol %q not in protocols", ep.Protocol)
		}
	}
	for p := range protoSet {
		if _, ok := epSeen[p]; !ok {
			return fmt.Errorf("protocol %q missing endpoint row", p)
		}
	}

	groups := []GroupInput{}
	if req.Groups != nil {
		groups = *req.Groups
	}
	if len(groups) == 0 {
		return fmt.Errorf("至少保留一个凭据分组")
	}
	for i, g := range groups {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			return fmt.Errorf("groups[%d].name is required", i)
		}
		if g.Weight < 0 {
			return fmt.Errorf("groups[%d].weight must be >= 0", i)
		}
		if g.PoolID == nil || *g.PoolID == 0 {
			return fmt.Errorf("groups[%d].pool_id is required", i)
		}
		if _, err := repos.Pool.Get(ctx, *g.PoolID); err != nil {
			return fmt.Errorf("groups[%d].pool_id %d not found", i, *g.PoolID)
		}
	}
	return nil
}

// syncProviderChildren 在事务内按请求体全量同步 endpoints / groups。
// provider 行本身由调用方已 Create/Update；本函数只写子表。仅全量路径调用（调用方已解引用）。
func syncProviderChildren(ctx context.Context, repos *repository.Repositories, providerID uint, endpoints []EndpointInput, groups []GroupInput) error {
	if err := syncEndpoints(ctx, repos, providerID, endpoints); err != nil {
		return err
	}
	return syncGroups(ctx, repos, providerID, groups)
}

func syncEndpoints(ctx context.Context, repos *repository.Repositories, providerID uint, inputs []EndpointInput) error {
	existing, err := repos.Endpoint.ListByProvider(ctx, providerID)
	if err != nil {
		return fmt.Errorf("list endpoints: %w", err)
	}
	byProto := make(map[string]models.Endpoint, len(existing))
	for _, e := range existing {
		byProto[e.Protocol] = e
	}

	keep := make(map[string]struct{}, len(inputs))
	for _, in := range inputs {
		keep[in.Protocol] = struct{}{}
		if old, ok := byProto[in.Protocol]; ok {
			if _, err := repos.Endpoint.UpdateFields(ctx, old.ID, map[string]any{
				"url":     in.URL,
				"enabled": in.Enabled,
			}); err != nil {
				return fmt.Errorf("update endpoint %q: %w", in.Protocol, err)
			}
			continue
		}
		ep := &models.Endpoint{
			ProviderID: providerID,
			Protocol:   in.Protocol,
			URL:        in.URL,
			Enabled:    in.Enabled,
		}
		if err := repos.Endpoint.Create(ctx, ep); err != nil {
			return fmt.Errorf("create endpoint %q: %w", in.Protocol, err)
		}
		// Endpoint.Enabled 带 gorm:"default:true"：GORM Create(struct) 会跳过零值
		// false，走 DB default 落成 true——必须用 UpdateFields 显式写 false（map 绕零值）。
		if !in.Enabled {
			if _, err := repos.Endpoint.UpdateFields(ctx, ep.ID, map[string]any{"enabled": false}); err != nil {
				return fmt.Errorf("persist endpoint disabled %q: %w", in.Protocol, err)
			}
		}
	}

	for proto, old := range byProto {
		if _, ok := keep[proto]; ok {
			continue
		}
		if _, err := repos.Endpoint.Delete(ctx, old.ID); err != nil {
			return fmt.Errorf("delete endpoint %q: %w", proto, err)
		}
	}
	return nil
}

// syncGroups 在事务内同步凭据分组。
//
// 分组是**就地更新**而非删光重建：组 ID 稳定（管理端弹窗定位、请求上下文里的钉定偏好
// 都不因保存而失效），且 KeyGroup.Models（模型同步专属数据）不在写入体里——重建会把
// 同步白名单冲成空（空 = 不限）并让组织级目录缩水、连累自动关联清理误删关联
// （白名单唯一写入方 = modelsync）。
// 组 id=0/缺失 = 新建（白名单空 = 未同步/不限）；既有 id 必须属于本 provider 且
// 不得重复（防跨供应商引用与同组双写）。凭据本身不由本函数触碰——统一由号池拥有。
func syncGroups(ctx context.Context, repos *repository.Repositories, providerID uint, inputs []GroupInput) error {
	existing, err := repos.KeyGroup.ListByProvider(ctx, providerID)
	if err != nil {
		return fmt.Errorf("list groups: %w", err)
	}
	existingByID := make(map[uint]models.KeyGroup, len(existing))
	for _, g := range existing {
		existingByID[g.ID] = g
	}

	// 先校验全部组 id 引用（未知/重复 → 400），再落任何写——半程失败不得留下部分更新。
	seenGroup := make(map[uint]struct{}, len(inputs))
	for i, in := range inputs {
		if in.ID == 0 {
			continue
		}
		if _, dup := seenGroup[in.ID]; dup {
			return fmt.Errorf("groups[%d] duplicates group id %d: %w", i, in.ID, errValidation)
		}
		seenGroup[in.ID] = struct{}{}
		if _, ok := existingByID[in.ID]; !ok {
			return fmt.Errorf("groups[%d] references group %d not found: %w", i, in.ID, errValidation)
		}
	}

	keptGroup := make(map[uint]struct{}, len(inputs))
	for i, in := range inputs {
		group := &models.KeyGroup{
			ProviderID: providerID,
			Name:       strings.TrimSpace(in.Name),
			Weight:     in.Weight,
			PoolID:     in.PoolID,
		}
		if in.ID != 0 {
			// 就地更新：只写表单拥有的字段（name/weight/pool），Models 永不触碰。
			// map 更新绕开 GORM 零值跳过：weight=0 / pool_id=nil 都是显式意图。
			fields := map[string]any{
				"name":    group.Name,
				"weight":  in.Weight,
				"pool_id": *in.PoolID,
			}
			if _, err := repos.KeyGroup.UpdateFields(ctx, in.ID, fields); err != nil {
				return fmt.Errorf("update group[%d] %d: %w", i, in.ID, err)
			}
			group.ID = in.ID
			group.Models = existingByID[in.ID].Models
		} else {
			if err := repos.KeyGroup.Create(ctx, group); err != nil {
				return fmt.Errorf("create group[%d]: %w", i, err)
			}
			// KeyGroup.Weight 带 gorm:"default:1"：GORM Create(struct) 跳过零值 0，
			// 走 DB default 落成 1——用 UpdateFields 显式写 weight（map 绕零值）。
			if in.Weight == 0 {
				if _, err := repos.KeyGroup.UpdateFields(ctx, group.ID, map[string]any{"weight": 0}); err != nil {
					return fmt.Errorf("persist group[%d] weight 0: %w", i, err)
				}
			}
		}
		keptGroup[group.ID] = struct{}{}
	}

	// 未提交的既有组删除。凭据归号池所有，分组删除不动凭据（池仍在，别处可能还引用）。
	for id := range existingByID {
		if _, ok := keptGroup[id]; ok {
			continue
		}
		if _, err := repos.KeyGroup.Delete(ctx, id); err != nil {
			return fmt.Errorf("delete group %d: %w", id, err)
		}
	}
	return nil
}

// loadProviderDetail 组装详情响应（分组只出 KeyGroup 本身；凭据经 PoolID 去号池页查看）。
func loadProviderDetail(ctx context.Context, repos *repository.Repositories, provider models.Provider) (*ProviderDetail, error) {
	// 顶层 AuthType 出库即派生（Stage 20）：响应直返实体，派生字段必须先于序列化填充。
	providers.FillDerivedAuthType(&provider)
	endpoints, err := repos.Endpoint.ListByProvider(ctx, provider.ID)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}
	groups, err := repos.KeyGroup.ListByProvider(ctx, provider.ID)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	details := make([]GroupDetail, 0, len(groups))
	for _, g := range groups {
		details = append(details, GroupDetail{KeyGroup: g})
	}

	if endpoints == nil {
		endpoints = []models.Endpoint{}
	}
	return &ProviderDetail{
		Provider:  provider,
		Endpoints: endpoints,
		Groups:    details,
	}, nil
}

// buildProviderListItems 批量附带端点/分组计数。
func buildProviderListItems(ctx context.Context, repos *repository.Repositories, list []models.Provider) ([]ProviderListItem, error) {
	ids := make([]uint, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.ID)
	}
	epCounts, err := repos.Endpoint.CountByProviderIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("count endpoints: %w", err)
	}
	groupCounts, err := repos.KeyGroup.CountByProviderIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("count groups: %w", err)
	}

	items := make([]ProviderListItem, 0, len(list))
	for _, p := range list {
		providers.FillDerivedAuthType(&p)
		items = append(items, ProviderListItem{
			Provider:      p,
			EndpointCount: epCounts[p.ID],
			GroupCount:    groupCounts[p.ID],
		})
	}
	return items, nil
}
