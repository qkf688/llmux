package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AssociationPreview 关联预览信息
type AssociationPreview struct {
	ModelID       uint   `json:"model_id"`
	ModelName     string `json:"model_name"`
	ProviderID    uint   `json:"provider_id"`
	ProviderName  string `json:"provider_name"`
	ProviderModel string `json:"provider_model"`
}

// PreviewAutoAssociate 预览将要添加的关联
func PreviewAutoAssociate(c *gin.Context) {
	ctx := c.Request.Context()

	allModels, err := gorm.G[models.Model](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get models: "+err.Error())
		return
	}

	allProviders, err := gorm.G[models.Provider](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get providers: "+err.Error())
		return
	}

	existingAssociations, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get existing associations: "+err.Error())
		return
	}

	manualTemplateItems, err := gorm.G[models.ModelTemplateItem](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get template items: "+err.Error())
		return
	}

	modelByID := make(map[uint]models.Model, len(allModels))
	for _, m := range allModels {
		modelByID[m.ID] = m
	}

	existingMap := make(map[string]bool)
	for _, assoc := range existingAssociations {
		key := fmt.Sprintf("%d_%d_%s", assoc.ModelID, assoc.ProviderID, assoc.ProviderModel)
		existingMap[key] = true
	}

	templateIndex := service.BuildTemplateIndexFromData(allModels, existingAssociations, manualTemplateItems)

	previews := make([]AssociationPreview, 0)
	for _, provider := range allProviders {
		// 跳过被拉黑的供应商
		if provider.Blacklisted != nil && *provider.Blacklisted {
			continue
		}
		providerModels, err := service.GetProviderModels(ctx, provider)
		if err != nil {
			continue
		}

		for _, providerModel := range providerModels {
			matchedModelIDs := templateIndex.Match(providerModel)
			for _, modelID := range matchedModelIDs {
				model, ok := modelByID[modelID]
				if !ok {
					continue
				}
				key := fmt.Sprintf("%d_%d_%s", model.ID, provider.ID, providerModel)
				if !existingMap[key] {
					previews = append(previews, AssociationPreview{
						ModelID:       model.ID,
						ModelName:     model.Name,
						ProviderID:    provider.ID,
						ProviderName:  provider.Name,
						ProviderModel: providerModel,
					})
				}
			}
		}
	}

	common.Success(c, previews)
}

// PreviewCleanInvalid 预览将要删除的关联
func PreviewCleanInvalid(c *gin.Context) {
	ctx := c.Request.Context()

	allAssociations, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get associations: "+err.Error())
		return
	}

	allProviders, err := gorm.G[models.Provider](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get providers: "+err.Error())
		return
	}

	allModels, err := gorm.G[models.Model](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get models: "+err.Error())
		return
	}

	providerMap := make(map[uint]*models.Provider)
	for i := range allProviders {
		providerMap[allProviders[i].ID] = &allProviders[i]
	}

	modelMap := make(map[uint]*models.Model)
	for i := range allModels {
		modelMap[allModels[i].ID] = &allModels[i]
	}

	previews := make([]AssociationPreview, 0)
	for _, assoc := range allAssociations {
		shouldDelete := false
		provider, providerExists := providerMap[assoc.ProviderID]
		model := modelMap[assoc.ModelID]

		if !providerExists {
			shouldDelete = true
		} else {
			providerModels, err := service.GetProviderModels(ctx, *provider)
			if err != nil {
				continue
			}

			modelExists := false
			for _, pm := range providerModels {
				if pm == assoc.ProviderModel {
					modelExists = true
					break
				}
			}

			if !modelExists {
				shouldDelete = true
			}
		}

		if shouldDelete {
			providerName := ""
			if provider != nil {
				providerName = provider.Name
			}
			modelName := ""
			if model != nil {
				modelName = model.Name
			}

			previews = append(previews, AssociationPreview{
				ModelID:       assoc.ModelID,
				ModelName:     modelName,
				ProviderID:    assoc.ProviderID,
				ProviderName:  providerName,
				ProviderModel: assoc.ProviderModel,
			})
		}
	}

	common.Success(c, previews)
}

// AutoAssociateModels 一键添加关联：将提供商模型自动关联到模板匹配的模型（并集规则）
func AutoAssociateModels(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取所有模型
	allModels, err := gorm.G[models.Model](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get models: "+err.Error())
		return
	}

	// 获取所有提供商
	allProviders, err := gorm.G[models.Provider](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get providers: "+err.Error())
		return
	}

	// 获取已存在的关联
	existingAssociations, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get existing associations: "+err.Error())
		return
	}

	manualTemplateItems, err := gorm.G[models.ModelTemplateItem](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get template items: "+err.Error())
		return
	}

	templateIndex := service.BuildTemplateIndexFromData(allModels, existingAssociations, manualTemplateItems)
	modelNameByID := make(map[uint]string, len(allModels))
	for _, m := range allModels {
		modelNameByID[m.ID] = m.Name
	}

	// 构建已存在关联的映射 (model_id + provider_id + provider_model)
	existingMap := make(map[string]bool)
	for _, assoc := range existingAssociations {
		key := fmt.Sprintf("%d_%d_%s", assoc.ModelID, assoc.ProviderID, assoc.ProviderModel)
		existingMap[key] = true
	}

	// 遍历每个提供商，获取其模型列表
	addedCount := 0
	defaultPriority := GetAutoPriorityDecayDefault(ctx)

	for _, provider := range allProviders {
		// 跳过被拉黑的供应商
		if provider.Blacklisted != nil && *provider.Blacklisted {
			continue
		}
		// 解析提供商配置中的模型列表
		providerModels, err := service.GetProviderModels(ctx, provider)
		if err != nil {
			slog.Warn("failed to get provider models", "provider", provider.Name, "error", err)
			continue
		}

		// 遍历提供商的每个模型
		for _, providerModel := range providerModels {
			matchedModelIDs := templateIndex.Match(providerModel)
			for _, modelID := range matchedModelIDs {
				// 检查是否已存在关联
				key := fmt.Sprintf("%d_%d_%s", modelID, provider.ID, providerModel)
				if existingMap[key] {
					continue
				}

				// 创建新关联
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
				if err := gorm.G[models.ModelWithProvider](models.DB).Create(ctx, &newAssoc); err != nil {
					slog.Warn("failed to create association", "model", modelNameByID[modelID], "provider", provider.Name, "error", err)
					continue
				}
				existingMap[key] = true
				addedCount++
			}
		}
	}

	common.Success(c, map[string]interface{}{
		"added": addedCount,
	})
}

// CleanInvalidAssociations 清除无效关联：删除提供商不存在或模型不存在的关联
func CleanInvalidAssociations(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取所有关联
	allAssociations, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get associations: "+err.Error())
		return
	}

	// 获取所有提供商
	allProviders, err := gorm.G[models.Provider](models.DB).Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to get providers: "+err.Error())
		return
	}

	// 构建提供商映射
	providerMap := make(map[uint]*models.Provider)
	for i := range allProviders {
		providerMap[allProviders[i].ID] = &allProviders[i]
	}

	// 检查每个关联
	removedCount := 0
	for _, assoc := range allAssociations {
		shouldDelete := false

		// 检查提供商是否存在
		provider, providerExists := providerMap[assoc.ProviderID]
		if !providerExists {
			shouldDelete = true
		} else {
			// 检查提供商是否还有该模型
			providerModels, err := service.GetProviderModels(ctx, *provider)
			if err != nil {
				slog.Warn("failed to get provider models", "provider_id", provider.ID, "error", err)
				continue
			}

			modelExists := false
			for _, pm := range providerModels {
				if pm == assoc.ProviderModel {
					modelExists = true
					break
				}
			}

			if !modelExists {
				shouldDelete = true
			}
		}

		// 删除无效关联
		if shouldDelete {
			if _, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", assoc.ID).Delete(ctx); err != nil {
				slog.Warn("failed to delete invalid association", "id", assoc.ID, "error", err)
			} else {
				removedCount++
			}
		}
	}

	common.Success(c, map[string]interface{}{
		"removed": removedCount,
	})
}

// triggerAutoAssociate 触发自动关联
func triggerAutoAssociate(ctx context.Context) {
	if !GetSettingBool(ctx, models.SettingKeyAutoAssociateOnAdd) {
		slog.Info("auto-associate disabled")
		return
	}
	slog.Info("auto-associate triggered")

	allModels, err := gorm.G[models.Model](models.DB).Find(ctx)
	if err != nil {
		return
	}

	allProviders, err := gorm.G[models.Provider](models.DB).Find(ctx)
	if err != nil {
		return
	}

	existingAssociations, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	if err != nil {
		return
	}

	manualTemplateItems, err := gorm.G[models.ModelTemplateItem](models.DB).Find(ctx)
	if err != nil {
		return
	}

	existingMap := make(map[string]bool)
	for _, assoc := range existingAssociations {
		key := fmt.Sprintf("%d_%d_%s", assoc.ModelID, assoc.ProviderID, assoc.ProviderModel)
		existingMap[key] = true
	}

	templateIndex := service.BuildTemplateIndexFromData(allModels, existingAssociations, manualTemplateItems)

	defaultPriority := GetAutoPriorityDecayDefault(ctx)
	addedCount := 0

	for _, provider := range allProviders {
		// 跳过被拉黑的供应商
		if provider.Blacklisted != nil && *provider.Blacklisted {
			continue
		}
		providerModels, err := service.GetProviderModels(ctx, provider)
		if err != nil {
			continue
		}

		for _, providerModel := range providerModels {
			matchedModelIDs := templateIndex.Match(providerModel)
			for _, modelID := range matchedModelIDs {
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
				if err := gorm.G[models.ModelWithProvider](models.DB).Create(ctx, &newAssoc); err == nil {
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

// triggerAutoClean 触发自动清理
func triggerAutoClean(ctx context.Context) {
	if !GetSettingBool(ctx, models.SettingKeyAutoCleanOnDelete) {
		slog.Info("auto-clean disabled")
		return
	}
	slog.Info("auto-clean triggered")

	allAssociations, err := gorm.G[models.ModelWithProvider](models.DB).Find(ctx)
	if err != nil {
		return
	}

	allProviders, err := gorm.G[models.Provider](models.DB).Find(ctx)
	if err != nil {
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
			providerModels, err := service.GetProviderModels(ctx, *provider)
			if err != nil {
				continue
			}

			modelExists := false
			for _, pm := range providerModels {
				if pm == assoc.ProviderModel {
					modelExists = true
					break
				}
			}

			if !modelExists {
				shouldDelete = true
			}
		}

		if shouldDelete {
			if _, err := gorm.G[models.ModelWithProvider](models.DB).Where("id = ?", assoc.ID).Delete(ctx); err == nil {
				removedCount++
			}
		}
	}

	if removedCount > 0 {
		slog.Info("auto-cleaned invalid associations", "count", removedCount)
	}
}
