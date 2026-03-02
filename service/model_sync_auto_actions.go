package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// autoAssociateModels 自动关联模型。
func (s *ModelSyncService) autoAssociateModels(ctx context.Context) {
	allModels, err := gorm.G[models.Model](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get models for auto-associate", "error", err)
		return
	}

	allProviders, err := gorm.G[models.Provider](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get providers for auto-associate", "error", err)
		return
	}

	existingAssociations, err := gorm.G[models.ModelWithProvider](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get existing associations", "error", err)
		return
	}

	manualTemplateItems, err := gorm.G[models.ModelTemplateItem](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get template items for auto-associate", "error", err)
		return
	}

	existingMap := make(map[string]bool)
	for _, assoc := range existingAssociations {
		key := fmt.Sprintf("%d_%d_%s", assoc.ModelID, assoc.ProviderID, assoc.ProviderModel)
		existingMap[key] = true
	}

	templateIndex := BuildTemplateIndexFromData(allModels, existingAssociations, manualTemplateItems)

	defaultPriority := 10
	if setting, err := gorm.G[models.Setting](s.db).Where("key = ?", models.SettingKeyAutoPriorityDecayDefault).First(ctx); err == nil {
		if val, err := strconv.Atoi(setting.Value); err == nil {
			defaultPriority = val
		}
	}

	addedCount := 0
	for _, provider := range allProviders {
		if provider.Blacklisted != nil && *provider.Blacklisted {
			continue
		}

		providerModels, err := GetProviderModels(ctx, provider)
		if err != nil {
			slog.Warn("failed to get provider models", "provider_id", provider.ID, "error", err)
			continue
		}

		for _, providerModel := range providerModels {
			matchedModelIDs := templateIndex.Match(providerModel)
			for _, modelID := range matchedModelIDs {
				model, err := gorm.G[models.Model](s.db).Where("id = ?", modelID).First(ctx)
				if err != nil {
					slog.Warn("failed to get model for auto-associate check", "model_id", modelID, "error", err)
					continue
				}

				if model.AutoAssociate != nil && !*model.AutoAssociate {
					slog.Debug("skipping auto-associate for model", "model_id", modelID, "model_name", model.Name)
					continue
				}

				key := fmt.Sprintf("%d_%d_%s", modelID, provider.ID, providerModel)
				if existingMap[key] {
					continue
				}

				trueVal := true
				falseVal := false
				newAssoc := models.ModelWithProvider{
					ModelID:          modelID,
					ProviderModel:    providerModel,
					ProviderID:       provider.ID,
					ToolCall:         &trueVal,
					StructuredOutput: &falseVal,
					Image:            &falseVal,
					WithHeader:       &falseVal,
					Status:           &trueVal,
					CustomerHeaders:  map[string]string{},
					Weight:           5,
					Priority:         defaultPriority,
				}

				if err := gorm.G[models.ModelWithProvider](s.db).Create(ctx, &newAssoc); err == nil {
					existingMap[key] = true
					addedCount++
				}
			}
		}
	}

	if addedCount > 0 {
		slog.Info("auto-associated models", "count", addedCount)
	}
}

// cleanInvalidAssociations 清理无效关联。
func (s *ModelSyncService) cleanInvalidAssociations(ctx context.Context) {
	allAssociations, err := gorm.G[models.ModelWithProvider](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get associations for auto-clean", "error", err)
		return
	}

	allProviders, err := gorm.G[models.Provider](s.db).Find(ctx)
	if err != nil {
		slog.Error("failed to get providers for auto-clean", "error", err)
		return
	}

	providerMap := make(map[uint]*models.Provider)
	for i := range allProviders {
		providerMap[allProviders[i].ID] = &allProviders[i]
	}

	removedCount := 0
	for _, assoc := range allAssociations {
		shouldDelete := false
		provider, providerExists := providerMap[assoc.ProviderID]
		if !providerExists {
			shouldDelete = true
		} else {
			providerModels, err := GetProviderModels(ctx, *provider)
			if err != nil {
				slog.Warn("failed to get provider models", "provider_id", provider.ID, "error", err)
				continue
			}

			modelExists := false
			for _, providerModel := range providerModels {
				if providerModel == assoc.ProviderModel {
					modelExists = true
					break
				}
			}
			if !modelExists {
				shouldDelete = true
			}
		}

		if shouldDelete {
			if _, err := gorm.G[models.ModelWithProvider](s.db).Where("id = ?", assoc.ID).Delete(ctx); err == nil {
				removedCount++
			}
		}
	}

	if removedCount > 0 {
		slog.Info("auto-cleaned invalid associations", "count", removedCount)
	}
}
