package chat

import (
	"context"
	"log/slog"
	"time"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func SaveChatLog(ctx context.Context, log models.ChatLog) (uint, error) {
	// 统计应独立于日志存储：即使关闭日志记录，也要更新仪表盘统计。
	if err := recordRequestStats(ctx, time.Now(), log.Name); err != nil {
		slog.Warn("failed to record request stats", "error", err)
	}
	// 真实模型统计：优先用 RealModelName，异常路径回退到 Name。
	realModelName := log.RealModelName
	if realModelName == "" {
		realModelName = log.Name
	}
	if err := recordRealModelRequestStats(ctx, time.Now(), realModelName); err != nil {
		slog.Warn("failed to record real model request stats", "error", err)
	}

	// 检查是否完全关闭日志记录
	if getDisableAllLogs(ctx) {
		return 0, nil // 返回0表示不记录日志
	}

	if err := gorm.G[models.ChatLog](models.DB).Create(ctx, &log); err != nil {
		return 0, err
	}
	// 异步执行日志清理，避免阻塞主流程
	go cleanupLogsIfNeeded()
	return log.ID, nil
}

// cleanupLogsIfNeeded 检查并清理超出保留条数的日志
func cleanupLogsIfNeeded() {
	ctx := context.Background()

	// 获取日志保留条数设置
	retentionCount := getLogRetentionCount(ctx)
	if retentionCount <= 0 {
		return // 0 表示不限制
	}

	// ChatLog：Unscoped 统计+硬删，先删 ChatIO（与 healthcheck 软删策略不同）
	deleted, err := models.EnforceRetentionByOldestID(ctx, models.DB, &models.ChatLog{}, retentionCount, models.RetentionDeleteOptions{
		UnscopedCount:  true,
		UnscopedDelete: true,
		BeforeDelete: func(ctx context.Context, ids []uint) error {
			if err := models.DB.WithContext(ctx).Unscoped().
				Where("log_id IN ?", ids).
				Delete(&models.ChatIO{}).Error; err != nil {
				slog.Error("failed to delete chat io records", "error", err)
				// 与旧逻辑一致：ChatIO 失败只记日志，不中断主表删除
			}
			return nil
		},
	})
	if err != nil {
		slog.Error("failed to cleanup excess logs", "error", err)
		return
	}
	if deleted > 0 {
		slog.Info("auto cleaned up excess logs", "deleted", deleted, "retention", retentionCount)
	}
}

// getLogRetentionCount 获取日志保留条数设置
func getLogRetentionCount(ctx context.Context) int {
	return settingsReader.Int(ctx, models.SettingKeyLogRetentionCount, 0, 0)
}
