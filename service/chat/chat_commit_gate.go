package chat

import (
	"bufio"
	"bytes"
	"context"
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

// 提交门（commit-before-write）：响应交给 handler 写给客户端**之前**先验「内容可提交」。
// 为什么在 attempt 内而不是 handler：HTTP 头写出即不可撤回，「失败还能换下家」的唯一
// 窗口是写头之前——判定与重试循环（本包）同层，失败才能回到候选池继续选下一家；
// handler 只做搬运（AGENTS 4.3：HTTP 接入在 handler，上游/重试语义在 service）。
//
// 两条判据（均先于任何客户端字节）：
//   - 流式：预读**第一非空帧**判可提交（错误载荷 / 非法 JSON / 零帧即失败），预读
//     字节经 Replay 完整回放，透传/转换/handler 全程无感知（Octopus relay 同款
//     「验证首事件后才 Commit」语义）。第二帧起的错误属已提交后语义，不再重试，
//     由 handler 的协议内 error 事件收口。
//   - 非流式：已物化 body 必须是合法 JSON 对象且非错误载荷（直通路径此前没有任何
//     解析步骤，200 + HTML/错误对象会原样漏给客户端）。转换路径的解析失败另有
//     既有出口，此处判据同源、双保险不冲突。

const (
	// streamGateMaxLineBytes 单行上限：与 common 分帧默认同值，防畸形行无界缓冲。
	streamGateMaxLineBytes = 1 << 20
	// streamGateMaxTotalBytes 预读预算：首帧迟迟不来的大段前言按不可提交处置
	// （无预算时恶意上游可发无限注释行拖住预读）。
	streamGateMaxTotalBytes = 4 << 20
)

// errStreamGateAccepted 预读内部哨兵：首帧已定论、停止扫描（不是失败）。
var errStreamGateAccepted = errors.New("stream content gate: first frame decided")

// errUpstreamContentInvalid 内容不可提交的分类哨兵：调用方据此走组织级淘汰
// （换下一家、不冷却凭据）；其余错误（看门狗超时 / 预读读错）走凭据级网络出口。
var errUpstreamContentInvalid = errors.New("upstream content not committable")

// gateUpstreamCommitment 对本次尝试的上游响应做提交门验证，返回可提交的 res
// （流式 body 已换为可重放流）。失败时返回的错误按 errUpstreamContentInvalid 分类。
// materialized 由 limitUpstreamResponseBody 给出（流式响应体未物化为 false）：
// 非流式仅在 body 已物化时校验——上游对非流式请求回 SSE 的残余盲区沿用现状
// （无法廉价整验，见 framework-conventions §6 已知边界）。
func gateUpstreamCommitment(input singleProviderAttemptInput, res *http.Response, materializedBody []byte, materialized bool) (*http.Response, error) {
	if input.Before.Stream {
		gated, err := gateStreamCommitment(res.Body, time.Duration(getStreamFirstByteTimeout(input.Ctx))*time.Second)
		if err != nil {
			return nil, err
		}
		res.Body = gated
		return res, nil
	}
	if materialized {
		if err := common.JSONContentError(materializedBody); err != nil {
			return nil, fmt.Errorf("%w: %w", errUpstreamContentInvalid, err)
		}
	}
	return res, nil
}

// gateStreamCommitment 预读流式 body 的第一非空帧判可提交：通过则返回可重放 body
// （预读字节完整回放，逐字节等于原始流）；失败按错误分类交调用方分流处置。
func gateStreamCommitment(body io.ReadCloser, firstByteTimeout time.Duration) (io.ReadCloser, error) {
	capture := newCapturingReadCloser(body)
	watched := newFirstByteWatchdogReader(capture, firstByteTimeout)

	var contentErr error
	_, scanErr := common.ScanSSEFrames(watched,
		common.SSEScanConfig{MaxLineBytes: streamGateMaxLineBytes, MaxTotalBytes: streamGateMaxTotalBytes},
		func(frame common.SSEFrame, _ common.SSEScanStats) error {
			contentErr = common.SSEFrameContentError(frame)
			// 首帧即定论：可提交则放行，不可提交也只需这一帧的证据（「第一非空事件」
			// 判据——主流故障形态是错误帧/垃圾体打头；第二帧起的错误已属提交后语义）。
			return errStreamGateAccepted
		})

	switch {
	case contentErr != nil:
		capture.Close()
		return nil, fmt.Errorf("%w: first frame rejected: %w", errUpstreamContentInvalid, contentErr)
	case errors.Is(scanErr, errStreamGateAccepted):
		return capture.Replay(), nil
	case isGateContentFault(scanErr):
		// 预算超限（*SSEBudgetError）与畸形巨行（bufio.ErrTooLong）是「上游吐垃圾」的
		// 主动止损/坏流判定，与错误帧同权归不可提交：组织级换下家、不冷却凭据。
		// 归凭据级网络出口会让组内 key 逐条陪烧、重试循环反复撞回同一坏端点
		//（垃圾洪水罚 key 的分类错位，2026-09-23 审查回批修正）。
		capture.Close()
		return nil, fmt.Errorf("%w: %w", errUpstreamContentInvalid, scanErr)
	case scanErr != nil:
		// 看门狗超时（errFirstByteWatchdog）与真读错同归凭据级网络出口，原样上抛分类。
		capture.Close()
		return nil, scanErr
	default:
		// 正常读完却零帧：200 + 非 SSE（JSON 错误体 / HTML / 空体）——不可提交。
		capture.Close()
		return nil, fmt.Errorf("%w: no SSE frames in stream response", errUpstreamContentInvalid)
	}
}

// isGateContentFault 判定扫描错误是否「上游内容坏」而非「传输失败」：预算超限是
// 主动止损、单行超限是畸形流（下游分帧器同样 1 MiB 封顶，这种流整条链路都消费不动），
// 都该按垃圾内容处置（组织级），与看门狗超时/读错（凭据级）区分。
func isGateContentFault(err error) bool {
	var budgetErr *common.SSEBudgetError
	return errors.As(err, &budgetErr) || errors.Is(err, bufio.ErrTooLong)
}

// capturingReadCloser 包住上游 body，记录预读字节供 Replay 完整回放。仅提交门
// 预读阶段单 goroutine 顺序「Read → Replay → 下游消费」，无并发、不加锁。
type capturingReadCloser struct {
	src       io.ReadCloser
	buf       bytes.Buffer
	recording bool
}

func newCapturingReadCloser(src io.ReadCloser) *capturingReadCloser {
	return &capturingReadCloser{src: src, recording: true}
}

func (c *capturingReadCloser) Read(p []byte) (int, error) {
	n, err := c.src.Read(p)
	if c.recording && n > 0 {
		c.buf.Write(p[:n])
	}
	return n, err
}

func (c *capturingReadCloser) Close() error {
	return c.src.Close()
}

// Replay 停止记录并返回「预读字节 + 剩余流」的完整重放体（逐字节等于原始流，
// 含扫描器预读进缓冲但未消费的字节——重放与续读无缝衔接、不重不漏）。
func (c *capturingReadCloser) Replay() io.ReadCloser {
	c.recording = false
	prefix := append([]byte(nil), c.buf.Bytes()...)
	return &replayReadCloser{
		Reader: io.MultiReader(bytes.NewReader(prefix), c.src),
		Closer: c.src,
	}
}

type replayReadCloser struct {
	io.Reader
	io.Closer
}

// handleUpstreamContentInvalid 内容不可提交出口：组织级失败（换下一家、不冷却凭据，
// 与响应转换失败同语义——垃圾内容是端点/供应商级行为，与本次用的哪条 key 无关）。
func handleUpstreamContentInvalid(input singleProviderAttemptInput, res *http.Response, logID uint, logRawOptions models.RawLogOptions, logSnapshot requestLogSnapshot, cause error) singleProviderAttemptResult {
	proxyTime := time.Since(input.Start)
	updateChatLogByID(input.Ctx, logID,
		buildAttemptFailureUpdate(cause, proxyTime, logRawOptions, logSnapshot, res, ""),
		"failed to update log status")
	settlement := beginFailureSettlement(input.Ctx)
	defer settlement.Close()
	if !settlement.Active {
		// 调用方已放弃：不淘汰、不衰减、不冷却——垃圾内容是端点行为，但这次请求不是它失败的。
		res.Body.Close()
		return requestAbandonedResult(input.Ctx)
	}
	applyProviderFailureAdjustments(settlement.Ctx, input.ModelWithProvider.ID, input.Provider.Name, input.ModelWithProvider.ProviderModel)
	if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
		slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
	}
	slog.Warn("upstream content not committable, failing over",
		"provider", input.Provider.Name, "model_provider_id", input.ModelWithProvider.ID, "error", cause)
	res.Body.Close()
	return singleProviderAttemptResult{RemoveWeight: true, RemovePriority: true}
}
