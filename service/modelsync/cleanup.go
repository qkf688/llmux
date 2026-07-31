package modelsync

import (
	"context"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func (s *Service) cleanOldLogs(ctx context.Context) {
	retentionCount, err := s.getSettingInt(ctx, models.SettingKeyModelSyncLogRetentionCount)
	if err != nil {
		retentionCount = 100
	}

	retentionDays, err := s.getSettingInt(ctx, models.SettingKeyModelSyncLogRetentionDays)
	if err != nil {
		retentionDays = 7
	}

	if retentionCount > 0 {
		var count int64
		s.db.WithContext(ctx).Model(&models.ModelSyncLog{}).Count(&count)
		if count > int64(retentionCount) {
			toDelete := count - int64(retentionCount)
			if _, err := gorm.G[models.ModelSyncLog](s.db).
				Order("synced_at ASC").
				Limit(int(toDelete)).
				Delete(ctx); err != nil {
				slog.Error("failed to clean old logs by count", "error", err)
			}
		}
	}

	if retentionDays > 0 {
		cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
		if _, err := gorm.G[models.ModelSyncLog](s.db).
			Where("synced_at < ?", cutoffTime).
			Delete(ctx); err != nil {
			slog.Error("failed to clean old logs by days", "error", err)
		}
	}
}
