package modelsync

import (
	"context"
	"encoding/json"

	"github.com/qkf688/llmux/models"
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

func buildConfigWithAllModels(config string, modelList []string) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return config
	}

	parsed["upstream_models"] = modelList

	updated, err := json.Marshal(parsed)
	if err != nil {
		return config
	}
	return string(updated)
}

// GetProviderModels 获取提供商的所有模型列表。
func GetProviderModels(ctx context.Context, provider models.Provider) ([]string, error) {
	return extractAllModels(provider.Config), nil
}
