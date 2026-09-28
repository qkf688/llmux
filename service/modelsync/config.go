package modelsync

import (
	"context"
	"strings"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/repository"
)

func splitModelsCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func mergeGroupWhitelists(groups []models.KeyGroup) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, g := range groups {
		for _, id := range splitModelsCSV(g.Models) {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// mergeProviderModelCatalog 组织级模型目录 = 各分组非空白名单并集 ∪ custom_models。
// 空白名单不贡献 ID（空 = 未同步/不限制，不是「拥有全集」）。
func mergeProviderModelCatalog(config string, groups []models.KeyGroup) []string {
	seen := make(map[string]struct{})
	out := mergeGroupWhitelists(groups)
	for _, id := range out {
		seen[id] = struct{}{}
	}
	for _, id := range common.ExtractCustomModels(config) {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func collectGroupWhitelistUnion(ctx context.Context, repos *repository.Repositories, providerID uint) ([]string, error) {
	groups, err := repos.KeyGroup.ListByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	return mergeGroupWhitelists(groups), nil
}

// GetProviderModels 获取提供商组织级模型目录（分组白名单并集 ∪ custom_models）。
// repos 为 nil 时用 repository.Default()。
func GetProviderModels(ctx context.Context, provider models.Provider, repos *repository.Repositories) ([]string, error) {
	if repos == nil {
		repos = repository.Default()
	}
	groups, err := repos.KeyGroup.ListByProvider(ctx, provider.ID)
	if err != nil {
		return nil, err
	}
	return mergeProviderModelCatalog(provider.Config, groups), nil
}

// GroupRef 分组引用（ID + Name），供前端徽标与筛选使用。
type GroupRef struct {
	ID   uint
	Name string
}

// UpstreamModel 上游模型条目 + 所属分组列表（ListUpstreamModels 返回）。
// Groups 按分组遍历顺序追加（GroupCandidates 的 ID ASC）。
type UpstreamModel struct {
	providers.Model
	Groups []GroupRef
}

// UpstreamModelEntry 单条上游模型及其所属分组列表。
// Groups 按 KeyGroup.ID ASC 排序（与 ListByProvider 一致）。
type UpstreamModelEntry struct {
	ID     string
	Groups []GroupRef
}

// buildUpstreamEntries 从分组白名单构建带分组归属的模型条目。
// 同一模型出现在多个分组时合并 Groups；模型首次出现的顺序决定输出顺序。
func buildUpstreamEntries(groups []models.KeyGroup) []UpstreamModelEntry {
	seen := make(map[string][]GroupRef)
	order := make([]string, 0)
	for _, g := range groups {
		ref := GroupRef{ID: g.ID, Name: g.Name}
		for _, id := range splitModelsCSV(g.Models) {
			if _, ok := seen[id]; !ok {
				order = append(order, id)
			}
			seen[id] = append(seen[id], ref)
		}
	}
	out := make([]UpstreamModelEntry, 0, len(order))
	for _, id := range order {
		out = append(out, UpstreamModelEntry{ID: id, Groups: seen[id]})
	}
	return out
}

// ProviderModelCatalog 单个供应商的组织级目录，按来源分类：
// Upstream = 各分组白名单并集（同步真相，带分组归属）；Custom = config.custom_models（手填）。
// 前端目录页（models / model-providers / providers）的 API 数据源，替代
// Config.upstream_models 遗留客户端解析。
type ProviderModelCatalog struct {
	ProviderID uint
	Upstream   []UpstreamModelEntry
	Custom     []string
}

// GetProviderModelCatalogs 批量获取全部供应商的目录。repos 为 nil 时用
// repository.Default()。空目录显式空切片（非 nil），序列化为 [] 而非 null。
// 供应商量级为管理端规模（几十内），按 provider 逐查分组与 GetProviderModels
// 同构；若目录页出现性能问题再做单查询批量化。
func GetProviderModelCatalogs(ctx context.Context, repos *repository.Repositories) ([]ProviderModelCatalog, error) {
	if repos == nil {
		repos = repository.Default()
	}
	providers, err := repos.Provider.List(ctx, repository.ProviderFilter{})
	if err != nil {
		return nil, err
	}

	out := make([]ProviderModelCatalog, 0, len(providers))
	for _, p := range providers {
		groups, err := repos.KeyGroup.ListByProvider(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		upstream := buildUpstreamEntries(groups)
		custom := common.ExtractCustomModels(p.Config)
		if upstream == nil {
			upstream = []UpstreamModelEntry{}
		}
		if custom == nil {
			custom = []string{}
		}
		out = append(out, ProviderModelCatalog{
			ProviderID: p.ID,
			Upstream:   upstream,
			Custom:     custom,
		})
	}
	return out, nil
}
