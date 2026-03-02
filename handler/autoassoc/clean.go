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

type cleanData struct {
	allAssociations []models.ModelWithProvider
	allProviders    []models.Provider
	allModels       []models.Model
}

func fetchCleanData(ctx context.Context, includeModels bool) (cleanData, error) {
	allAssociations, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	if err != nil {
		return cleanData{}, fmt.Errorf("get associations: %w", err)
	}

	allProviders, err := gorm.G[models.Provider](models.DB).Find(ctx)
	if err != nil {
		return cleanData{}, fmt.Errorf("get providers: %w", err)
	}

	data := cleanData{
		allAssociations: allAssociations,
		allProviders:    allProviders,
	}
	if !includeModels {
		return data, nil
	}

	allModels, err := gorm.G[models.Model](models.DB).Find(ctx)
	if err != nil {
		return cleanData{}, fmt.Errorf("get models: %w", err)
	}
	data.allModels = allModels
	return data, nil
}

func forEachInvalidAssociation(
	ctx context.Context,
	allAssociations []models.ModelWithProvider,
	allProviders []models.Provider,
	onProviderModelsError func(provider *models.Provider, err error),
	visit func(assoc models.ModelWithProvider, provider *models.Provider),
) {
	providerMap := indexProvidersByID(allProviders)

	for _, assoc := range allAssociations {
		provider, providerExists := providerMap[assoc.ProviderID]
		if !providerExists {
			visit(assoc, nil)
			continue
		}

		providerModels, err := service.GetProviderModels(ctx, *provider)
		if err != nil {
			if onProviderModelsError != nil {
				onProviderModelsError(provider, err)
			}
			continue
		}

		if providerModelExists(providerModels, assoc.ProviderModel) {
			continue
		}

		visit(assoc, provider)
	}
}

// PreviewCleanInvalid 预览将要删除的关联。
func PreviewCleanInvalid(c *gin.Context) {
	ctx := c.Request.Context()

	data, err := fetchCleanData(ctx, true)
	if err != nil {
		common.InternalServerError(c, "Failed to "+err.Error())
		return
	}
	modelByID := indexModelsByID(data.allModels)

	previews := make([]AssociationPreview, 0)
	forEachInvalidAssociation(
		ctx,
		data.allAssociations,
		data.allProviders,
		nil,
		func(assoc models.ModelWithProvider, provider *models.Provider) {
			providerName := ""
			if provider != nil {
				providerName = provider.Name
			}

			modelName := ""
			if model, ok := modelByID[assoc.ModelID]; ok {
				modelName = model.Name
			}

			previews = append(previews, AssociationPreview{
				ModelID:       assoc.ModelID,
				ModelName:     modelName,
				ProviderID:    assoc.ProviderID,
				ProviderName:  providerName,
				ProviderModel: assoc.ProviderModel,
			})
		},
	)

	common.Success(c, previews)
}

// CleanInvalidAssociations 清除无效关联：删除提供商不存在或模型不存在的关联。
func CleanInvalidAssociations(c *gin.Context) {
	ctx := c.Request.Context()

	data, err := fetchCleanData(ctx, false)
	if err != nil {
		common.InternalServerError(c, "Failed to "+err.Error())
		return
	}

	removedCount := 0
	forEachInvalidAssociation(
		ctx,
		data.allAssociations,
		data.allProviders,
		func(provider *models.Provider, err error) {
			slog.Warn("failed to get provider models", "provider_id", provider.ID, "error", err)
		},
		func(assoc models.ModelWithProvider, _ *models.Provider) {
			if _, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", assoc.ID).Delete(ctx); err != nil {
				slog.Warn("failed to delete invalid association", "id", assoc.ID, "error", err)
				return
			}
			removedCount++
		},
	)

	common.Success(c, map[string]interface{}{
		"removed": removedCount,
	})
}

// TriggerAutoClean 触发自动清理。
func TriggerAutoClean(ctx context.Context) {
	if !settings.GetSettingBool(ctx, models.SettingKeyAutoCleanOnDelete) {
		slog.Info("auto-clean disabled")
		return
	}
	slog.Info("auto-clean triggered")

	data, err := fetchCleanData(ctx, false)
	if err != nil {
		return
	}

	removedCount := 0
	forEachInvalidAssociation(
		ctx,
		data.allAssociations,
		data.allProviders,
		nil,
		func(assoc models.ModelWithProvider, _ *models.Provider) {
			if _, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", assoc.ID).Delete(ctx); err == nil {
				removedCount++
			}
		},
	)

	if removedCount > 0 {
		slog.Info("auto-cleaned invalid associations", "count", removedCount)
	}
}
