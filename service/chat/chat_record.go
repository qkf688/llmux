package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
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
	// ReqStart 请求级开始时刻（handler/v1/chat.go 的 startReq），**必须非零**：
	// ProxyTime / FirstChunkTime 都以它零点；传零值时 time.Since 会溢出成巨大的
	// 负 Duration（非零，能绕过 UpdateByID 的零值跳过）直接落库。
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
			// 失败留痕归口外层收口恰一条（Stage 18 口径）：此处禁自记——内层+外层双份
			// 会把一次故障算成两次；细节经错误链进外层留痕、并落 chat_logs Error 字段。
			// 失败出口回写经单源收口（Stage 17 回批）：wrapper 内部派生不陪葬请求
			// ctx 的写库 ctx 并短路 logID==0——RecordLog 无论被传什么 ctx，此行
			// 都落库（此前直调靠调用方恰好传 Background 保平安，无防线）。
			// ProxyTime 回填语义不变：错误日志同样反映「请求进入网关到失败判定」
			// 的真实时长（struct Updates 跳零值，毫秒级真实耗时非零可安全写入）。
			updateChatLogByID(ctx, in.LogID, models.ChatLog{
				Status:    "error",
				Error:     fmt.Sprintf("processer error: %v", err),
				ProxyTime: time.Since(in.ReqStart),
			}, "failed to update log status on processer error")

			if statErr := chatstats.RecordProviderStats(ctx, in.ProviderName, false, 0, 0); statErr != nil {
				slog.Warn("failed to record provider stats on processer error", "error", statErr)
			}
			return fmt.Errorf("processer error: %w", err)
		}

		logUpdate := *log

		// 回填端到端「代理耗时」：建行时的 ProxyTime 快照取在 `Client.Do` 之前
		// （见 executeSingleProviderAttempt），不包含任何上游耗时。此处 processer
		// 已读到响应 EOF（流式响应读完 / 非流式读完），是成功路径最晚、最完整的
		// 终值时刻；与 FirstChunkTime 同以 ReqStart（handler startReq）为零点。
		logUpdate.ProxyTime = time.Since(in.ReqStart)

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
		// 本次后处理失败的唯一一条留痕（Stage 18 口径）：内层不自记，细节在错误链
		// 与 chat_logs 的 Error 字段。客户端断连类降 Warn（framework-conventions §5
		// 级别约定）：断连不是服务器故障，按 ERROR 计数的告警会被「建连即断」刷爆。
		if isClientDisconnectErr(err) {
			slog.Warn("record log error", "log_id", in.LogID, "error", err)
		} else {
			slog.Error("record log error", "log_id", in.LogID, "error", err)
		}
	}
}

// MarkLogUnfinished 给「日志后处理任务被关机丢弃」的行写「未能写完」失败留痕
// （Stage 35 丢弃窗口）。丢弃时日志正文随任务一起丢是共识代价，但行不能停在
// 建行占位的 success——那会让「这行日志没记全」伪装成一条成功记录。
//
// 走 updateChatLogByID 单源收口（detached ctx）：丢弃只发生在关机收尾段，请求
// ctx 与 bgtask 都在关，不陪葬的写才落得下去。刻意不走 RecordLog 的错误分支——
// 那条路会连带写 ProviderStats 失败计数，而这个请求本身是成功的（响应已交付，
// 丢的只是日志正文），误记失败会冤枉供应商。
func MarkLogUnfinished(ctx context.Context, logID uint, reqStart time.Time) {
	updateChatLogByID(ctx, logID, models.ChatLog{
		Status:    "error",
		Error:     "log record task discarded at shutdown: response body not recorded",
		ProxyTime: time.Since(reqStart),
	}, "failed to mark log unfinished")
}

// isClientDisconnectErr 判定客户端断连类失败（坏管道 / 连接重置 / 请求取消）。
// 只认 errors.Is/As 语义匹配：*net.Error 覆盖 Windows WSAECONNRESET 等平台差异，
// 不做错误文本匹配。
// 超时**不是**断连（Stage 33）：deadline / 网络超时归「服务端或上游掐断」，按断连
// 降级会把超时吞成「调用方撤走」的同形留痕——net.Error 家族里 context.DeadlineExceeded
// 与 http 客户端超时都带 Timeout()=true，必须先于断连判定排除。
func isClientDisconnectErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return !netErr.Timeout()
	}
	return false
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
