package credprobe

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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

// SSEFrame 一个已分帧的 SSE 事件。Event 取自 event: 行（OpenAI Chat 通常为空，
// Anthropic / OpenAI Responses 有值）；Data 是该事件内所有 data: 行按 SSE 规范以 \n 合并的载荷。
//
// 载荷逐字节原样保留、不做反序列化再序列化：折叠层依赖 delta 字段名，任何「顺手规范化」
// 都会静默改变折叠结果。
type SSEFrame struct {
	Event string
	Data  string
}

// ProbeSummaryStreamUnsupported 上游 2xx 但没按 SSE 回（含吃掉 stream 参数直接回 JSON 的
// 兼容网关）：是能力缺失而非网络故障，故 error_type 归 validation。
// 这条文案同时是管理端 ReAct 测试 panic 登记项的根因诊断入口。
const (
	ProbeSummaryStreamUnsupported = "上游不支持流式"
	ProbeSummaryStreamTooLarge    = "流式响应超出预算"
)

const sseContentType = "text/event-stream"

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
	Timeout time.Duration // 同 RunCompletionProbe：ctx deadline 口径，≤0 兜底 DefaultCompletionTimeout
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
// Do → 非 2xx 归类 → Content-Type 硬门 → 逐帧读取 → 耗时统计。
func RunStreamProbe(req StreamProbeRequest) (out StreamProbeResult) {
	if req.Ctx == nil {
		req.Ctx = context.Background()
	}
	if req.Timeout <= 0 {
		req.Timeout = DefaultCompletionTimeout
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
	headers.Set("Accept", sseContentType)
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

	httpClient := providers.GetClientWithProxy(req.Timeout+CompletionTransportBuffer, req.Client.GetProxy())
	res, err := httpClient.Do(httpReq)
	if err != nil {
		return StreamProbeResult{ErrorType: classifyStreamDoError(err), Summary: ProbeSummaryConnect, Detail: err.Error()}
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
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

	// Content-Type 硬门放在 2xx 之后：非 2xx 无论什么形状都是「上游回了错误」，
	// 归状态码更有诊断价值；只有 2xx 时才需要区分「回了内容但不是流」。
	if !isEventStreamContentType(res.Header.Get("Content-Type")) {
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

	frames, firstFrameMS, overBudget, readErr := readSSEFrames(res.Body, start)
	if overBudget != nil {
		return StreamProbeResult{
			StatusCode:   res.StatusCode,
			ErrorType:    common.ErrorTypeProvider,
			Summary:      ProbeSummaryStreamTooLarge,
			Detail:       fmt.Sprintf("已中止读取（%v，已读到 %d 帧）", overBudget, len(frames)),
			RawOutput:    renderSSEText(frames, streamProbeRawHeadBytes, streamProbeRawTailBytes),
			Framed:       true,
			FirstFrameMS: firstFrameMS,
		}
	}
	if readErr != nil {
		return StreamProbeResult{
			StatusCode:   res.StatusCode,
			ErrorType:    classifyStreamDoError(readErr),
			Summary:      ProbeSummaryReadResponse,
			Detail:       readErr.Error(),
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

// isEventStreamContentType 判定「上游是否真的回了 SSE」。**判据必须与转发链路一致**：
// service/transform/transform_provider_response.go 与 service/chat/chat_attempt_log.go
// 都用 Contains，探针若用 HasPrefix 就会出现「网关按流处理、测试判它不支持流」的分裂结论。
// 参数（charset 等）出现在媒体类型之前/之后都算流式，参数本身由上游负责。
func isEventStreamContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), sseContentType)
}

// readSSEFrames 按 SSE 规范分帧读取。切帧语义与 service/transform/streaming 的网关读流
// 保持一致（空行交付、event: 记名、多行 data 以 \n 合并、: 开头为注释），因为「网关能解析
// 的流」与「测试能解析的流」必须是同一条判据——否则测试判通过的流在真实转发链路上仍可能崩。
//
// 与网关那份的一处有意差异：这里去掉 data: 后的**一个**前导空格（SSE 规范），而非
// TrimSpace——行首空白在 JSON 里虽无意义，但 TrimSpace 会把上游原样该保留的内容改掉，
// 而本函数的输出要喂给逐字节比对帧的折叠层。
//
// overBudget 与 err 互斥：前者是「上游超预算，主动止损」，后者是读流失败。
// 无副作用、不解析 JSON：纯分帧。
func readSSEFrames(body io.Reader, start time.Time) (frames []SSEFrame, firstFrameMS int64, overBudget error, err error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), streamProbeMaxLineBytes)

	var eventName string
	var dataLines []string
	totalBytes := 0
	firstSeen := false

	flush := func() {
		if len(dataLines) == 0 {
			eventName = ""
			return
		}
		frames = append(frames, SSEFrame{Event: eventName, Data: strings.Join(dataLines, "\n")})
		if !firstSeen {
			firstSeen = true
			firstFrameMS = time.Since(start).Milliseconds()
		}
		eventName = ""
		dataLines = dataLines[:0]
	}

	for scanner.Scan() {
		totalBytes += len(scanner.Bytes()) + 1 // +1 计入换行符，与「消费字节数」同口径
		if totalBytes > streamProbeMaxTotalBytes {
			return frames, firstFrameMS, fmt.Errorf("字节数超过 %d 预算", streamProbeMaxTotalBytes), nil
		}
		if len(frames) >= streamProbeMaxFrames {
			return frames, firstFrameMS, fmt.Errorf("帧数超过 %d 预算", streamProbeMaxFrames), nil
		}

		line := strings.TrimRight(scanner.Text(), "\r")
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, ":"):
			// SSE 注释行（Anthropic 的 ping/keep-alive 常见）：不成帧。
		case strings.HasPrefix(line, "data:"):
			value := strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			if value == "" {
				continue
			}
			dataLines = append(dataLines, value)
		}
		// 其余行（无冒号的裸行、id:/retry: 等字段）对判定无信息量，忽略。
	}
	if err = scanner.Err(); err != nil {
		return frames, firstFrameMS, nil, err
	}
	// EOF 且末帧没有分隔空行：仍要交付，否则 message_stop 这类收尾事件会凭空消失，
	// 端点会把「正常结束的流」误判成「缺终止事件」。
	flush()
	return frames, firstFrameMS, nil, nil
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
	fmt.Fprintf(&b, "\n...[llmux truncated %d bytes of raw SSE middle section, head/tail preserved]...\n\n", dropped)
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
