package virtualmodel

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// UpdateRoundRobinIndex 更新轮询索引（在请求成功后调用）。
func (s *Service) UpdateRoundRobinIndex(virtualModelID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mappings, err := gorm.G[models.VirtualModelMapping](s.db).
		Where("virtual_model_id = ? AND enabled = ?", virtualModelID, true).
		Find(context.Background())
	if err != nil || len(mappings) == 0 {
		return
	}

	currentIndex := s.roundRobinState[virtualModelID]
	s.roundRobinState[virtualModelID] = (currentIndex + 1) % len(mappings)

	slog.Debug("updated round_robin index", "virtual_model_id", virtualModelID, "new_index", s.roundRobinState[virtualModelID])
}
