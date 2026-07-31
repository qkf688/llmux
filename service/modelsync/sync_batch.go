package modelsync

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// SyncAllProviders 同步所有启用模型端点的提供商。
func (s *Service) SyncAllProviders(ctx context.Context) ([]*models.ModelSyncLog, error) {
	batchID := fmt.Sprintf("%d", time.Now().Unix())

	var providers []models.Provider
	if err := repository.WhereModelEndpointEnabled(s.db.WithContext(ctx)).
		Find(&providers).Error; err != nil {
		return nil, err
	}

	logs := make([]*models.ModelSyncLog, 0, len(providers))
	for _, provider := range providers {
		log, err := s.SyncProviderModels(ctx, provider.ID)
		if err != nil {
			slog.Error("failed to sync provider", "provider_id", provider.ID, "error", err)
			continue
		}
		if log == nil {
			continue
		}

		log.BatchID = &batchID
		if err := s.db.Model(log).Update("batch_id", batchID).Error; err != nil {
			slog.Error("failed to update batch_id", "log_id", log.ID, "error", err)
			continue
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// GetRecentAddedModels 获取最近一次同步批次新增的模型。
func (s *Service) GetRecentAddedModels(ctx context.Context) (*RecentAddedModelsResponse, error) {
	var latestLog models.ModelSyncLog
	err := s.db.WithContext(ctx).
		Where("batch_id IS NOT NULL AND batch_id != ?", "").
		Order("batch_id DESC").
		First(&latestLog).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &RecentAddedModelsResponse{Data: []AddedModel{}, TotalCount: 0}, nil
		}
		return nil, err
	}

	var logs []models.ModelSyncLog
	err = s.db.WithContext(ctx).
		Where("batch_id = ? AND status = ?", latestLog.BatchID, "success").
		Where("added_count > 0").
		Find(&logs).Error
	if err != nil {
		return nil, err
	}

	result := make([]AddedModel, 0)
	for _, log := range logs {
		for _, modelName := range log.AddedModels {
			result = append(result, AddedModel{
				ModelName:    modelName,
				ProviderName: log.ProviderName,
				AddedAt:      log.SyncedAt,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].AddedAt.After(result[j].AddedAt)
	})

	return &RecentAddedModelsResponse{
		Data:       result,
		SyncTime:   &latestLog.SyncedAt,
		TotalCount: len(result),
	}, nil
}
