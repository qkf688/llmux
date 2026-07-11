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

	// 获取总日志数
	var total int64
	// 注意：ChatLog 使用 gorm.Model（包含 DeletedAt）。这里必须使用 Unscoped 统计，确保历史软删记录也会被真正清理，
	// 否则 UI 看起来只保留了 retentionCount 条，但数据库内仍会堆积软删记录，导致“清空日志”时 deleted 远大于 retentionCount。
	if err := models.DB.Unscoped().Model(&models.ChatLog{}).Count(&total).Error; err != nil {
		slog.Error("failed to count logs for cleanup", "error", err)
		return
	}

	// 如果日志数超过保留条数，删除多余的
	if int(total) > retentionCount {
		deleteCount := int(total) - retentionCount

		// 获取需要删除的日志ID（最旧的）
		var logsToDelete []models.ChatLog
		if err := models.DB.Unscoped().Model(&models.ChatLog{}).
			Order("id ASC").
			Limit(deleteCount).
			Find(&logsToDelete).Error; err != nil {
			slog.Error("failed to find logs to delete", "error", err)
			return
		}

		// 提取ID列表
		ids := make([]uint, len(logsToDelete))
		for i, log := range logsToDelete {
			ids[i] = log.ID
		}

		// 删除对应的ChatIO记录（硬删）
		if err := models.DB.WithContext(ctx).Unscoped().
			Where("log_id IN ?", ids).
			Delete(&models.ChatIO{}).Error; err != nil {
			slog.Error("failed to delete chat io records", "error", err)
		}

		// 删除日志记录（硬删）
		if err := models.DB.WithContext(ctx).Unscoped().
			Where("id IN ?", ids).
			Delete(&models.ChatLog{}).Error; err != nil {
			slog.Error("failed to delete logs", "error", err)
			return
		}

		slog.Info("auto cleaned up excess logs", "deleted", deleteCount, "retention", retentionCount)
	}
}

// getLogRetentionCount 获取日志保留条数设置
func getLogRetentionCount(ctx context.Context) int {
	return models.GetSettingInt(ctx, models.SettingKeyLogRetentionCount, 0, 0)
}
