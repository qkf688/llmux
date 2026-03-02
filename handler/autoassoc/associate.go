package autoassoc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/handler/settings"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func fetchAutoAssociateData(ctx context.Context) ([]models.Model, []models.Provider, []models.ModelWithProvider, []models.ModelTemplateItem, error) {
	allModels, err := gorm.G[models.Model](models.DB).Find(ctx)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("get models: %w", err)
	}

	allProviders, err := gorm.G[models.Provider](models.DB).Find(ctx)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("get providers: %w", err)
	}

	existingAssociations, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("get existing associations: %w", err)
	}

	manualTemplateItems, err := gorm.G[models.ModelTemplateItem](models.DB).Find(ctx)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("get template items: %w", err)
	}

	return allModels, allProviders, existingAssociations, manualTemplateItems, nil
}

func forEachMissingAssociation(
	ctx context.Context,
	allModels []models.Model,
	allProviders []models.Provider,
	existingAssociations []models.ModelWithProvider,
	manualTemplateItems []models.ModelTemplateItem,
	onProviderModelsError func(provider models.Provider, err error),
	visit func(candidate associationCandidate, key string, existingMap map[string]bool),
) {
	modelByID := indexModelsByID(allModels)
	existingMap := buildExistingAssociationMap(existingAssociations)
	templateIndex := service.BuildTemplateIndexFromData(allModels, existingAssociations, manualTemplateItems)

	for _, provider := range allProviders {
		if isProviderBlacklisted(provider) {
			continue
		}

		providerModels, err := service.GetProviderModels(ctx, provider)
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

// PreviewAutoAssociate 预览将要添加的关联。
func PreviewAutoAssociate(c *gin.Context) {
	ctx := c.Request.Context()

	allModels, allProviders, existingAssociations, manualTemplateItems, err := fetchAutoAssociateData(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to "+err.Error())
		return
	}

	previews := make([]AssociationPreview, 0)
	forEachMissingAssociation(
		ctx,
		allModels,
		allProviders,
		existingAssociations,
		manualTemplateItems,
		nil,
		func(candidate associationCandidate, _ string, _ map[string]bool) {
			previews = append(previews, AssociationPreview{
				ModelID:       candidate.ModelID,
				ModelName:     candidate.ModelName,
				ProviderID:    candidate.ProviderID,
				ProviderName:  candidate.ProviderName,
				ProviderModel: candidate.ProviderModel,
			})
		},
	)

	common.Success(c, previews)
}

// AutoAssociateModels 一键添加关联：将提供商模型自动关联到模板匹配的模型（并集规则）。
func AutoAssociateModels(c *gin.Context) {
	ctx := c.Request.Context()

	allModels, allProviders, existingAssociations, manualTemplateItems, err := fetchAutoAssociateData(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to "+err.Error())
		return
	}

	defaultPriority := settings.GetAutoPriorityDecayDefault(ctx)
	addedCount := 0

	forEachMissingAssociation(
		ctx,
		allModels,
		allProviders,
		existingAssociations,
		manualTemplateItems,
		func(provider models.Provider, err error) {
			slog.Warn("failed to get provider models", "provider", provider.Name, "error", err)
		},
		func(candidate associationCandidate, key string, existingMap map[string]bool) {
			newAssoc := newDefaultAssociation(
				candidate.ModelID,
				candidate.ProviderID,
				candidate.ProviderModel,
				defaultPriority,
			)
			if err := gorm.G[models.ModelWithProvider](models.DB).Create(ctx, &newAssoc); err != nil {
				slog.Warn("failed to create association", "model", candidate.ModelName, "provider", candidate.ProviderName, "error", err)
				return
			}

			existingMap[key] = true
			addedCount++
		},
	)

	common.Success(c, map[string]interface{}{
		"added": addedCount,
	})
}

// TriggerAutoAssociate 触发自动关联。
func TriggerAutoAssociate(ctx context.Context) {
	if !settings.GetSettingBool(ctx, models.SettingKeyAutoAssociateOnAdd) {
		slog.Info("auto-associate disabled")
		return
	}
	slog.Info("auto-associate triggered")

	allModels, allProviders, existingAssociations, manualTemplateItems, err := fetchAutoAssociateData(ctx)
	if err != nil {
		return
	}

	defaultPriority := settings.GetAutoPriorityDecayDefault(ctx)
	addedCount := 0

	forEachMissingAssociation(
		ctx,
		allModels,
		allProviders,
		existingAssociations,
		manualTemplateItems,
		nil,
		func(candidate associationCandidate, key string, existingMap map[string]bool) {
			newAssoc := newDefaultAssociation(
				candidate.ModelID,
				candidate.ProviderID,
				candidate.ProviderModel,
				defaultPriority,
			)
			if err := gorm.G[models.ModelWithProvider](models.DB).Create(ctx, &newAssoc); err == nil {
				existingMap[key] = true
				addedCount++
			}
		},
	)

	if addedCount > 0 {
		slog.Info("auto-associated models", "count", addedCount)
	}
}
