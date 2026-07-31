package modelsync

import (
	"context"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// StartAutoSync 启动自动同步定时任务。
func (s *Service) StartAutoSync(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.checkAndSync(ctx)
			}
		}
	}()
}

func (s *Service) checkAndSync(ctx context.Context) {
	enabled, err := s.getSettingBool(ctx, models.SettingKeyModelSyncEnabled)
	if err != nil || !enabled {
		return
	}

	interval, err := s.getSettingInt(ctx, models.SettingKeyModelSyncInterval)
	if err != nil {
		interval = 12
	}

	var lastLog models.ModelSyncLog
	enabledProviderIDs := repository.WhereModelEndpointEnabled(
		s.db.WithContext(ctx).Model(&models.Provider{}).Select("id"),
	)

	if err := s.db.WithContext(ctx).
		Where("provider_id IN (?)", enabledProviderIDs).
		Order("synced_at DESC").
		First(&lastLog).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			slog.Error("failed to get last sync log", "error", err)
			return
		}
		_, _ = s.SyncAllProviders(ctx)
		return
	}

	if time.Since(lastLog.SyncedAt) >= time.Duration(interval)*time.Hour {
		_, _ = s.SyncAllProviders(ctx)
	}
}
