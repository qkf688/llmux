package virtualmodel

import (
	"fmt"
	"log/slog"

	"github.com/atopos31/llmio/models"
)

// selectByPriority 按优先级+权重选择。
func (s *Service) selectByPriority(mappings []models.VirtualModelMapping, modelByID map[uint]models.Model) (*models.Model, error) {
	maxPriority := mappings[0].Priority
	for _, mapping := range mappings {
		if mapping.Priority > maxPriority {
			maxPriority = mapping.Priority
		}
	}

	highPriorityMappings := make([]models.VirtualModelMapping, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.Priority == maxPriority {
			highPriorityMappings = append(highPriorityMappings, mapping)
		}
	}

	if len(highPriorityMappings) == 1 {
		model, ok := modelByID[highPriorityMappings[0].RealModelID]
		if !ok {
			return nil, errModelNotFoundInMap
		}
		return cloneModel(model), nil
	}

	weightItems := make(map[uint]int, len(highPriorityMappings))
	for _, mapping := range highPriorityMappings {
		weightItems[mapping.RealModelID] = mapping.Weight
	}

	selectedID, err := weightedRandom(weightItems)
	if err != nil {
		return nil, fmt.Errorf("failed to select by weight: %w", err)
	}

	model, ok := modelByID[*selectedID]
	if !ok {
		return nil, errSelectedModelNotFound
	}

	slog.Debug("selected model by priority", "model_id", model.ID, "model_name", model.Name, "priority", maxPriority)
	return cloneModel(model), nil
}

// selectByRoundRobin 轮询选择。
func (s *Service) selectByRoundRobin(virtualModelID uint, mappings []models.VirtualModelMapping, modelByID map[uint]models.Model) (*models.Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentIndex := s.roundRobinState[virtualModelID]
	if currentIndex >= len(mappings) {
		currentIndex = 0
	}

	selectedMapping := mappings[currentIndex]
	model, ok := modelByID[selectedMapping.RealModelID]
	if !ok {
		return nil, errSelectedModelNotFound
	}

	s.roundRobinState[virtualModelID] = (currentIndex + 1) % len(mappings)

	slog.Debug("selected model by round_robin", "model_id", model.ID, "model_name", model.Name, "index", currentIndex)
	return cloneModel(model), nil
}

// selectByRandom 完全随机选择。
func (s *Service) selectByRandom(mappings []models.VirtualModelMapping, modelByID map[uint]models.Model) (*models.Model, error) {
	weightItems := make(map[uint]int, len(mappings))
	for _, mapping := range mappings {
		weightItems[mapping.RealModelID] = 1
	}

	selectedID, err := weightedRandom(weightItems)
	if err != nil {
		return nil, fmt.Errorf("failed to select randomly: %w", err)
	}

	model, ok := modelByID[*selectedID]
	if !ok {
		return nil, errSelectedModelNotFound
	}

	slog.Debug("selected model by random", "model_id", model.ID, "model_name", model.Name)
	return cloneModel(model), nil
}

func cloneModel(model models.Model) *models.Model {
	result := model
	return &result
}
