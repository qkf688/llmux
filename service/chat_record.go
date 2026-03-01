package service

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
			if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, models.ChatLog{
				Status: "error",
				Error:  fmt.Sprintf("processer error: %v", err),
			}); updateErr != nil {
				slog.Error("failed to update log status on processer error", "log_id", logId, "error", updateErr)
			}
			return err
		}

		// 更新日志记录
		logUpdate := *log

		// 检查是否启用原始请求响应记录
		logRawOptions := getLogRawRequestResponse(ctx)
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
		return nil
	}
	if err := recordFunc(); err != nil {
		slog.Error("record log error", "log_id", logId, "error", err)
	}
}
