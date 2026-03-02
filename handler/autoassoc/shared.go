package autoassoc

import (
	"fmt"

	"github.com/atopos31/llmio/models"
)

func buildAssociationKey(modelID, providerID uint, providerModel string) string {
	return fmt.Sprintf("%d_%d_%s", modelID, providerID, providerModel)
}

func buildExistingAssociationMap(associations []models.ModelWithProvider) map[string]bool {
	existingMap := make(map[string]bool, len(associations))
	for _, assoc := range associations {
		existingMap[buildAssociationKey(assoc.ModelID, assoc.ProviderID, assoc.ProviderModel)] = true
	}
	return existingMap
}

func indexModelsByID(allModels []models.Model) map[uint]models.Model {
	modelByID := make(map[uint]models.Model, len(allModels))
	for _, model := range allModels {
		modelByID[model.ID] = model
	}
	return modelByID
}

func indexProvidersByID(allProviders []models.Provider) map[uint]*models.Provider {
	providerByID := make(map[uint]*models.Provider, len(allProviders))
	for i := range allProviders {
		providerByID[allProviders[i].ID] = &allProviders[i]
	}
	return providerByID
}

func providerModelExists(providerModels []string, target string) bool {
	for _, model := range providerModels {
		if model == target {
			return true
		}
	}
	return false
}

func isProviderBlacklisted(provider models.Provider) bool {
	return provider.Blacklisted != nil && *provider.Blacklisted
}

func newDefaultAssociation(modelID, providerID uint, providerModel string, defaultPriority int) models.ModelWithProvider {
	trueVal := true
	falseVal := false
	return models.ModelWithProvider{
		ModelID:          modelID,
		ProviderModel:    providerModel,
		ProviderID:       providerID,
		ToolCall:         &trueVal,
		StructuredOutput: &falseVal,
		Image:            &falseVal,
		WithHeader:       &falseVal,
		Status:           &trueVal,
		CustomerHeaders:  map[string]string{},
		Weight:           5,
		Priority:         defaultPriority,
	}
}
