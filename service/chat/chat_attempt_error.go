package chat

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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
	if !classifyCredentialFailure(res.StatusCode) {
		applyProviderFailureAdjustments(settlement.Ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
	}
	if err := chatstats.RecordProviderStats(context.Background(), provider.Name, false, 0, 0); err != nil {
		slog.Warn("failed to record provider stats", "provider", provider.Name, "error", err)
	}
	res.Body.Close()

	// 凭据级失败（限流/鉴权/5xx）的凭据侧处置（#13 组内故障转移 + #6-2 分类型）：
	// 鉴权失败（401/403）走判停——连败计数 + 达阈值 temp_unsched（不写冷却）；
	// 其余（429/5xx）写最小冷却，窗口内选路跳过该凭据并允许同组换 key。
	// 组织级淘汰标记保留，单 key 组（存量迁移形态）组耗尽后行为与现状完全等价。
	if classifyCredentialFailure(res.StatusCode) {
		// 终身统计：凭据级失败域（401/403/429/5xx）一处收口，与下方状态机处置正交。
		recordCredentialStatsError(settlement.Ctx, cred)
		if classifyAuthFailure(res.StatusCode) {
			applyCredentialAuthFailure(settlement.Ctx, cred)
		} else {
			applyCredentialCooldown(settlement.Ctx, cred, cooldownReasonForStatus(res.StatusCode))
		}
	}

	if classifyCredentialFailure(res.StatusCode) {
		// 5xx / 401 / 403 / 429：凭据级失败，组织级淘汰标记保留（现状语义），
		// 组内耗尽才真正消费（retry loop 淘汰候选时同现状）。
		// 429 一票淘汰（2026-09-22 有意废弃旧「ReduceWeight 留池降权」语义）：
		// 限流候选本次请求内不再回头，防重试循环随机撞回刚限流的供应商——与
		// 「失败即换下一家」语义对齐；组内换 key 仍优先（CredentialFailure 标记），
		// 同组未限流 key 不受影响。
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

func applyProviderSelectionResult(weightItems, priorityItems map[uint]int, id uint, result singleProviderAttemptResult) {
	if result.RemoveWeight {
		delete(weightItems, id)
	}
	if result.RemovePriority {
		delete(priorityItems, id)
	}
}
