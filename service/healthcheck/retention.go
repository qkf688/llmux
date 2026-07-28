package healthcheck

import (
	"context"
	"log/slog"
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
	deleted, err := repos().HealthCheckLog.EnforceRetention(ctx, retentionCount)
	if err != nil {
		slog.Error("failed to cleanup excess health check logs", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("cleaned up excess health check logs", "deleted", deleted, "retention", retentionCount)
	}
}
