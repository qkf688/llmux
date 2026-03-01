package service

import (
	"context"

	"github.com/atopos31/llmio/models"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func queryEnabledModelProviders(ctx context.Context, modelID uint, before Before) ([]models.ModelWithProvider, error) {
	modelWithProviderChain := gorm.G[models.ModelWithProvider](models.DB).
		Where("model_id = ? AND status = ?", modelID, true)

	if getStrictCapabilityMatch(ctx) {
		if before.toolCall {
			modelWithProviderChain = modelWithProviderChain.Where("tool_call = ?", true)
		}
		if before.structuredOutput {
			modelWithProviderChain = modelWithProviderChain.Where("structured_output = ?", true)
		}
		if before.image {
			modelWithProviderChain = modelWithProviderChain.Where("image = ?", true)
		}
	}

	return modelWithProviderChain.Find(ctx)
}

func buildProviderMapByModelProviders(ctx context.Context, modelWithProviders []models.ModelWithProvider) (map[uint]models.Provider, error) {
	providersList, err := gorm.G[models.Provider](models.DB).
		Where("id IN ?", lo.Map(modelWithProviders, func(mp models.ModelWithProvider, _ int) uint { return mp.ProviderID })).
		Find(ctx)
	if err != nil {
		return nil, err
	}
	return lo.KeyBy(providersList, func(p models.Provider) uint { return p.ID }), nil
}

func buildSelectionItemsByModelProviders(modelWithProviders []models.ModelWithProvider, providerMap map[uint]models.Provider) (map[uint]int, map[uint]int) {
	weightItems := make(map[uint]int)
	priorityItems := make(map[uint]int)
	for _, mp := range modelWithProviders {
		if _, ok := providerMap[mp.ProviderID]; !ok {
			continue
		}
		weightItems[mp.ID] = mp.Weight
		priorityItems[mp.ID] = mp.Priority
	}
	return weightItems, priorityItems
}
