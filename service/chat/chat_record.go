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

	// ServingModelWithProviderID 本次响应**实际由哪个关联提供**（含故障转移后的赢家）；
	// 0 = 无可比对对象。提交后失败发生在响应已交付之后、下游只能经日志管道感知，日志侧
	// 据此判断「这段对话的钉定是否还指着刚失败的这家，要清掉它」——本字段与表内当前钉定
	// 不一致时（并发里另一次请求刚改过钉）不动它，见 clearStickyTarget / balancer.DeleteIf。
	ServingModelWithProviderID uint

	// RealModelName 本次真实命中的模型名，与建行时刻写入日志的同源（虚拟路径下
	// 是落地名，直连时为空 → 由 Before.Model 兜底）。token 统计键靠它，不能从
	// processer 的返回值取——那边只填了耗时与 usage，模型名不在其中。
	RealModelName string
}

// RecordLog 是后处理编排器：processer 解析 → stats 统计 → 日志落库 → raw 字段清理。
// 各关注点的实现分散在 stats.go / chat_record_persist.go（含 raw 清理），
// 本函数仅负责按正确顺序串联并处理错误传播。
func RecordLog(ctx context.Context, in RecordLogInput) {
	recordFunc := func() error {
		defer in.Reader.Close()

		log, output, err := in.Processer(ctx, in.Reader, in.Before.Stream, in.ReqStart)
		if err != nil {
			// 失败留痕归口外层收口恰一条（Stage 18 口径）：此处禁自记——内层+外层双份
			// 会把一次故障算成两次；细节经错误链进外层留痕、并落 chat_logs Error 字段。
			// 失败出口回写经单源收口（Stage 17 回批）：wrapper 内部派生不陪葬请求
			// ctx 的写库 ctx 并短路 logID==0——RecordLog 无论被传什么 ctx，此行
			// 都落库（此前直调靠调用方恰好传 Background 保平安，无防线）。
			// ProxyTime 回填语义不变：错误日志同样反映「请求进入网关到失败判定」
			// 的真实时长（struct Updates 跳零值，毫秒级真实耗时非零可安全写入）。
			updateChatLogByID(ctx, in.LogID, models.ChatLog{
				Status:    failureLogStatus(err),
				Error:     fmt.Sprintf("processer error: %v", err),
				ProxyTime: time.Since(in.ReqStart),
			}, "failed to update log status on processer error")

			// 提交后失败清钉（2026-10-01）：本分支是**响应已开写之后**所有后处理失败的
			// 唯一收口——客户端已拿到 200 与部分内容，无法换家重来，故当「钉定仍指着
			// 刚服务完的那个关联」时清掉它，让下一轮回到常规随机选择。
			//
			// 范围不止「上游腰斩」：流中吐 error 帧、SSE 扫描错、usage 字段吐了合法
			// JSON 但非对象（process.go 的三条上抛路径）都汇进本分支。它们同属供应商侧
			// 缺陷，清钉对它们同样成立。
			//
			// 两条护栏，缺一不可：① ServingModelWithProviderID 与表内当前钉定一致（比对在
			// balancer.DeleteIf，故并发里另一次请求刚改过钉时不动它）② 失败不是客户端撤走
			// ——供应商没做错就别动它的记忆。② 分两路：IsUpstreamReadErr 是**错误产生处
			// 记下的方向事实**，优先于 isClientDisconnectErr 的类型反推（后者把上游 RST
			// 这类非超时 net.Error 误判成客户端断连，真机实测过）；其余走后者，把
			// context.Canceled / 坏管道 / 非超时 net.Error 归给客户端、把上游截断与上游
			// 超时归给供应商（超时=供应商慢，与 Stage 24 结算口径一致）。
			//
			// 这些类非超时 net.Error 与**写回被写上限掐断**（Stage 39，客户端不收结果；其
			// 错误形状与上游超时同形、类型层分不开）同属客户端侧，清钉对它们不成立。
			// 提交前失败会换家重试并在成功后改钉，本来就自愈；502（多家全挂）在 handler
			// 提前返回、根本不进本函数，结构上到不了这里。
			//
			// 刻意只挂本分支、绝不挂下方外层收口：外层还兜着 persistChatLog 失败，
			// 那不是本次尝试的失败，挂那里会让「日志写库一失败就抹掉好端端的钉定」。
			if in.ServingModelWithProviderID != 0 && (IsUpstreamReadErr(err) || !clientSideFailure(err)) {
				clearStickyTarget(ctx, in.Before, in.ServingModelWithProviderID)
			}

			// 提交后失败的归属单源（Stage 44 拍板，2026-10-16）：**调用方撤走**不记供应商账，
			// 其余记。统计面与调度面共用它——此前统计面只排除了「写回被掐」一类、调度面则
			// 完全不记，于是同一个「客户端点停止」按错误走的代码路径分到两种归属，而稳定
			// 腰斩的供应商永不被降权。
			if providerAtFault(err) {
				if statErr := chatstats.RecordProviderStats(ctx, in.ProviderName, false, 0, 0); statErr != nil {
					slog.Warn("failed to record provider stats on processer error", "error", statErr)
				}
				settlePostCommitFailure(ctx, in)
			}
			return fmt.Errorf("processer error: %w", err)
		}

		logUpdate := *log

		// 成功出口**显式**写终态（状态机）：建行值是 processing，而 struct Updates
		// 跳零值——processer 返回值不含 Status（空串）时上面这行改动会被静默丢弃，
		// 行就永远停在 processing、看板把它算成「进行中」。同时它让下方
		// maybeClearRawOnSuccess 能按「是不是 success」判定，而不是按「是不是 error」。
		logUpdate.Status = models.ChatLogStatusSuccess

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

		// 真实模型维度的 token：与上面的时间维度 token 同一个 usage 快照。
		// 必须排在 resolveUsageSource 之后（否则用的是修正前的 token），
		// 也必须在 LogID==0 的早退之前（统计不随关日志关闭）。
		// 统计键取自入参而非 processer 返回值：后者只填了耗时与 usage。
		if err := chatstats.RecordRealModelTokenStats(
			ctx,
			realModelStatKeyOf(in.RealModelName, in.Before.Model),
			logUpdate.Usage.PromptTokens,
			logUpdate.Usage.CompletionTokens,
			logUpdate.Usage.TotalTokens,
		); err != nil {
			slog.Warn("failed to record real model token stats", "error", err)
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
		if clientSideFailure(err) {
			slog.Warn("record log error", "log_id", in.LogID, "error", err)
		} else {
			slog.Error("record log error", "log_id", in.LogID, "error", err)
		}
	}
}

// MarkLogUnfinished 给「日志后处理任务被关机丢弃」的行写中断留痕（分片 #3：
// 状态从 error 改写 interrupted）。丢弃时日志正文随任务一起丢是共识代价，但行不能停在
// 建行值 processing——那会让「这行日志没记全」伪装成一条成功记录；也不能记 error：
// 关机收尾是本机行为，与供应商无关，记 error 会冤枉上游。同一语义在启动兜底里由
// repository.MarkUnfinishedLogsInterrupted 覆盖上一次进程遗留的行，两个写入点合起来
// 才是完整的「进程先走了」口径。
//
// 走 updateChatLogByID 单源收口（detached ctx）：丢弃只发生在关机收尾段，请求
// ctx 与 bgtask 都在关，不陪葬的写才落得下去。刻意不走 RecordLog 的错误分支——
// 那条路会连带写 ProviderStats 失败计数，而这个请求本身是成功的（响应已交付，
// 丢的只是日志正文），误记失败会冤枉供应商。
func MarkLogUnfinished(ctx context.Context, logID uint, reqStart time.Time) {
	updateChatLogByID(ctx, logID, models.ChatLog{
		Status:    models.ChatLogStatusInterrupted,
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
//
// 与 callerWithdrawn 的差别只有 io.ErrClosedPipe 一条（见后者注释）。
func isClientDisconnectErr(err error) bool {
	if errors.Is(err, io.ErrClosedPipe) {
		return true
	}
	return callerWithdrawn(err)
}

// callerWithdrawn 判定「调用方撤走」：请求 ctx 被取消（点停止 / 关页面）或连接被重置
// （非超时 net.Error）。超时**不算**——慢上游与「预算烧完」都是供应商侧的责任
// （Stage 33 / Stage 39 拍板），归它记失败。
//
// 与 isClientDisconnectErr 的差别只有 io.ErrClosedPipe：那个错误来自日志管道提前收尾
// （handler 的 TeeReader 刻意打在方向标记之外），成因通常在链路或上游内容、不在调用方，
// 归属待 Stage 44 的线上取样结论，故暂由 isClientDisconnectErr 单独叠加那一条。取样
// 结论明确后两条判据合并成单一判据（届时 providerAtFault 与 clientSideFailure 互为否定）。
func callerWithdrawn(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return !netErr.Timeout()
	}
	return false
}

// clientSideFailure 判定这次失败是不是调用方造成的：客户端断连（点停止、代理撤走、
// 坏管道）与**写回被写上限掐断**（Stage 39，客户端不收结果）同族，供应商都没做错。
//
// 清粘性钉定与留痕级别两处同源：两处各判一次会随新增失败类型漂移（Stage 39 之前
// 清钉判「不是断连就清」、级别判「是断连就降 Warn」，写超时一类同时落在「该清」与
// 「该升 ERROR」两个错误结论上）。既有行为逐条等价：不含写侧标记时本函数 ≡
// isClientDisconnectErr。
func clientSideFailure(err error) bool {
	return isClientDisconnectErr(err) || IsClientWriteErr(err)
}

// providerAtFault 判「这次提交后失败该不该记在供应商头上」：统计面与调度面共用的单源
// 判据（Stage 44 拍板，2026-10-16）——调用方撤走不记，其余记。
//
// 判据优先级与清钉一致：读侧标记是**错误产生处记下的方向事实**，优先于类型反推——
// 上游 RST（非超时 net.Error）会被 callerWithdrawn 误判成客户端撤走（真机实测过），
// 若不给标记优先，稳定截断的家反而免账。
//
// 它与 clientSideFailure **不是**互补关系：唯一差集是 io.ErrClosedPipe（日志管道提前
// 收尾那格），该格归属待线上取样结论，故刻意留在「记供应商账」一侧、两处暂不同步。
func providerAtFault(err error) bool {
	if IsUpstreamReadErr(err) {
		return true
	}
	return !IsClientWriteErr(err) && !callerWithdrawn(err)
}

// failureLogStatus 决定客户端侧失败出口给 chat_logs 行落哪个终态（状态机）：调用方
// 主动撤走记 canceled，其余一律 error。
//
// 判据只认 errors.Is(err, context.Canceled)，**刻意不扩到 clientSideFailure 全族**：
// context.Canceled 在请求路径上精确对应「调用方撤走」这一个事实——本仓请求路径不
// 自造 WithCancel（只有 WithTimeout，超时落 DeadlineExceeded 而非 Canceled），而写回
// 被写上限掐断（Stage 39）与坏管道是「客户端不收结果」、不是「客户端主动取消」，混进来
// 会让该状态失去语义。
//
// 单点判定的理由同 clientSideFailure：状态写入有两处（attempt 层的
// buildAttemptFailureUpdate 覆盖提交前五个失败出口、RecordLog 的 processer 错误分支覆盖
// 提交后失败），各判一次会随新增失败类型漂移。
//
// **覆盖面边界**（新增失败出口时先看这里）：另三类状态写入点刻意不走本判定，因为它们的
// 失败原因与请求 ctx 无关，判据不成立——非通过状态码（`handleNonOKProviderResponse`，
// cause 就是上游状态码文本）、关机丢弃（`MarkLogUnfinished`，按「进程先走了」独立记
// interrupted）、构建期失败（`savePreProviderFailureLog` / `ChatLog.WithError` 走的
// retryLog 通道行）。只有「失败原因可能由调用方撤走引发」的出口才该接本判定。
func failureLogStatus(err error) string {
	if errors.Is(err, context.Canceled) {
		return models.ChatLogStatusCanceled
	}
	return models.ChatLogStatusError
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
