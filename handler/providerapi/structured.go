package providerapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

const (
	groupSourceInline = "inline"
	groupSourcePool   = "pool"
)

// errValidation 标记「请求数据/配置不合法」这类非 DB 故障（客户端可修复、重发即可）
// 的错误哨兵。事务内 syncProviderChildren 的业务校验错用 %w 包装它，RunInTx 返回后
// handler 凭 errors.Is 区分 BadRequest（400） 与 InternalServerError（500），
// 避免把「加密未配置 / 引用失效」这类语义错误误报为服务器故障。
var errValidation = errors.New("provider input validation")

// validateStructuredRequest 校验 S6 结构化字段；失败返回可直接给 BadRequest 的文案。
func validateStructuredRequest(ctx context.Context, repos *repository.Repositories, req *ProviderRequest) error {
	if len(req.Protocols) == 0 {
		return fmt.Errorf("protocols is required (at least one)")
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

	if len(req.Endpoints) == 0 {
		return fmt.Errorf("endpoints is required (at least one)")
	}
	epSeen := make(map[string]struct{}, len(req.Endpoints))
	for _, ep := range req.Endpoints {
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

	if len(req.Groups) == 0 {
		return fmt.Errorf("groups is required (at least one)")
	}
	for i, g := range req.Groups {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			return fmt.Errorf("groups[%d].name is required", i)
		}
		if g.Weight < 0 {
			return fmt.Errorf("groups[%d].weight must be >= 0", i)
		}
		switch g.Source {
		case groupSourceInline:
			keys := normalizeInlineKeys(g.InlineKeys)
			if len(keys) == 0 {
				return fmt.Errorf("groups[%d].inline_keys requires at least one key", i)
			}
		case groupSourcePool:
			if g.PoolID == nil || *g.PoolID == 0 {
				return fmt.Errorf("groups[%d].pool_id is required when source=pool", i)
			}
			if _, err := repos.Pool.Get(ctx, *g.PoolID); err != nil {
				return fmt.Errorf("groups[%d].pool_id %d not found", i, *g.PoolID)
			}
		default:
			return fmt.Errorf("groups[%d].source must be %q or %q", i, groupSourceInline, groupSourcePool)
		}
	}
	return nil
}

func normalizeInlineKeys(keys []string) []string {
	out := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		plain := strings.TrimSpace(k)
		if plain == "" {
			continue
		}
		if _, ok := seen[plain]; ok {
			continue
		}
		seen[plain] = struct{}{}
		out = append(out, plain)
	}
	return out
}

// syncProviderChildren 在事务内按请求体全量同步 endpoints / groups / inline credentials。
// provider 行本身由调用方已 Create/Update；本函数只写子表。
func syncProviderChildren(ctx context.Context, repos *repository.Repositories, providerID uint, req *ProviderRequest) error {
	if err := syncEndpoints(ctx, repos, providerID, req.Endpoints); err != nil {
		return err
	}
	return syncGroups(ctx, repos, providerID, req.Groups)
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

func syncGroups(ctx context.Context, repos *repository.Repositories, providerID uint, inputs []GroupInput) error {
	cipher := credentialcrypto.Default()
	if cipher == nil {
		// inline 组需要加密；若本次全是 pool 仍可能无 cipher——仅 inline 路径检查。
		// 这是配置缺失（BadRequest），非 DB 故障——打哨兵供 handler 判 400。
		for _, g := range inputs {
			if g.Source == groupSourceInline {
				return fmt.Errorf("encryption not configured: %w", errValidation)
			}
		}
	}

	existing, err := repos.KeyGroup.ListByProvider(ctx, providerID)
	if err != nil {
		return fmt.Errorf("list groups: %w", err)
	}
	groupIDs := make([]uint, 0, len(existing))
	for _, g := range existing {
		groupIDs = append(groupIDs, g.ID)
	}

	var oldCreds []models.Credential
	if len(groupIDs) > 0 {
		oldCreds, err = repos.Credential.ListByGroups(ctx, groupIDs, nil)
		if err != nil {
			return fmt.Errorf("list group credentials: %w", err)
		}
	}
	byHash := make(map[string]models.Credential, len(oldCreds))
	for _, c := range oldCreds {
		// 同 hash 多条时保留最先一条；后续重复在 sync 时跳过已用
		if _, ok := byHash[c.KeyHash]; !ok {
			byHash[c.KeyHash] = c
		}
	}
	usedCred := make(map[uint]struct{}, len(oldCreds))

	for _, g := range existing {
		if _, err := repos.KeyGroup.Delete(ctx, g.ID); err != nil {
			return fmt.Errorf("delete group %d: %w", g.ID, err)
		}
	}

	for i, in := range inputs {
		group := &models.KeyGroup{
			ProviderID: providerID,
			Name:       strings.TrimSpace(in.Name),
			Weight:     in.Weight,
			Models:     in.Models,
		}
		if in.Source == groupSourcePool {
			group.PoolID = in.PoolID
		}
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

		if in.Source == groupSourcePool {
			continue
		}

		keys := normalizeInlineKeys(in.InlineKeys)
		for _, plain := range keys {
			hash := cipher.Hash(plain)
			if old, ok := byHash[hash]; ok {
				if _, used := usedCred[old.ID]; !used {
					fields := map[string]any{
						"group_id": group.ID,
						"pool_id":  nil,
					}
					if _, err := repos.Credential.UpdateFields(ctx, old.ID, fields); err != nil {
						return fmt.Errorf("reassign credential hash=%s: %w", hash, err)
					}
					usedCred[old.ID] = struct{}{}
					continue
				}
			}
			enc, encErr := cipher.Encrypt(plain)
			if encErr != nil {
				return fmt.Errorf("encrypt key: %w", encErr)
			}
			gid := group.ID
			cred := &models.Credential{
				Key:     enc,
				KeyHash: hash,
				GroupID: &gid,
				Status:  models.CredentialStatusActive,
			}
			if err := repos.Credential.Create(ctx, cred); err != nil {
				return fmt.Errorf("create credential: %w", err)
			}
			usedCred[cred.ID] = struct{}{}
		}
	}

	for _, c := range oldCreds {
		if _, ok := usedCred[c.ID]; ok {
			continue
		}
		if _, err := repos.Credential.Delete(ctx, c.ID); err != nil {
			return fmt.Errorf("delete unused credential %d: %w", c.ID, err)
		}
	}
	return nil
}

// loadProviderDetail 组装详情响应（含解密后的 inline keys）。
func loadProviderDetail(ctx context.Context, repos *repository.Repositories, provider models.Provider) (*ProviderDetail, error) {
	endpoints, err := repos.Endpoint.ListByProvider(ctx, provider.ID)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}
	groups, err := repos.KeyGroup.ListByProvider(ctx, provider.ID)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}

	groupIDs := make([]uint, 0, len(groups))
	for _, g := range groups {
		groupIDs = append(groupIDs, g.ID)
	}
	var creds []models.Credential
	if len(groupIDs) > 0 {
		creds, err = repos.Credential.ListByGroups(ctx, groupIDs, nil)
		if err != nil {
			return nil, fmt.Errorf("list credentials: %w", err)
		}
	}
	credsByGroup := make(map[uint][]models.Credential, len(groups))
	for _, c := range creds {
		if c.GroupID == nil {
			continue
		}
		credsByGroup[*c.GroupID] = append(credsByGroup[*c.GroupID], c)
	}

	cipher := credentialcrypto.Default()
	details := make([]GroupDetail, 0, len(groups))
	for _, g := range groups {
		gd := GroupDetail{
			KeyGroup:   g,
			InlineKeys: []string{},
		}
		if g.PoolID != nil {
			gd.Source = groupSourcePool
		} else {
			gd.Source = groupSourceInline
			for _, c := range credsByGroup[g.ID] {
				// 解密失败不占有意义的占位符（如 "****"）——前端原样回写会把假 key 落库
				// 顶替真实凭据。回空串让 normalizeInlineKeys 在提交时跳过该占位。
				plain := ""
				if cipher != nil {
					if dec, decErr := cipher.Decrypt(c.Key); decErr == nil {
						plain = dec
					}
				}
				gd.InlineKeys = append(gd.InlineKeys, plain)
			}
		}
		details = append(details, gd)
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
		items = append(items, ProviderListItem{
			Provider:      p,
			EndpointCount: epCounts[p.ID],
			GroupCount:    groupCounts[p.ID],
		})
	}
	return items, nil
}
