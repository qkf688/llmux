package modelsync

import (
	"context"
	"log/slog"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"gorm.io/gorm"
)

// SyncProviderModels 同步单个提供商的上游模型。
// 返回日志记录（无论是否有变化），以及可能的错误。
func (s *Service) SyncProviderModels(ctx context.Context, providerID uint) (*models.ModelSyncLog, error) {
	provider, err := gorm.G[models.Provider](s.db).Where("id = ?", providerID).First(ctx)
	if err != nil {
		slog.Error("failed to get provider", "provider_id", providerID, "error", err)
		return nil, err
	}

	// nil 视为开启。
	if provider.ModelEndpoint != nil && !*provider.ModelEndpoint {
		return s.createAndSaveErrorLog(ctx, provider, "model_endpoint disabled"), nil
	}

	currentModels := extractUpstreamModels(provider.Config)
	config := provider.Config
	if cleanedConfig, err := dropCustomModels(config); err == nil {
		config = cleanedConfig
	}

	chatModel, err := providers.New(provider.Type, config, provider.Proxy)
	if err != nil {
		slog.Error("failed to create provider client", "provider_id", providerID, "error", err)
		return s.createAndSaveErrorLog(ctx, provider, err.Error()), nil
	}

	upstreamModels, err := chatModel.Models(ctx)
	if err != nil {
		slog.Error("failed to fetch upstream models", "provider_id", providerID, "error", err)
		return s.createAndSaveErrorLog(ctx, provider, err.Error()), nil
	}

	if provider.ModelFilterEnabled != nil && *provider.ModelFilterEnabled {
		filteredModels, err := s.filterModelsByRules(ctx, upstreamModels)
		if err != nil {
			slog.Warn("failed to filter models by rules", "provider_id", providerID, "error", err)
		} else {
			upstreamModels = filteredModels
		}
	}

	addedModels, removedModels := diffUpstreamModels(currentModels, upstreamModels)
	hasChanges := len(addedModels) > 0 || len(removedModels) > 0
	status := "success"
	if !hasChanges {
		status = "unchanged"
	}

	if hasChanges {
		updatedModels := collectUpstreamModelIDs(upstreamModels)
		newConfig := buildConfigWithAllModels(provider.Config, updatedModels)
		if _, err := gorm.G[models.Provider](s.db).Where("id = ?", providerID).Update(ctx, "config", newConfig); err != nil {
			slog.Error("failed to update provider config", "provider_id", providerID, "error", err)
		}
	}

	syncLog := &models.ModelSyncLog{
		ProviderID:    providerID,
		ProviderName:  provider.Name,
		Status:        status,
		AddedCount:    len(addedModels),
		RemovedCount:  len(removedModels),
		AddedModels:   addedModels,
		RemovedModels: removedModels,
		SyncedAt:      time.Now(),
	}

	if err := gorm.G[models.ModelSyncLog](s.db).Create(ctx, syncLog); err != nil {
		slog.Error("failed to create sync log", "provider_id", providerID, "error", err)
		return nil, err
	}

	if hasChanges {
		s.triggerAutoActions(ctx, len(addedModels) > 0, len(removedModels) > 0)
	}

	s.cleanOldLogs(ctx)

	return syncLog, nil
}

func (s *Service) createAndSaveErrorLog(ctx context.Context, provider models.Provider, message string) *models.ModelSyncLog {
	syncLog := &models.ModelSyncLog{
		ProviderID:    provider.ID,
		ProviderName:  provider.Name,
		Status:        "error",
		Error:         message,
		AddedCount:    0,
		RemovedCount:  0,
		AddedModels:   []string{},
		RemovedModels: []string{},
		SyncedAt:      time.Now(),
	}
	if err := gorm.G[models.ModelSyncLog](s.db).Create(ctx, syncLog); err != nil {
		slog.Error("failed to create sync log", "provider_id", provider.ID, "error", err)
	}
	return syncLog
}

func collectUpstreamModelIDs(upstreamModels []providers.Model) []string {
	modelIDs := make([]string, 0, len(upstreamModels))
	for _, model := range upstreamModels {
		modelIDs = append(modelIDs, model.ID)
	}
	return modelIDs
}

func diffUpstreamModels(currentModels []string, upstreamModels []providers.Model) ([]string, []string) {
	upstreamModelSet := make(map[string]struct{}, len(upstreamModels))
	for _, model := range upstreamModels {
		upstreamModelSet[model.ID] = struct{}{}
	}

	currentModelSet := make(map[string]struct{}, len(currentModels))
	for _, modelID := range currentModels {
		currentModelSet[modelID] = struct{}{}
	}

	addedModels := make([]string, 0, len(upstreamModels))
	for _, model := range upstreamModels {
		if _, exists := currentModelSet[model.ID]; !exists {
			addedModels = append(addedModels, model.ID)
		}
	}

	removedModels := make([]string, 0, len(currentModels))
	for _, modelID := range currentModels {
		if _, exists := upstreamModelSet[modelID]; !exists {
			removedModels = append(removedModels, modelID)
		}
	}

	return addedModels, removedModels
}
