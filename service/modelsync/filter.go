package modelsync

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"gorm.io/gorm"
)

// filterModelsByRules 根据配置的规则过滤模型列表。
func (s *Service) filterModelsByRules(ctx context.Context, upstreamModels []providers.Model) ([]providers.Model, error) {
	setting, err := gorm.G[models.Setting](s.db).Where("key = ?", models.SettingKeyModelSyncFilterRules).First(ctx)
	if err != nil {
		return upstreamModels, err
	}

	var rules []string
	if err := json.Unmarshal([]byte(setting.Value), &rules); err != nil {
		return upstreamModels, err
	}
	if len(rules) == 0 {
		return upstreamModels, nil
	}

	filtered := make([]providers.Model, 0, len(upstreamModels))
	for _, model := range upstreamModels {
		if matchesAnyRule(model.ID, rules) {
			filtered = append(filtered, model)
		}
	}

	return filtered, nil
}

// matchesAnyRule 检查模型 ID 是否匹配任一规则。
func matchesAnyRule(modelID string, rules []string) bool {
	for _, rule := range rules {
		if strings.Contains(modelID, rule) {
			return true
		}
	}
	return false
}
