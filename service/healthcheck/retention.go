package healthcheck

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
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
	// HealthCheck：普通 Count + 软删（禁止 Unscoped 硬删）
	deleted, err := models.EnforceRetentionByOldestID(ctx, models.DB, &models.HealthCheckLog{}, retentionCount, models.RetentionDeleteOptions{
		UnscopedCount:  false,
		UnscopedDelete: false,
	})
	if err != nil {
		slog.Error("failed to cleanup excess health check logs", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("cleaned up excess health check logs", "deleted", deleted, "retention", retentionCount)
	}
}