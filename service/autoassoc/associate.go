package autoassoc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/modelsync"
)

// skipAutoAssociate 是自动关联路径的模型级跳过谓词。
var skipAutoAssociate = func(m models.Model) bool { return !allowsAutoAssociate(m) }

type associateData struct {
	allModels            []models.Model
	allProviders         []models.Provider
	existingAssociations []models.ModelWithProvider
	manualTemplateItems  []models.ModelTemplateItem
}

func (s *Service) fetchAssociateData(ctx context.Context) (associateData, error) {
	repos := s.repositories()

	allModels, err := repos.Model.List(ctx)
	if err != nil {
		return associateData{}, fmt.Errorf("get models: %w", err)
	}
	allProviders, err := repos.Provider.List(ctx, repository.ProviderFilter{})
	if err != nil {
		return associateData{}, fmt.Errorf("get providers: %w", err)
	}
	existingAssociations, err := repos.ModelWithProvider.ListAll(ctx)
	if err != nil {
		return associateData{}, fmt.Errorf("get existing associations: %w", err)
	}
	manualTemplateItems, err := repos.ModelTemplateItem.ListAll(ctx)
	if err != nil {
		return associateData{}, fmt.Errorf("get template items: %w", err)
	}

	return associateData{
		allModels:            allModels,
		allProviders:         allProviders,
		existingAssociations: existingAssociations,
		manualTemplateItems:  manualTemplateItems,
	}, nil
}

func (s *Service) associate(ctx context.Context, skip func(models.Model) bool) (Result, error) {
	data, err := s.fetchAssociateData(ctx)
	if err != nil {
		return Result{}, err
	}

	defaultPriority := s.settingInt(ctx, models.SettingKeyAutoPriorityDecayDefault, DefaultPriorityFallback, 0)
	defaultWeight := s.settingInt(ctx, models.SettingKeyAutoWeightDecayDefault, DefaultWeightFallback, 0)
	result := Result{}
	repos := s.repositories()

	s.forEachMissingAssociation(
		ctx,
		data,
		skip,
		func(provider models.Provider, err error) {
			slog.Warn("failed to get provider models", "provider", provider.Name, "error", err)
		},
		func(candidate associationCandidate, key string, existingMap map[string]bool) {
			newAssoc := NewDefaultAssociation(
				candidate.ModelID,
				candidate.ProviderID,
				candidate.ProviderModel,
				defaultWeight,
				defaultPriority,
			)
			if err := repos.ModelWithProvider.Create(ctx, &newAssoc); err != nil {
				result.Failed++
				slog.Warn("failed to create association",
					"model", candidate.ModelName,
					"provider", candidate.ProviderName,
					"error", err,
				)
				return
			}
			existingMap[key] = true
			result.Success++
		},
	)

	return result, nil
}

func (s *Service) forEachMissingAssociation(
	ctx context.Context,
	data associateData,
	skip func(models.Model) bool,
	onProviderModelsError func(provider models.Provider, err error),
	visit func(candidate associationCandidate, key string, existingMap map[string]bool),
) {
	modelByID := indexModelsByID(data.allModels)
	existingMap := buildExistingAssociationMap(data.existingAssociations)
	templateIndex := s.buildIndex(data.allModels, data.existingAssociations, data.manualTemplateItems)

	for _, provider := range data.allProviders {
		if isProviderBlacklisted(provider) {
			continue
		}

		providerModels, err := modelsync.GetProviderModels(ctx, provider)
		if err != nil {
			if onProviderModelsError != nil {
				onProviderModelsError(provider, err)
			}
			continue
		}

		for _, providerModel := range providerModels {
			matchedModelIDs := templateIndex.Match(providerModel)
			for _, modelID := range matchedModelIDs {
				model, ok := modelByID[modelID]
				if !ok {
					continue
				}
				if skip != nil && skip(model) {
					continue
				}

				key := buildAssociationKey(model.ID, provider.ID, providerModel)
				if existingMap[key] {
					continue
				}

				visit(associationCandidate{
					ModelID:       model.ID,
					ModelName:     model.Name,
					ProviderID:    provider.ID,
					ProviderName:  provider.Name,
					ProviderModel: providerModel,
				}, key, existingMap)
			}
		}
	}
}
