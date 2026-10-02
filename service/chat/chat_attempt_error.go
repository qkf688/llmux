package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/adjustment"
	"github.com/qkf688/llmux/service/chatstats"
)

func handleNonOKProviderResponse(
	ctx context.Context,
	res *http.Response,
	logID uint,
	logRawOptions models.RawLogOptions,
	logSnapshot requestLogSnapshot,
	modelWithProvider models.ModelWithProvider,
	provider models.Provider,
	start time.Time, // 请求开始时刻（handler startReq）：用于回填 ProxyTime 终值
	cred models.Credential, // 本次尝试选中的凭据：凭据级失败时写冷却（#13）
) singleProviderAttemptResult {
	// Stage 2 资源防御：错误体走诊断单源上限（1 MiB）——成败判定只依赖状态码，
	// 错误细节提取与 RawResponseBody 采集都不需要超限部分；超大 body 的病态/被
	// 劫持上游在此被 LimitReader 拦下，不再整块进内存再拼进日志。
	byteBody, err := common.ReadCappedBody(res.Body)
	if err != nil {
		slog.Error("read body error", "error", err)
	}

	// Stage 2-B/D：错误体读侧 1 MiB 诊断单源封顶，落库文本再截到采集上限（四字段同源）。
	// 截断上限读在活 ctx 上（审查回批 W2）：本出口常在请求 ctx 已死时执行，
	// 死 ctx 读设置会退默认值，管理员配置的采集上限在失败路径静默失效。
	var fieldLimit int
	withDetachedLogCtx(ctx, func(dctx context.Context) { fieldLimit = getRawLogFieldMaxBytes(dctx) })
	rawBody := common.TruncateUTF8(string(byteBody), fieldLimit)

	errorUpdate := models.ChatLog{
		Status: "error",
		Error:  fmt.Sprintf("status: %d, body: %s", res.StatusCode, rawBody),
		// 回填端到端耗时：上游已返回完整错误响应，建行快照不含上游耗时。
		ProxyTime: time.Since(start),
	}

	if logRawOptions.RequestHeaders || logRawOptions.RequestBody || logRawOptions.RawRequestBody || logRawOptions.ResponseHeaders || logRawOptions.RawResponseBody {
		if logRawOptions.ResponseHeaders {
			errorUpdate.ResponseHeaders = string(responseHeadersJSON(res.Header))
		}
		if logRawOptions.RequestHeaders {
			errorUpdate.RequestHeaders = string(logSnapshot.RequestHeadersJSON)
		}
		if logRawOptions.RequestBody {
			errorUpdate.RequestBody = logSnapshot.RequestBodyStr
		}
		if logRawOptions.RawRequestBody {
			errorUpdate.RawRequestBody = logSnapshot.RawRequestBodyStr
		}
		if logRawOptions.RawResponseBody {
			errorUpdate.RawResponseBody = rawBody
		}
	}

	updateChatLogByID(ctx, logID, errorUpdate, "failed to update log status")

	settlement := beginFailureSettlement(ctx)
	defer settlement.Close()
	if !settlement.Active {
		// 调用方已放弃（客户端断连）：不是上游/凭据的故障，冷却/判停/统计/衰减一律不写。
		res.Body.Close()
		return requestAbandonedResult(ctx)
	}

	// 组织级 adjustment 只在「非凭据级失败」时在 attempt 内累计（#13）：凭据级失败
	// （限流/鉴权/5xx）组内消化，由 retry loop 组耗尽时统一补 ConsecutiveFailures/
	// 衰减（设计定案第 5 节「层内耗尽才累计组织级」）。chatstats 统计保留现状。
	//
	// 凭据级失败（限流/鉴权/5xx）的凭据侧处置（#13 组内故障转移 + #6-2 分类型）：
	// 鉴权失败（401/403）走判停——连败计数 + 达阈值 temp_unsched（不写冷却）；
	// 其余（429/5xx）写最小冷却，窗口内选路跳过该凭据并允许同组换 key。
	// 组织级淘汰标记保留，单 key 组（存量迁移形态）组耗尽后行为与现状完全等价。
	// 凭据级返回标记与 429 一票淘汰（2026-09-22 有意废弃旧「ReduceWeight 留池降权」
	// 语义）：限流候选本次请求内不再回头，防重试循环随机撞回刚限流的供应商——与
	// 「失败即换下一家」语义对齐；组内换 key 仍优先（CredentialFailure 标记），
	// 同组未限流 key 不受影响。组织级淘汰标记组内耗尽才真正消费。
	//
	// （Stage 25 合并：两个紧挨的同条件 classifyCredentialFailure 分支合为一处，
	// 改一处漏一处的维护陷阱消除，行为等价。）
	if classifyCredentialFailure(res.StatusCode) {
		recordCredentialStatsError(settlement.Ctx, cred)
		if classifyAuthFailure(res.StatusCode) {
			applyCredentialAuthFailure(settlement.Ctx, cred)
		} else {
			applyCredentialCooldown(settlement.Ctx, cred, cooldownReasonForStatus(res.StatusCode))
		}
	} else {
		applyProviderFailureAdjustments(settlement.Ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
	}
	if err := chatstats.RecordProviderStats(context.Background(), provider.Name, false, 0, 0); err != nil {
		slog.Warn("failed to record provider stats", "provider", provider.Name, "error", err)
	}
	res.Body.Close()

	if classifyCredentialFailure(res.StatusCode) {
		return singleProviderAttemptResult{CredentialFailure: true, RemoveWeight: true, RemovePriority: true}
	}
	// 其余 4xx（请求体被拒等）：请求本身的问题，换 key 无意义——组织级失败，不标凭据级。
	return singleProviderAttemptResult{RemoveWeight: true, RemovePriority: true}
}

func applyProviderFailureAdjustments(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	adjustment.IncrementConsecutiveFailures(ctx, modelProviderID, providerName, providerModel)
	adjustment.ApplyWeightDecayByModelProviderID(ctx, modelProviderID, providerName, providerModel)
	adjustment.ApplyPriorityDecayByModelProviderID(ctx, modelProviderID, providerName, providerModel)
}

// wrapTransformError 是 transform 失败出口的错误包装单源（request/response 两处共用，
// Stage 25）：必须用 %w 保住错误链——转换失败常因「请求取消打断上游读」，
// isClientDisconnectErr / httpresp.isClientDisconnect 只认 errors.Is/As 语义匹配，
// 断链包装会让断连降 Warn 的级别判定在该路径静默失效。契约测试
// chat_attempt_transform_wrap_test.go 锁 unwrap 与文案形状。
func wrapTransformError(stage string, err error) error {
	return fmt.Errorf("transform %s error: %w", stage, err)
}

// buildAttemptFailureUpdate 组装「尝试失败」日志更新（Status=error + 原因 + 终值耗时 +
// 可选采集字段）。全部失败出口共用此处的字段选择逻辑（DRY：此前 Client.Do 错误 /
// 转换失败 / 读体超限 / 读体失败 / 提交门拒绝五处各写一份近似块，改一处漏四处）。
// res 为 nil 时跳过响应头采集；rawResponseBodyStr 非空且开启采集时落 RawResponseBody。
func buildAttemptFailureUpdate(cause error, proxyTime time.Duration, logRawOptions models.RawLogOptions, logSnapshot requestLogSnapshot, res *http.Response, rawResponseBodyStr string) models.ChatLog {
	errorUpdate := models.ChatLog{
		Status:    "error",
		Error:     cause.Error(),
		ProxyTime: proxyTime,
	}
	if res != nil && logRawOptions.ResponseHeaders {
		errorUpdate.ResponseHeaders = string(responseHeadersJSON(res.Header))
	}
	if logRawOptions.RequestHeaders {
		errorUpdate.RequestHeaders = string(logSnapshot.RequestHeadersJSON)
	}
	if logRawOptions.RequestBody {
		errorUpdate.RequestBody = logSnapshot.RequestBodyStr
	}
	if logRawOptions.RawRequestBody {
		errorUpdate.RawRequestBody = logSnapshot.RawRequestBodyStr
	}
	if logRawOptions.RawResponseBody && rawResponseBodyStr != "" {
		errorUpdate.RawResponseBody = rawResponseBodyStr
	}
	return errorUpdate
}

// headerWindowTimeoutMarker 是 net/http 在 ResponseHeaderTimeout 到点时给出的原文片段。
// Do 阶段还有两类超时来自 providers 连接池调优常量（dial 30s / TLS 握手 10s），与本次
// 窗口值无关——判据刻意用**正向标记**而不是「排除已知的其它超时来源」：将来出现新的
// 超时来源会落到「不贴类别」（原始文本仍在日志里），而不是「贴错类别」。若 stdlib 改了
// 这句文案，退化为不贴类别，不会给出错误归因。
const headerWindowTimeoutMarker = "timeout awaiting response headers"

// isUpstreamWindowTimeout 判「这次 Do 失败是我们自己的单次尝试窗口到点掐断的」：
//   - 错误自报超时（net.Error.Timeout()）；
//   - 请求 ctx 未到点——ctx 到点是网关总超时（600s 帽子，归因走 504），不算窗口掐断；
//   - 原文命中 net/http 的等响应头超时标记。
func isUpstreamWindowTimeout(ctx context.Context, err error) bool {
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	return strings.Contains(err.Error(), headerWindowTimeoutMarker)
}

// describeUpstreamWindowTimeout 给「被我们窗口掐断」的失败文本补上人能读的类别与窗口
// 秒数（Stage 38）：面板上要能一眼分开「窗口掐的」与「上游自己断的」，而客户端原文只有
// 一句英文。原始错误用 %w 保留在链上——isClientDisconnectErr / httpresp.isClientDisconnect
// 等判据按 errors.Is/As 匹配，断链会让归因静默失效（同 wrapTransformError 的约束）。
// 窗口未知（≤0，测试直构 attempt 的形态）时不贴类别：无从判断是哪个窗口掐的。
func describeUpstreamWindowTimeout(ctx context.Context, err error, stream bool, window time.Duration) error {
	if window <= 0 || !isUpstreamWindowTimeout(ctx, err) {
		return err
	}
	seconds := int(window / time.Second)
	if stream {
		return fmt.Errorf("等待上游响应头超过 %d 秒（流式等头窗口，网关主动掐断）: %w", seconds, err)
	}
	return fmt.Errorf("等待上游整段回答超过 %d 秒（非流式整段预算窗口，网关主动掐断）: %w", seconds, err)
}

func applyProviderSelectionResult(weightItems, priorityItems map[uint]int, id uint, result singleProviderAttemptResult) {
	if result.RemoveWeight {
		delete(weightItems, id)
	}
	if result.RemovePriority {
		delete(priorityItems, id)
	}
}
