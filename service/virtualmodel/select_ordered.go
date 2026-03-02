package virtualmodel

import (
	"log/slog"
	"math/rand/v2"
	"sort"

	"github.com/atopos31/llmio/models"
)

// selectOrderedByPriority 按优先级降序，同优先级按权重降序排序。
func (s *Service) selectOrderedByPriority(mappings []models.VirtualModelMapping, modelByID map[uint]models.Model) ([]OrderedRealModel, error) {
	sorted := append([]models.VirtualModelMapping(nil), mappings...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority > sorted[j].Priority
		}
		return sorted[i].Weight > sorted[j].Weight
	})

	ordered := make([]OrderedRealModel, 0, len(sorted))
	for _, mapping := range sorted {
		model, ok := modelByID[mapping.RealModelID]
		if !ok {
			continue
		}
		ordered = append(ordered, OrderedRealModel{
			Model:    model,
			Priority: mapping.Priority,
			Weight:   mapping.Weight,
		})
	}

	slog.Debug("selected ordered models by priority", "count", len(ordered), "strategy", "priority")
	return ordered, nil
}

// selectOrderedByRoundRobin 从当前索引开始的轮询顺序。
func (s *Service) selectOrderedByRoundRobin(virtualModelID uint, mappings []models.VirtualModelMapping, modelByID map[uint]models.Model) ([]OrderedRealModel, error) {
	s.mu.RLock()
	currentIndex := s.roundRobinState[virtualModelID]
	s.mu.RUnlock()

	if currentIndex >= len(mappings) {
		currentIndex = 0
	}

	ordered := make([]OrderedRealModel, 0, len(mappings))
	for i := 0; i < len(mappings); i++ {
		idx := (currentIndex + i) % len(mappings)
		mapping := mappings[idx]
		model, ok := modelByID[mapping.RealModelID]
		if !ok {
			continue
		}
		ordered = append(ordered, OrderedRealModel{
			Model:    model,
			Priority: mapping.Priority,
			Weight:   mapping.Weight,
		})
	}

	slog.Debug("selected ordered models by round_robin", "count", len(ordered), "start_index", currentIndex)
	return ordered, nil
}

// selectOrderedByRandom 随机打乱顺序。
func (s *Service) selectOrderedByRandom(mappings []models.VirtualModelMapping, modelByID map[uint]models.Model) ([]OrderedRealModel, error) {
	ordered := make([]OrderedRealModel, 0, len(mappings))
	for _, mapping := range mappings {
		model, ok := modelByID[mapping.RealModelID]
		if !ok {
			continue
		}
		ordered = append(ordered, OrderedRealModel{
			Model:    model,
			Priority: mapping.Priority,
			Weight:   mapping.Weight,
		})
	}

	for i := len(ordered) - 1; i > 0; i-- {
		j := rand.IntN(i + 1)
		ordered[i], ordered[j] = ordered[j], ordered[i]
	}

	slog.Debug("selected ordered models by random", "count", len(ordered))
	return ordered, nil
}
