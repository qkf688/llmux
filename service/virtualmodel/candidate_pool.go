package virtualmodel

import (
	"context"
	"fmt"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func (s *Service) loadCandidatePool(ctx context.Context, virtualModelID uint) (*candidatePool, error) {
	mappings, err := s.loadEnabledMappings(virtualModelID)
	if err != nil {
		return nil, err
	}

	modelIDs := collectModelIDs(mappings)
	modelByID, err := s.loadRealModels(ctx, modelIDs)
	if err != nil {
		return nil, err
	}

	blacklistedModels := s.loadBlacklistedModelSet(modelIDs)

	filteredMappings := make([]models.VirtualModelMapping, 0, len(mappings))
	filteredModels := make(map[uint]models.Model, len(modelByID))
	for _, mapping := range mappings {
		if blacklistedModels[mapping.RealModelID] {
			continue
		}
		model, ok := modelByID[mapping.RealModelID]
		if !ok {
			continue
		}
		filteredMappings = append(filteredMappings, mapping)
		filteredModels[mapping.RealModelID] = model
	}

	if len(filteredMappings) == 0 {
		return nil, errNoAvailableModelsAfterBlacklist
	}

	return &candidatePool{
		mappings:  filteredMappings,
		modelByID: filteredModels,
	}, nil
}

func (s *Service) loadEnabledMappings(virtualModelID uint) ([]models.VirtualModelMapping, error) {
	var mappings []models.VirtualModelMapping
	result := s.db.
		Where("virtual_model_id = ? AND enabled = ?", virtualModelID, true).
		Order("id ASC").
		Find(&mappings)
	if result.Error != nil {
		return nil, errNoEnabledMappings
	}
	if len(mappings) == 0 {
		return nil, errNoEnabledMappings
	}
	return mappings, nil
}

func (s *Service) loadRealModels(ctx context.Context, modelIDs []uint) (map[uint]models.Model, error) {
	realModels, err := gorm.G[models.Model](s.db).
		Where("id IN ?", modelIDs).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get real models: %w", err)
	}
	if len(realModels) == 0 {
		return nil, errNoRealModels
	}

	modelByID := make(map[uint]models.Model, len(realModels))
	for _, model := range realModels {
		modelByID[model.ID] = model
	}

	return modelByID, nil
}

func (s *Service) loadBlacklistedModelSet(modelIDs []uint) map[uint]bool {
	var modelWithProviders []models.ModelWithProvider
	if err := s.db.Where("model_id IN ?", modelIDs).Find(&modelWithProviders).Error; err != nil {
		return map[uint]bool{}
	}

	providerIDs := make([]uint, 0, len(modelWithProviders))
	for _, mwp := range modelWithProviders {
		providerIDs = append(providerIDs, mwp.ProviderID)
	}

	var providers []models.Provider
	if err := s.db.Where("id IN ?", providerIDs).Find(&providers).Error; err != nil {
		return map[uint]bool{}
	}

	providerBlacklisted := make(map[uint]bool, len(providers))
	for _, provider := range providers {
		blacklisted := false
		if provider.Blacklisted != nil {
			blacklisted = *provider.Blacklisted
		}
		providerBlacklisted[provider.ID] = blacklisted
	}

	modelBlacklisted := make(map[uint]bool)
	for _, mwp := range modelWithProviders {
		if providerBlacklisted[mwp.ProviderID] {
			modelBlacklisted[mwp.ModelID] = true
		}
	}

	return modelBlacklisted
}

func collectModelIDs(mappings []models.VirtualModelMapping) []uint {
	modelIDs := make([]uint, 0, len(mappings))
	for _, mapping := range mappings {
		modelIDs = append(modelIDs, mapping.RealModelID)
	}
	return modelIDs
}
