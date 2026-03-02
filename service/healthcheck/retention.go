package healthcheck

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// EnforceHealthCheckLogRetention 清理超出保留条数的健康检测日志。
func EnforceHealthCheckLogRetention(ctx context.Context) {
	retention := GetHealthChecker().getLogRetentionCount(ctx)
	if retention <= 0 {
		return
	}
	cleanupHealthCheckLogs(ctx, retention)
}

func cleanupHealthCheckLogs(ctx context.Context, retentionCount int) {
	var total int64
	if err := models.DB.WithContext(ctx).Model(&models.HealthCheckLog{}).Count(&total).Error; err != nil {
		slog.Error("failed to count health check logs for cleanup", "error", err)
		return
	}
	if int(total) <= retentionCount {
		return
	}

	deleteCount := int(total) - retentionCount
	var ids []uint
	if err := models.DB.WithContext(ctx).
		Model(&models.HealthCheckLog{}).
		Order("id ASC").
		Limit(deleteCount).
		Pluck("id", &ids).Error; err != nil {
		slog.Error("failed to find health check logs to delete", "error", err)
		return
	}
	if len(ids) == 0 {
		return
	}

	if _, err := gorm.G[models.HealthCheckLog](models.DB).Where("id IN ?", ids).Delete(ctx); err != nil {
		slog.Error("failed to delete health check logs", "error", err)
		return
	}

	slog.Info("cleaned up excess health check logs", "deleted", len(ids), "retention", retentionCount)
}
