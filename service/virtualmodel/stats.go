package virtualmodel

import (
	"context"
	"fmt"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// GetVirtualModelStats 获取虚拟模型统计信息。
func (s *Service) GetVirtualModelStats(ctx context.Context, virtualModelID uint) (map[string]interface{}, error) {
	virtualModel, err := gorm.G[models.VirtualModel](s.db).Where("id = ?", virtualModelID).First(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual model: %w", err)
	}

	mappings, err := gorm.G[models.VirtualModelMapping](s.db).
		Where("virtual_model_id = ?", virtualModelID).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get mappings: %w", err)
	}

	stats := map[string]interface{}{
		"virtual_model_id":   virtualModel.ID,
		"virtual_model_name": virtualModel.Name,
		"strategy":           virtualModel.Strategy,
		"total_mappings":     len(mappings),
		"enabled_mappings":   0,
		"disabled_mappings":  0,
	}

	for _, mapping := range mappings {
		if mapping.Enabled != nil && *mapping.Enabled {
			stats["enabled_mappings"] = stats["enabled_mappings"].(int) + 1
			continue
		}
		stats["disabled_mappings"] = stats["disabled_mappings"].(int) + 1
	}

	return stats, nil
}
