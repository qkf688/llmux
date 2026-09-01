package modelsync

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

func dropCustomModels(config string) (string, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return "", err
	}

	delete(parsed, "custom_models")
	delete(parsed, "upstream_models")

	updated, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}
	return string(updated), nil
}

func extractCustomModels(config string) []string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return nil
	}
	custom, ok := parsed["custom_models"].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(custom))
	for _, item := range custom {
		if modelID, ok := item.(string); ok {
			out = append(out, modelID)
		}
	}
	return out
}

// extractAllModels 保留给旧 Config 解析测；组织级目录请用 mergeProviderModelCatalog。
func extractAllModels(config string) []string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return []string{}
	}

	modelList := make([]string, 0)
	if upstream, ok := parsed["upstream_models"].([]interface{}); ok {
		for _, item := range upstream {
			if modelID, ok := item.(string); ok {
				modelList = append(modelList, modelID)
			}
		}
	}
	if custom, ok := parsed["custom_models"].([]interface{}); ok {
		for _, item := range custom {
			if modelID, ok := item.(string); ok {
				modelList = append(modelList, modelID)
			}
		}
	}

	return modelList
}

func extractUpstreamModels(config string) []string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return []string{}
	}

	upstream, ok := parsed["upstream_models"].([]interface{})
	if !ok {
		return []string{}
	}

	modelList := make([]string, 0, len(upstream))
	for _, item := range upstream {
		if modelID, ok := item.(string); ok {
			modelList = append(modelList, modelID)
		}
	}

	return modelList
}

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
	for _, id := range extractCustomModels(config) {
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
