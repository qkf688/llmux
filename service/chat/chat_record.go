package chat

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func RecordLog(ctx context.Context, reqStart time.Time, reader io.ReadCloser, processer Processer, logId uint, before Before, ioLog bool) {
	recordFunc := func() error {
		defer reader.Close()

		// 获取性能追踪和 token 统计的开关设置
		disablePerformanceTracking := getDisablePerformanceTracking(ctx)
		disableTokenCounting := getDisableTokenCounting(ctx)

		log, output, err := processer(ctx, reader, before.Stream, reqStart, disablePerformanceTracking, disableTokenCounting)
		if err != nil {
			slog.Error("processer error", "log_id", logId, "error", err)
			// 更新日志状态为错误
			if logId != 0 {
				if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, models.ChatLog{
					Status: "error",
					Error:  fmt.Sprintf("processer error: %v", err),
				}); updateErr != nil {
					slog.Error("failed to update log status on processer error", "log_id", logId, "error", updateErr)
				}
			}
			return err
		}

		// 更新日志记录
		logUpdate := *log

		// 统计应独立于日志存储：即使关闭日志记录（logId==0），也要写入 tokens 统计。
		if err := recordTokenStats(ctx, reqStart, logUpdate.Usage.TotalTokens); err != nil {
			slog.Warn("failed to record token stats", "error", err)
		}

		// 若未记录 ChatLog（例如 disable_all_logs=true），这里不再进行任何日志表更新/写入。
		if logId == 0 {
			return nil
		}

		// 检查是否启用原始请求响应记录
		logRawOptions := getLogRawRequestResponse(ctx)
		logRawErrorsOnly := getLogRawRequestResponseErrorsOnly(ctx)
		rawLogEnabled := logRawOptions.RequestHeaders || logRawOptions.RequestBody || logRawOptions.ResponseHeaders || logRawOptions.ResponseBody || logRawOptions.RawResponseBody
		if logRawOptions.ResponseBody {
			// 记录完整的响应体内容
			var responseBodyStr string
			if output != nil {
				if output.OfString != "" {
					responseBodyStr = output.OfString
				} else if len(output.OfStringArray) > 0 {
					// 对于流式响应，记录所有chunk
					for _, chunk := range output.OfStringArray {
						responseBodyStr += chunk + "\n"
					}
				}
			}

			logUpdate.ResponseBody = responseBodyStr
		}

		if _, err := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, logUpdate); err != nil {
			slog.Error("failed to update log", "log_id", logId, "error", err)
			return err
		}

		// 只有在启用 IO 日志时才记录输入输出
		if ioLog {
			if err := gorm.G[models.ChatIO](models.DB).Create(ctx, &models.ChatIO{
				Input:       string(before.raw),
				LogId:       logId,
				OutputUnion: *output,
			}); err != nil {
				slog.Error("failed to create chat io", "log_id", logId, "error", err)
				return err
			}
		}

		// 若开启“仅保留错误日志原始请求响应”，则在成功日志写入完成后清空原始字段，避免成功日志长期占用存储。
		if logRawErrorsOnly && rawLogEnabled {
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
		return nil
	}
	if err := recordFunc(); err != nil {
		slog.Error("record log error", "log_id", logId, "error", err)
	}
}

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
