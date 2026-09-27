package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/chatstats"
)

// errUpstreamBodyOverflow 上游非流式响应体超过「上游最大响应体」设置（Stage 2-C）。
// 哨兵错误：调用侧据此判**组织级**失败——超大 body 是端点级行为（被劫持/上游故障），
// 与本次用的哪条 key 无关，故不写凭据冷却，只剔除该候选换下一家。
var errUpstreamBodyOverflow = errors.New("upstream response body exceeds configured limit")

// limitUpstreamResponseBody 在响应头提交给客户端**之前**、于 attempt 内对非流式上游
// Body 做整读体积闸：读到 limit+1 字节即判定超限（多探 1 字节区分「恰好等于」与「更大」），
// 未超限则回填为可重复读的内存字节，下游（采集/转换/直通 io.Copy）照常消费。
// 返回已物化字节供提交门（chat_commit_gate.go）做内容校验，避免二次整读（32 MiB
// 上限内无必要 copy）。event-stream 响应不整读，body 原样留给流式路径、返回 (nil, nil)
// ——「未物化」的判据单源是 Content-Type（common.IsEventStreamContentType），提交门
// 按它自行分流，本函数不再返回 materialized 标记（err≠nil 时 body 恒 nil，无死信息格）。
//
// 为什么闸在 attempt 而不是 transform 整读处：直通路径（客户端与上游同协议）根本不进
// transform_provider_response.go 的整读，闸放 attempt 一处即覆盖直通+转换两条链路（OCP：
// 以后加新协议也不用再补闸）。流式（SSE）不整读、天然无界缓冲问题，不在本闸内
// （帧级防御归提交门的预读预算与 maxSSEEventSize）。
//
// 失败语义分两类（上游 Body 只有"超限"这一种是我们主动裁决的失败，其余读取错误是
// 传输层事件）：
//   - 超限 → errUpstreamBodyOverflow → 组织级淘汰，不冷却凭据；
//   - 读错误（含 ctx 超时打断读）→ 与 Client.Do 失败分支同归**凭据级网络**（#13）。
//     超时分类必须成对（Do 分支 + 读体分支），只挂 Do 侧则读体超时静默漏走网络类
//     ——RULES「Go HTTP 超时」条目，io.ReadAll(res.Body) 会被 ctx deadline 打断。
func limitUpstreamResponseBody(ctx context.Context, res *http.Response) (body []byte, err error) {
	if common.IsEventStreamContentType(res.Header.Get("Content-Type")) {
		return nil, nil
	}
	limit := getUpstreamMaxResponseBytes(ctx)
	body, err = io.ReadAll(io.LimitReader(res.Body, limit+1))
	// 整读后连接已耗尽回池，Close 错误（http 响应体正常路径恒为 nil）无处置价值，忽略。
	_ = res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read upstream response body: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: got at least %d bytes, limit %d bytes", errUpstreamBodyOverflow, len(body), limit)
	}
	res.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// handleUpstreamBodyOverflow 超限出口：组织级失败（换下一家、不冷却 key），
// 与 transform 错误分支同处置（失败计数/权重衰减照常累计——这是供应商级异常）。
func handleUpstreamBodyOverflow(input singleProviderAttemptInput, res *http.Response, logID uint, logRawOptions models.RawLogOptions, logSnapshot requestLogSnapshot, cause error) singleProviderAttemptResult {
	updateChatLogByID(input.Ctx, logID,
		buildAttemptFailureUpdate(cause, time.Since(input.Start), logRawOptions, logSnapshot, res, ""),
		"failed to update log status")
	settlement := beginFailureSettlement(input.Ctx)
	defer settlement.Close()
	if !settlement.Active {
		// 调用方已放弃：不是端点/凭据的故障，不淘汰不衰减不冷却。
		return requestAbandonedResult(input.Ctx)
	}
	applyProviderFailureAdjustments(settlement.Ctx, input.ModelWithProvider.ID, input.Provider.Name, input.ModelWithProvider.ProviderModel)
	if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
		slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
	}
	slog.Warn("upstream response body over limit, failing over",
		"provider", input.Provider.Name, "model_provider_id", input.ModelWithProvider.ID, "error", cause)
	// Body 已由 limitUpstreamResponseBody 关闭（超限路径不回填 reader）。
	return singleProviderAttemptResult{RemoveWeight: true, RemovePriority: true}
}

// handleUpstreamBodyReadError 读体中途失败（网络错误/ctx 超时打断）出口：
// 与 Client.Do 失败分支完全同构（#13 凭据级：冷却 + 终身统计 + 组织级淘汰标记）。
// 提交门预读失败（看门狗超时 / 预读读错）同归本出口。
func handleUpstreamBodyReadError(input singleProviderAttemptInput, res *http.Response, logID uint, logRawOptions models.RawLogOptions, logSnapshot requestLogSnapshot, cause error) singleProviderAttemptResult {
	updateChatLogByID(input.Ctx, logID,
		buildAttemptFailureUpdate(cause, time.Since(input.Start), logRawOptions, logSnapshot, res, ""),
		"failed to update log status")
	settlement := beginFailureSettlement(input.Ctx)
	defer settlement.Close()
	if !settlement.Active {
		// 调用方已放弃（客户端断连打断读体 / 提交门预读）：不是 key 的错，不冷却不计数。
		return requestAbandonedResult(input.Ctx)
	}
	if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
		slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
	}
	// 组织级 adjustment 不在 attempt 内累计（同 Do 分支）：读体失败按凭据级网络处理，
	// 组耗尽由 retry loop 统一补。
	applyCredentialCooldown(settlement.Ctx, input.Selection.Credential, cooldownReasonNetwork)
	recordCredentialStatsError(settlement.Ctx, input.Selection.Credential)
	return singleProviderAttemptResult{CredentialFailure: true, RemoveWeight: true, RemovePriority: true}
}

// responseHeadersJSON 响应头采集的序列化单一实现（RawLogOptions.ResponseHeaders 各失败出口
// 与成功路径回写共用；marshal 失败退化为空对象——采集是尽力而为的旁路，不能因它改写失败语义）。
// Stage 26：落库前过敏感头掩码（判据单源 common.MaskSensitiveLogHeaders，与请求侧同名单，
// Set-Cookie 已入名单）——响应头不再明文落库。
func responseHeadersJSON(h http.Header) []byte {
	b, err := json.Marshal(common.MaskSensitiveLogHeaders(h))
	if err != nil {
		slog.Error("failed to marshal response headers", "error", err)
		return []byte("{}")
	}
	return b
}
