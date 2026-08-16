package modelsync

import (
	"context"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"gorm.io/gorm"
)

// StartAutoSync 启动自动同步定时任务。ctx 为进程级 ctx，其取消即本循环的退出信号。
func (s *Service) StartAutoSync(ctx context.Context) {
	// 经 bgtask 登记：何时停仍由传入的 ctx 决定，Manager 只保证关闭时等到它停干净，
	// 不会在一轮同步写库中途被硬切。
	bgtask.Go(func(context.Context) {
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
	})
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
