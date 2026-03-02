package chat

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func SaveChatLog(ctx context.Context, log models.ChatLog) (uint, error) {
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
	if err := models.DB.Model(&models.ChatLog{}).Count(&total).Error; err != nil {
		slog.Error("failed to count logs for cleanup", "error", err)
		return
	}

	// 如果日志数超过保留条数，删除多余的
	if int(total) > retentionCount {
		deleteCount := int(total) - retentionCount

		// 获取需要删除的日志ID（最旧的）
		var logsToDelete []models.ChatLog
		if err := models.DB.Model(&models.ChatLog{}).
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

		// 删除对应的ChatIO记录
		if _, err := gorm.G[models.ChatIO](models.DB).
			Where("log_id IN ?", ids).
			Delete(ctx); err != nil {
			slog.Error("failed to delete chat io records", "error", err)
		}

		// 删除日志记录
		if _, err := gorm.G[models.ChatLog](models.DB).
			Where("id IN ?", ids).
			Delete(ctx); err != nil {
			slog.Error("failed to delete logs", "error", err)
			return
		}

		slog.Info("auto cleaned up excess logs", "deleted", deleteCount, "retention", retentionCount)
	}
}

// getLogRetentionCount 获取日志保留条数设置
func getLogRetentionCount(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyLogRetentionCount).
		First(ctx)
	if err != nil {
		return 0 // 默认不限制
	}
	count, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 0
	}
	return count
}
