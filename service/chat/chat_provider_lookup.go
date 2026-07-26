package chat

import (
	"context"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
	"github.com/samber/lo"
)

func queryEnabledModelProviders(ctx context.Context, modelID uint, before Before) ([]models.ModelWithProvider, error) {
	var caps repository.ModelCapabilityFilter
	if getStrictCapabilityMatch(ctx) {
		caps = repository.ModelCapabilityFilter{
			ToolCall:         before.toolCall,
			StructuredOutput: before.structuredOutput,
			Image:            before.image,
		}
	}

	return repos().ModelWithProvider.ListEnabledByModelID(ctx, modelID, caps)
}

func buildProviderMapByModelProviders(ctx context.Context, modelWithProviders []models.ModelWithProvider) (map[uint]models.Provider, error) {
	providerIDs := lo.Map(modelWithProviders, func(mp models.ModelWithProvider, _ int) uint { return mp.ProviderID })
	providersList, err := repos().Provider.ListByIDs(ctx, providerIDs)
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
