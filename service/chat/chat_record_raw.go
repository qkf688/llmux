package chat

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/models"
)

// maybeClearRawOnSuccess 实现「仅保留错误日志的原始请求响应」策略：
// 若 errorsOnly=true 且 rawLogEnabled，则查询该日志当前状态；
// 状态非 error（即成功）时清空 raw 字段，避免成功日志长期占用存储。
// 错误状态下保留 raw 字段以便排查。
func maybeClearRawOnSuccess(ctx context.Context, logId uint, errorsOnly bool, rawLogEnabled bool) {
	if !errorsOnly || !rawLogEnabled || logId == 0 {
		return
	}

	var current models.ChatLog
	if err := models.DB.WithContext(ctx).
		Model(&models.ChatLog{}).
		Select("status").
		Where("id = ?", logId).
		Take(&current).Error; err == nil && current.Status != "error" {
		if err := clearChatLogRawRequestResponseFields(ctx, logId); err != nil {
			slog.Error("failed to clear raw request/response fields", "log_id", logId, "error", err)
		}
	}
}

// clearChatLogRawRequestResponseFields 将指定日志的 5 个 raw 字段置空。
func clearChatLogRawRequestResponseFields(ctx context.Context, logID uint) error {
	return models.DB.WithContext(ctx).
		Model(&models.ChatLog{}).
		Where("id = ?", logID).
		Updates(map[string]interface{}{
			"request_headers":   "",
			"request_body":      "",
			"response_headers":  "",
			"response_body":     "",
			"raw_response_body": "",
		}).Error
}
