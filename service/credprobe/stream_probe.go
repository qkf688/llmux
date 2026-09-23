package credprobe

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/providers"
	"github.com/tidwall/sjson"
)

// 本文件是「一次真实流式 completion 探针」编排的单一来源，与 completion_probe.go 并列：
// 两者共享超时余量分级、状态码归类与失败细节口径，差异只在「读一条 SSE 流」而非「读一块 body」。
//
// 分层纪律（SRP）：探针只回答「打到上游了吗、回的是不是合规 SSE 流、读完花了多久」，
// **不判断内容对不对**——哪些 delta 算推理、终止事件是否缺失、能力是否具备，属调用方
// （handler/testapi）的协议语义。因此空流也算探针通过，由端点判失败。

// SSEFrame 一个已分帧的 SSE 事件。分帧判据全库单源于 common.ScanSSEFrames，
// 此别名保住本包与 handler/testapi 折叠链的既有引用面。
type SSEFrame = common.SSEFrame

// ProbeSummaryStreamUnsupported 上游状态码已判通过但没按 SSE 回（含吃掉 stream 参数直接回 JSON 的
// 兼容网关）：是能力缺失而非网络故障，故 error_type 归 validation。
// 这条文案同时是管理端 ReAct 测试 panic 登记项的根因诊断入口。
const (
	ProbeSummaryStreamUnsupported = "上游不支持流式"
	ProbeSummaryStreamTooLarge    = "流式响应超出预算"
)

const (
	// streamProbeMaxLineBytes 单行（单帧内任一行）上限：与 common.ReadCappedBody 的诊断口径
	// 同数量级，合法探针单帧远小于此，只挡畸形上游。
	streamProbeMaxLineBytes = 1 << 20
	// streamProbeMaxTotalBytes 整条流的读取预算。非流式探针有 1 MiB 上限是因为「判定不看
	// 响应全文」；流式判定恰恰要看完整折叠内容，所以不能用它，改按「流」单独设预算。
	// 超限即中止并按上游异常归类：管理端点被一个失控上游打爆进程是不可接受的。
	streamProbeMaxTotalBytes = 4 << 20
	// streamProbeMaxFrames 帧数预算。字节预算**封不住判定输入**：最坏情况下每条帧只占
	// 9 字节（`data: x\n\n`），4 MiB 能攒出 46 万帧，Frames 切片的常驻内存与 renderSSEText
	// 的重建开销随之放大到几十 MB。合法探针流是几十~几百帧量级，1 万足够宽，超了就是上游异常。
	streamProbeMaxFrames = 10000
	// RawOutput 的头尾保留预算（策略与 service/transform/streaming/raw_accumulator.go 一致：
	// 收尾事件都在末尾，纯头截会丢掉最需要的信息）。
	streamProbeRawHeadBytes = 224 << 10
	streamProbeRawTailBytes = 32 << 10

	// streamProbePreviewBytes 「不支持流式」时响应体预览长度：够看清上游到底回了什么形状。
	streamProbePreviewBytes = 200
)

// StreamProbeRequest 一次流式 completion 探针输入。Body 传**原始测试体**：stream 标记由
// 探针内部注入，测试体因此不需要为流式复制一份常量（见 handler/testapi 测试体注册表）。
type StreamProbeRequest struct {
	Ctx     context.Context
	Client  providers.Provider
	Timeout time.Duration // 同 RunCompletionProbe：ctx deadline 口径，≤0 兜底 common.DiagnosticCompletionTimeout
	Headers http.Header
	Model   string
	Body    []byte
}

// StreamProbeResult Passed=false 时 ErrorType/Summary/Detail 有值。
//
// 双轨口径（重要）：Frames 是**无损**判定输入；RawOutput 是**有损**回显（超预算时中段丢弃并
// 留截断标记，且由帧重建、不回显 SSE 注释行）。调用方绝不可拿 RawOutput 反推判定，
// 否则截断会把「流被切断」伪装成「内容完整」。
type StreamProbeResult struct {
	Passed     bool
	StatusCode int // 0 = 未打到上游或未拿到响应
	ErrorType  string
	Summary    string
	Detail     string

	Frames []SSEFrame
	// Framed 标记「真的进到分帧阶段」（Content-Type 门已过）。它决定调用方能不能报
	// terminated_by：连接失败/不支持流式时 frames=0 与 terminated_by=none 都是无意义的事实，
	// 写出去会把「没连上」和「流被切断」两类不同缺陷合成一条结论。
	Framed       bool
	RawOutput    string
	FirstFrameMS int64 // 发出请求 → 首个 SSE 帧交付（含连接与 TTFB；探针不认协议，故不是「首个内容增量」）
	DurationMS   int64
}

// RunStreamProbe 打一次真实流式 completion 并按统一口径归类：注入 stream → BuildReq →
// Do → 未通过判定归类 → Content-Type 硬门 → 逐帧读取 → 耗时统计。
func RunStreamProbe(req StreamProbeRequest) (out StreamProbeResult) {
	if req.Ctx == nil {
		req.Ctx = context.Background()
	}
	if req.Timeout <= 0 {
		req.Timeout = common.DiagnosticCompletionTimeout
	}

	start := time.Now()
	// 命名返回值 + defer：DurationMS 在所有返回路径上都得填，逐处手写必漏一处。
	defer func() { out.DurationMS = time.Since(start).Milliseconds() }()

	streamBody, err := sjson.SetBytes(req.Body, "stream", true)
	if err != nil {
		// 注入失败只可能是测试体本身不是合法 JSON，属配置问题，不是网络故障。
		return StreamProbeResult{ErrorType: common.ErrorTypeValidation, Summary: ProbeSummaryBuildRequest, Detail: err.Error()}
	}

	// Clone：headers 是调用方（testapi 的 customer_headers 合并结果）的共享对象，
	// 探针不就地改别人的 map（并发安全与可推理性）。
	headers := req.Headers.Clone()
	if headers == nil {
		headers = http.Header{}
	}
	headers.Set("Accept", common.SSEContentType)
	// 防 `customer_headers` 显式配错：透传路径已在 `testapi.BuildTestHeaders` 白名单封死
	// Accept-Encoding，但 customer_headers 是设计上的无限制显式通道——管理员配了
	// `Accept-Encoding: gzip` 仍会走到这里；一旦由调用方设定，Go Transport 就**不再透明解压**，
	// 探针会对着 gzip 字节流分帧、把好的流式渠道判成「无内容增量」。解压交回 Transport 自己处理
	// 才有正确的分帧输入。（非流式 RunCompletionProbe 未做同款 Del：误配后果由显式通道自负。）
	headers.Del("Accept-Encoding")

	probeCtx, cancel := context.WithTimeout(req.Ctx, req.Timeout)
	defer cancel()

	httpReq, err := req.Client.BuildReq(probeCtx, headers, req.Model, streamBody)
	if err != nil {
		return StreamProbeResult{ErrorType: common.ErrorTypeNetwork, Summary: ProbeSummaryBuildRequest, Detail: err.Error()}
	}

	httpClient := providers.GetClientWithProxy(req.Timeout+common.DiagnosticTransportBuffer, req.Client.GetProxy())
	res, err := httpClient.Do(httpReq)
	if err != nil {
		return StreamProbeResult{ErrorType: classifyStreamDoError(err), Summary: ProbeSummaryConnect, Detail: err.Error()}
	}
	defer res.Body.Close()

	if !common.IsPassingStatus(res.StatusCode) {
		bodyBytes, readErr := common.ReadCappedBody(res.Body)
		if readErr != nil {
			return StreamProbeResult{
				StatusCode: res.StatusCode,
				ErrorType:  classifyStreamDoError(readErr),
				Summary:    ProbeSummaryReadResponse,
				Detail:     readErr.Error(),
			}
		}
		return StreamProbeResult{
			StatusCode: res.StatusCode,
			ErrorType:  common.ErrorTypeFromStatus(res.StatusCode),
			Summary:    ProbeSummaryUpstreamError,
			Detail:     common.UpstreamErrorDetail(res.StatusCode, bodyBytes),
			RawOutput:  string(bodyBytes),
		}
	}

	// Content-Type 硬门放在通过判定之后：未通过的状态码无论什么形状都是「上游回了错误」，
	// 归状态码更有诊断价值；只有判定通过后才需要区分「回了内容但不是流」。
	if !common.IsEventStreamContentType(res.Header.Get("Content-Type")) {
		bodyBytes, bodyErr := common.ReadCappedBody(res.Body)
		actualContentType := strings.TrimSpace(res.Header.Get("Content-Type"))
		if actualContentType == "" {
			actualContentType = "（缺失）"
		}
		detail := fmt.Sprintf("实际 Content-Type: %s；响应体预览: %s",
			actualContentType, truncateForDetail(bodyBytes, streamProbePreviewBytes))
		if bodyErr != nil {
			// 读体失败与「上游回了空体」必须可区分，否则诊断信息恰好在最需要时归零。
			detail += fmt.Sprintf("；响应体读取中断: %v", bodyErr)
		}
		return StreamProbeResult{
			StatusCode: res.StatusCode,
			ErrorType:  common.ErrorTypeValidation,
			Summary:    ProbeSummaryStreamUnsupported,
			Detail:     detail,
			RawOutput:  string(bodyBytes),
		}
	}

	// 分帧走全库唯一判据（common.ScanSSEFrames）：探针攒帧、首帧计时是消费侧策略，
	// 预算数值经 cfg 注入（探针止损语义不变）。
	var frames []SSEFrame
	var firstFrameMS int64
	_, scanErr := common.ScanSSEFrames(res.Body, common.SSEScanConfig{
		MaxLineBytes:  streamProbeMaxLineBytes,
		MaxTotalBytes: streamProbeMaxTotalBytes,
		MaxFrames:     streamProbeMaxFrames,
	}, func(frame SSEFrame, _ common.SSEScanStats) error {
		if len(frames) == 0 {
			firstFrameMS = time.Since(start).Milliseconds()
		}
		frames = append(frames, frame)
		return nil
	})

	var overBudget *common.SSEBudgetError
	switch {
	case errors.As(scanErr, &overBudget):
		return StreamProbeResult{
			StatusCode:   res.StatusCode,
			ErrorType:    common.ErrorTypeProvider,
			Summary:      ProbeSummaryStreamTooLarge,
			Detail:       fmt.Sprintf("已中止读取（%v，已读到 %d 帧）", overBudget, len(frames)),
			RawOutput:    renderSSEText(frames, streamProbeRawHeadBytes, streamProbeRawTailBytes),
			Framed:       true,
			FirstFrameMS: firstFrameMS,
		}
	case scanErr != nil:
		return StreamProbeResult{
			StatusCode:   res.StatusCode,
			ErrorType:    classifyStreamDoError(scanErr),
			Summary:      ProbeSummaryReadResponse,
			Detail:       scanErr.Error(),
			RawOutput:    renderSSEText(frames, streamProbeRawHeadBytes, streamProbeRawTailBytes),
			Framed:       true,
			FirstFrameMS: firstFrameMS,
		}
	}

	return StreamProbeResult{
		Passed:       true,
		StatusCode:   res.StatusCode,
		Frames:       frames,
		RawOutput:    renderSSEText(frames, streamProbeRawHeadBytes, streamProbeRawTailBytes),
		Framed:       true,
		FirstFrameMS: firstFrameMS,
	}
}

// classifyStreamDoError 把「没有状态码」的失败分成 timeout / network：两个探针共用同一
// 判据（errors.Is(DeadlineExceeded)），否则同一慢上游在非流式路径归 timeout、
// 流式路径归 network，用户在两个弹窗看到两个不相干的结论。
func classifyStreamDoError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return common.ErrorTypeTimeout
	}
	return common.ErrorTypeNetwork
}

// renderSSEText 由帧重建给人看的 SSE 文本并按头尾预算截断。
//
// 为什么重建而不另存一份原始字节：另存要多占一份与流等大的缓冲（预算上限是 MiB 量级），
// 而判定根本不需要它。代价是注释行与畸形行不回显——已记入方案「已知风险」。
func renderSSEText(frames []SSEFrame, headBytes, tailBytes int) string {
	var b strings.Builder
	for _, frame := range frames {
		if frame.Event != "" {
			b.WriteString("event: ")
			b.WriteString(frame.Event)
			b.WriteByte('\n')
		}
		b.WriteString("data: ")
		b.WriteString(strings.ReplaceAll(frame.Data, "\n", "\ndata: "))
		b.WriteString("\n\n")
	}
	return truncateHeadTail(b.String(), headBytes, tailBytes)
}

// truncateHeadTail 保留头尾并标注丢弃字节数（0 表示未截断，调用方据此区分
// 「上游就这么短」与「被截断」）。
func truncateHeadTail(text string, headBytes, tailBytes int) string {
	if len(text) <= headBytes+tailBytes {
		return text
	}
	head := snapToRuneBoundary(text[:headBytes], 1)
	tail := snapToRuneBoundary(text[len(text)-tailBytes:], -1)
	dropped := len(text) - len(head) - len(tail)

	var b strings.Builder
	b.Grow(len(head) + len(tail) + 96)
	b.WriteString(head)
	b.WriteString(common.SSETruncatedMarker(dropped))
	b.WriteString(tail)
	return b.String()
}

// snapToRuneBoundary 把按字节切出来的边界挪回 UTF-8 字符边界，避免在中文回答中间截出
// 半个字符（前端渲染成替换符，反而看不清截断位置）。direction>0 处理头段末尾、<0 处理尾段开头。
func snapToRuneBoundary(s string, direction int) string {
	if direction < 0 {
		for len(s) > 0 && !utf8.RuneStart(s[0]) {
			s = s[1:]
		}
		return s
	}
	for len(s) > 0 && !utf8.RuneStart(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	// 末字节已是字符首字节，但它后面的续字节被切掉了 → 半个字符，一并丢弃。
	if len(s) > 0 {
		if r, size := utf8.DecodeLastRuneInString(s); r == utf8.RuneError && size <= 1 {
			s = s[:len(s)-1]
		}
	}
	return s
}

// truncateForDetail 把上游响应体压进人读细节的短预览（截断标记与 common 的 512B 口径
// 无关，这里是探针自己的 Detail 字段，长度自成一体）。
func truncateForDetail(body []byte, maxBytes int) string {
	text := strings.TrimSpace(string(body))
	if len(text) <= maxBytes {
		return text
	}
	// 按字节切会截断 UTF-8 字符（Detail 要进 JSON 回包，半个字符会渲染成替换符）。
	return snapToRuneBoundary(text[:maxBytes], 1) + "...(截断)"
}
