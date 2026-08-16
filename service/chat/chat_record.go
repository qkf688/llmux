package chat

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/chatstats"
)

// RecordLogInput 收敛 RecordLog 的后处理入参，避免签名膨胀成超长参数列表。
// ctx 按 Go 惯例保留在 RecordLog 外层，不进结构体。
//
// SideChannel 是转换层旁路：携带流式响应的原始 SSE 累积体（流结束后已写满）与上游原始 usage。
// 未经协议转换（style == provider type）或未开启记录时为 nil，读取方法均 nil-safe。
type RecordLogInput struct {
	ReqStart     time.Time
	Reader       io.ReadCloser
	Processer    Processer
	LogID        uint
	Before       Before
	IOLog        bool
	ProviderName string
	SideChannel  *models.TransformSideChannel
}

// RecordLog 是后处理编排器：processer 解析 → stats 统计 → 日志落库 → raw 字段清理。
// 各关注点的实现分散在 stats.go / chat_record_persist.go（含 raw 清理），
// 本函数仅负责按正确顺序串联并处理错误传播。
func RecordLog(ctx context.Context, in RecordLogInput) {
	recordFunc := func() error {
		defer in.Reader.Close()

		// 配置开关经 chat_settings getter → 包级 settingsReader（默认 ModelsReader）。
		disablePerformanceTracking := getDisablePerformanceTracking(ctx)
		disableTokenCounting := getDisableTokenCounting(ctx)

		log, output, err := in.Processer(ctx, in.Reader, in.Before.Stream, in.ReqStart, disablePerformanceTracking, disableTokenCounting)
		if err != nil {
			slog.Error("processer error", "log_id", in.LogID, "error", err)
			if in.LogID != 0 {
				if _, updateErr := repos().ChatLog.UpdateByID(ctx, in.LogID, models.ChatLog{
					Status: "error",
					Error:  fmt.Sprintf("processer error: %v", err),
				}); updateErr != nil {
					slog.Error("failed to update log status on processer error", "log_id", in.LogID, "error", updateErr)
				}
			}

			if statErr := chatstats.RecordProviderStats(ctx, in.ProviderName, false, 0, 0); statErr != nil {
				slog.Warn("failed to record provider stats on processer error", "error", statErr)
			}
			return err
		}

		logUpdate := *log

		// usage 归集：优先采用转换层旁路交出的上游原始 usage。
		// 必须在统计之前完成——RecordTokenStats / RecordProviderStats 都读
		// logUpdate.Usage.TotalTokens，晚于它们修正就只修了日志、没修计量。
		resolveUsageSource(&logUpdate, in.SideChannel, in.ProviderName)

		// 统计独立于日志存储：即使关闭日志记录（logID==0），也要写入 tokens 统计。
		if err := chatstats.RecordTokenStats(ctx, in.ReqStart, logUpdate.Usage.TotalTokens); err != nil {
			slog.Warn("failed to record token stats", "error", err)
		}

		responseTimeMs := int64(logUpdate.FirstChunkTime.Milliseconds())
		if statErr := chatstats.RecordProviderStats(ctx, in.ProviderName, true, responseTimeMs, logUpdate.Usage.TotalTokens); statErr != nil {
			slog.Warn("failed to record provider stats", "error", statErr)
		}

		// 若未记录 ChatLog（例如 disable_all_logs=true），这里不再进行任何日志表更新/写入。
		if in.LogID == 0 {
			return nil
		}

		// 读取 raw 日志开关，决定是否拼装响应体与后续清理。
		logRawOptions := getLogRawRequestResponse(ctx)
		logRawErrorsOnly := getLogRawRequestResponseErrorsOnly(ctx)
		rawLogEnabled := logRawOptions.RequestHeaders || logRawOptions.RequestBody || logRawOptions.RawRequestBody || logRawOptions.ResponseHeaders || logRawOptions.ResponseBody || logRawOptions.RawResponseBody

		// 流式响应的原始 body 由转换层旁路提供（非流式已在 chat_attempt 层写入）。
		// goroutine 已结束（processer 读到 EOF = 流结束），此处读取旁路无并发。
		if logRawOptions.RawResponseBody && logUpdate.RawResponseBody == "" {
			if raw := in.SideChannel.RawBody(); raw != "" {
				logUpdate.RawResponseBody = raw
			}
		}

		// IO 落库：更新 ChatLog + 可选写 ChatIO。
		if err := persistChatLog(ctx, in, logUpdate, output, logRawOptions); err != nil {
			return err
		}

		// 成功日志的 raw 字段清理策略（errors-only）。
		maybeClearRawOnSuccess(ctx, in.LogID, logRawErrorsOnly, rawLogEnabled)
		return nil
	}
	if err := recordFunc(); err != nil {
		slog.Error("record log error", "log_id", in.LogID, "error", err)
	}
}

// resolveUsageSource 决定最终写入的 usage 并打上来源标记。
//
// 优先级（可信度递减）：
//  1. 侧信道的上游原始 usage——转换层解析上游响应时直接得到，无协议裁剪。
//  2. processer 的解析结果——直通路径（sideChannel == nil）下它读的就是上游响应，
//     同样可信；走了转换却落到这里，说明它读的是**转换后**的下游流，可能因目标
//     协议缺字段（如 Anthropic 无 reasoning_tokens）而失真。
//
// 两条路都拿不到有效 token 时打 missing 并告警：token 恒为 0 会静默影响计费与
// 配额，必须留下运行时信号而不是让它沉在 DB 里。
func resolveUsageSource(logUpdate *models.ChatLog, sideChannel *models.TransformSideChannel, providerName string) {
	transformed := sideChannel != nil

	if u, ok := sideChannel.UpstreamUsage(); ok {
		logUpdate.Usage = u
		logUpdate.UsageSource = models.UsageSourceUpstream
		return
	}

	if logUpdate.Usage.HasTokens() {
		if transformed {
			logUpdate.UsageSource = models.UsageSourceDownstream
			slog.Warn("usage fell back to downstream stream (transform side channel had none)",
				"provider", providerName,
				"total_tokens", logUpdate.Usage.TotalTokens)
			return
		}
		logUpdate.UsageSource = models.UsageSourcePassthrough
		return
	}

	logUpdate.UsageSource = models.UsageSourceMissing
	slog.Warn("no usage captured for request",
		"provider", providerName,
		"transformed", transformed)
}
