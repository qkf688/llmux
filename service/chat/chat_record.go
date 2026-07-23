package chat

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/chatstats"
	"gorm.io/gorm"
)

// RecordLog 是后处理编排器：processer 解析 → stats 统计 → 日志落库 → raw 字段清理。
// 各关注点的实现分散在 stats.go / chat_record_persist.go / chat_record_raw.go，
// 本函数仅负责按正确顺序串联并处理错误传播。
func RecordLog(ctx context.Context, reqStart time.Time, reader io.ReadCloser, processer Processer, logId uint, before Before, ioLog bool, providerName string) {
	recordFunc := func() error {
		defer reader.Close()

		// 配置开关经 chat_settings getter → 包级 settingsReader（默认 ModelsReader）。
		disablePerformanceTracking := getDisablePerformanceTracking(ctx)
		disableTokenCounting := getDisableTokenCounting(ctx)

		log, output, err := processer(ctx, reader, before.Stream, reqStart, disablePerformanceTracking, disableTokenCounting)
		if err != nil {
			slog.Error("processer error", "log_id", logId, "error", err)
			if logId != 0 {
				if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, models.ChatLog{
					Status: "error",
					Error:  fmt.Sprintf("processer error: %v", err),
				}); updateErr != nil {
					slog.Error("failed to update log status on processer error", "log_id", logId, "error", updateErr)
				}
			}

			if statErr := chatstats.RecordProviderStats(ctx, providerName, false, 0, 0); statErr != nil {
				slog.Warn("failed to record provider stats on processer error", "error", statErr)
			}
			return err
		}

		logUpdate := *log

		// 统计独立于日志存储：即使关闭日志记录（logId==0），也要写入 tokens 统计。
		if err := chatstats.RecordTokenStats(ctx, reqStart, logUpdate.Usage.TotalTokens); err != nil {
			slog.Warn("failed to record token stats", "error", err)
		}

		responseTimeMs := int64(logUpdate.FirstChunkTime.Milliseconds())
		if statErr := chatstats.RecordProviderStats(ctx, providerName, true, responseTimeMs, logUpdate.Usage.TotalTokens); statErr != nil {
			slog.Warn("failed to record provider stats", "error", statErr)
		}

		// 若未记录 ChatLog（例如 disable_all_logs=true），这里不再进行任何日志表更新/写入。
		if logId == 0 {
			return nil
		}

		// 读取 raw 日志开关，决定是否拼装响应体与后续清理。
		logRawOptions := getLogRawRequestResponse(ctx)
		logRawErrorsOnly := getLogRawRequestResponseErrorsOnly(ctx)
		rawLogEnabled := logRawOptions.RequestHeaders || logRawOptions.RequestBody || logRawOptions.ResponseHeaders || logRawOptions.ResponseBody || logRawOptions.RawResponseBody

		// IO 落库：更新 ChatLog + 可选写 ChatIO。
		if err := persistChatLog(ctx, logId, logUpdate, before, output, ioLog, logRawOptions); err != nil {
			return err
		}

		// 成功日志的 raw 字段清理策略（errors-only）。
		maybeClearRawOnSuccess(ctx, logId, logRawErrorsOnly, rawLogEnabled)
		return nil
	}
	if err := recordFunc(); err != nil {
		slog.Error("record log error", "log_id", logId, "error", err)
	}
}
