package credprobe

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/providers"
	"github.com/tidwall/gjson"
)

// sseBody 把若干「event 名 + data 载荷」拼成 SSE 文本（空 event 名只写 data 行，
// 即 OpenAI Chat 的形状）。每帧后跟空行；最后一帧由 withTrailingBlankLine 决定。
func sseBody(frames []SSEFrame, trailingBlankLine bool) string {
	var b strings.Builder
	for _, f := range frames {
		if f.Event != "" {
			fmt.Fprintf(&b, "event: %s\n", f.Event)
		}
		for _, line := range strings.Split(f.Data, "\n") {
			fmt.Fprintf(&b, "data: %s\n", line)
		}
		b.WriteString("\n")
	}
	if !trailingBlankLine {
		// 去掉末尾空行，模拟「上游靠 EOF 收流、不发分隔空行」
		return strings.TrimSuffix(b.String(), "\n")
	}
	return b.String()
}

func sseServer(contentType, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, body)
	}))
}

func runStreamProbe(t *testing.T, url string, timeout time.Duration) StreamProbeResult {
	t.Helper()
	return runStreamProbeWith(t, url, timeout, nil)
}

func runStreamProbeWith(t *testing.T, url string, timeout time.Duration, capture *probeRequestCapture) StreamProbeResult {
	t.Helper()
	return RunStreamProbe(StreamProbeRequest{
		Ctx:     t.Context(),
		Client:  probeFakeClient{url: url, capture: capture},
		Timeout: timeout,
		Headers: http.Header{"Authorization": []string{"Bearer k"}},
		Model:   "probe-model",
		Body:    []byte(`{"model":"probe-model","messages":[{"role":"user","content":"hi"}]}`),
	})
}

// TestRunStreamProbe_InjectsStreamFlagAndAcceptHeader 锁探针的两个「请求侧」职责：
// 注入 stream:true（测试体本身是静态字面量，不为流式复制一份）与带 Accept 头。
// 刻意断言原键仍在：sjson 是改不是换，丢掉 messages 会让所有流式测试恒判上游异常。
func TestRunStreamProbe_InjectsStreamFlagAndAcceptHeader(t *testing.T) {
	capture := &probeRequestCapture{}
	srv := sseServer("text/event-stream", sseBody([]SSEFrame{{Data: `{"choices":[{"delta":{"content":"ok"}}]}`}, {Data: "[DONE]"}}, true))
	t.Cleanup(srv.Close)

	out := runStreamProbeWith(t, srv.URL, 2*time.Second, capture)
	if !out.Passed {
		t.Fatalf("探针应通过; got %+v", out)
	}
	if !gjson.GetBytes(capture.body, "stream").Bool() {
		t.Fatalf("body = %s, want 含 stream:true", capture.body)
	}
	if !gjson.GetBytes(capture.body, "messages").IsArray() {
		t.Fatalf("body = %s, want 保留原 messages", capture.body)
	}
	if got := capture.header.Get("Accept"); got != "text/event-stream" {
		t.Fatalf("Accept = %q, want text/event-stream", got)
	}
}

// TestRunStreamProbe_FramesOpenAIChat 锁 OpenAI Chat（无 event: 行）的分帧与无损载荷。
func TestRunStreamProbe_FramesOpenAIChat(t *testing.T) {
	frames := []SSEFrame{
		{Data: `{"choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`},
		{Data: `{"choices":[{"index":0,"delta":{"content":" world"},"finish_reason":"stop"}]}`},
		{Data: "[DONE]"},
	}
	srv := sseServer("text/event-stream", sseBody(frames, true))
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 2*time.Second)
	if !out.Passed {
		t.Fatalf("探针应通过; got %+v", out)
	}
	if len(out.Frames) != len(frames) {
		t.Fatalf("frames = %d, want %d (%+v)", len(out.Frames), len(frames), out.Frames)
	}
	for i, want := range frames {
		if out.Frames[i].Data != want.Data {
			t.Fatalf("frames[%d].Data = %q, want %q", i, out.Frames[i].Data, want.Data)
		}
		if out.Frames[i].Event != "" {
			t.Fatalf("frames[%d].Event = %q, want 空", i, out.Frames[i].Event)
		}
	}
	if !strings.Contains(out.RawOutput, "[DONE]") {
		t.Fatalf("RawOutput 应含原始 SSE; got %q", out.RawOutput)
	}
}

// TestRunStreamProbe_EventNamesAndEOF 锁带 event: 行的协议（Anthropic / Responses）：
// 事件名要保留（协议语义靠它）、`:` 注释行要忽略、多行 data 要合成一帧、
// 末帧无分隔空行时靠 EOF 也要 flush（否则 message_stop 会丢，严格门误判缺终止事件）。
func TestRunStreamProbe_EventNamesAndEOF(t *testing.T) {
	raw := "event: message_start\n" +
		"data: {\"type\":\"message_start\"}\n" +
		"\n" +
		": keep-alive\n" +
		"\n" +
		"event: ping\ndata: {\"type\": \"ping\"}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\n" +
		"data:  \"index\":0}\n" +
		"\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n"

	srv := sseServer("text/event-stream; charset=utf-8", raw)
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 2*time.Second)
	if !out.Passed {
		t.Fatalf("探针应通过（charset 参数也算 event-stream）; got %+v", out)
	}
	wantEvents := []string{"message_start", "ping", "content_block_delta", "message_stop"}
	if len(out.Frames) != len(wantEvents) {
		t.Fatalf("frames = %d, want %d（注释行/空帧没滤掉？）\n%+v", len(out.Frames), len(wantEvents), out.Frames)
	}
	for i, want := range wantEvents {
		if out.Frames[i].Event != want {
			t.Fatalf("frames[%d].Event = %q, want %q", i, out.Frames[i].Event, want)
		}
	}
	if got := out.Frames[2].Data; got != "{\"type\":\"content_block_delta\",\n \"index\":0}" {
		t.Fatalf("多行 data 未按 SSE 以 \\n 合并: %q", got)
	}
}

// TestRunStreamProbe_UpstreamIgnoresStreamIsNotSSE 是本切片的核心回归：上游吃掉 stream
// 参数、回 200 + application/json 时，必须归成「上游不支持流式」的明确结论，
// 而不是把 JSON 当 SSE 读出一个空帧流（.local/find-issues 里 ReAct panic 的同一根因）。
func TestRunStreamProbe_UpstreamIgnoresStreamIsNotSSE(t *testing.T) {
	body := `{"id":"cmpl-1","choices":[{"message":{"content":"ignored stream"}}]}`
	srv := sseServer("application/json", body)
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 2*time.Second)
	if out.Passed {
		t.Fatal("非 SSE 响应不应判通过")
	}
	if out.Summary != ProbeSummaryStreamUnsupported {
		t.Fatalf("Summary = %q, want %q", out.Summary, ProbeSummaryStreamUnsupported)
	}
	if out.ErrorType != common.ErrorTypeValidation {
		t.Fatalf("ErrorType = %q, want validation（能力缺失不是网络故障）", out.ErrorType)
	}
	if !strings.Contains(out.Detail, "application/json") {
		t.Fatalf("Detail 应带上实际 Content-Type 供排查; got %q", out.Detail)
	}
	if !strings.Contains(out.Detail, "ignored stream") {
		t.Fatalf("Detail 应带上响应体预览; got %q", out.Detail)
	}
	if out.RawOutput != body {
		t.Fatalf("RawOutput = %q, want 上游原文 %q", out.RawOutput, body)
	}
	if len(out.Frames) != 0 {
		t.Fatalf("失败路径不应交出帧; got %+v", out.Frames)
	}
}

// TestRunStreamProbe_MissingContentTypeIsNotSSE 上游连 Content-Type 都不给时不能当成流式。
func TestRunStreamProbe_MissingContentTypeIsNotSSE(t *testing.T) {
	srv := sseServer("", `{"ok":true}`)
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 2*time.Second)
	if out.Passed || out.Summary != ProbeSummaryStreamUnsupported {
		t.Fatalf("结果 = %+v, want 不支持流式", out)
	}
}

// TestRunStreamProbe_Non2xxKeepsUpstreamErrorClassification 流式的非 2xx 走与非流式
// 完全相同的状态码归类与细节口径（探针的失败归类必须只有一份）。
func TestRunStreamProbe_Non2xxKeepsUpstreamErrorClassification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
	}))
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 2*time.Second)
	if out.Passed {
		t.Fatal("401 不应判通过")
	}
	if out.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want 401", out.StatusCode)
	}
	if out.ErrorType != common.ErrorTypeAuth || out.Summary != ProbeSummaryUpstreamError {
		t.Fatalf("归类 = %s/%s, want auth/%s", out.ErrorType, out.Summary, ProbeSummaryUpstreamError)
	}
	if !strings.Contains(out.Detail, "invalid api key") {
		t.Fatalf("Detail = %q, want 解析出的 error.message", out.Detail)
	}
	if out.RawOutput != `{"error":{"message":"invalid api key"}}` {
		t.Fatalf("RawOutput = %q, want 错误体原文", out.RawOutput)
	}
}

// TestRunStreamProbe_EmptyStreamStillPassesProbe 「有没有内容」是端点的领域判定，
// 不是探针结论——探针只负责「拿到合规 SSE 流并读完」。锁这条边界，否则端点的
// 「无内容增量」门永远不会被触发，流式空响应会被探针层静默判成通过。
func TestRunStreamProbe_EmptyStreamStillPassesProbe(t *testing.T) {
	srv := sseServer("text/event-stream", "")
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 2*time.Second)
	if !out.Passed {
		t.Fatalf("空流应过探针层（由端点判失败）; got %+v", out)
	}
	if len(out.Frames) != 0 {
		t.Fatalf("frames = %d, want 0", len(out.Frames))
	}
}

// TestRunStreamProbe_SlowDripIsTimeoutNotNetwork 流式读体中途 deadline 到期：
// 与非流式读体路径同口径归 timeout（否则「慢上游稳定归 timeout」的契约从流式路径漏回 network）。
func TestRunStreamProbe_SlowDripIsTimeoutNotNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				if _, err := io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n"); err != nil {
					return
				}
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		}
	}))
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 150*time.Millisecond)
	if out.Passed {
		t.Fatal("滴流超期不应判通过")
	}
	if out.ErrorType != common.ErrorTypeTimeout || out.Summary != ProbeSummaryReadResponse {
		t.Fatalf("归类 = %s/%s, want timeout/%s", out.ErrorType, out.Summary, ProbeSummaryReadResponse)
	}
}

// TestRunStreamProbe_UnreachableUpstreamIsNetwork 连不上 → network（与非流式探针同口径）。
func TestRunStreamProbe_UnreachableUpstreamIsNetwork(t *testing.T) {
	srv := sseServer("text/event-stream", "")
	url := srv.URL
	srv.Close()

	out := runStreamProbe(t, url, 2*time.Second)
	if out.Passed || out.ErrorType != common.ErrorTypeNetwork || out.Summary != ProbeSummaryConnect {
		t.Fatalf("结果 = %+v, want network/%s", out, ProbeSummaryConnect)
	}
}

// TestRunStreamProbe_BuildReqFailureShortCircuits BuildReq 失败零上游请求。
func TestRunStreamProbe_BuildReqFailureShortCircuits(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits++ }))
	t.Cleanup(srv.Close)

	out := RunStreamProbe(StreamProbeRequest{
		Ctx:     t.Context(),
		Client:  probeFakeClient{url: srv.URL, buildReqErr: fmt.Errorf("boom")},
		Timeout: time.Second,
		Body:    []byte(`{"model":"m"}`),
	})
	if out.Passed || out.Summary != ProbeSummaryBuildRequest || out.Detail != "boom" {
		t.Fatalf("结果 = %+v, want %s/boom", out, ProbeSummaryBuildRequest)
	}
	if hits != 0 {
		t.Fatalf("upstream hits = %d, want 0", hits)
	}
}

// TestRunStreamProbe_oversizedStreamIsBounded 上游失控狂吐时必须按预算止损，
// 而不是把整条流读进内存（探针挂在管理端，一个坏渠道不该打爆网关进程）。
func TestRunStreamProbe_oversizedStreamIsBounded(t *testing.T) {
	chunk := `{"choices":[{"index":0,"delta":{"content":"` + strings.Repeat("x", 200) + `"}}]}` + "\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		for i := 0; i < streamProbeMaxTotalBytes/len(chunk)+10; i++ {
			if r.Context().Err() != nil {
				return
			}
			if _, err := io.WriteString(w, "data: "+chunk+"\n"); err != nil {
				return
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}))
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 10*time.Second)
	if out.Passed {
		t.Fatal("超预算流不应判通过")
	}
	if out.Summary != ProbeSummaryStreamTooLarge {
		t.Fatalf("Summary = %q, want %q", out.Summary, ProbeSummaryStreamTooLarge)
	}
	if out.ErrorType != common.ErrorTypeProvider {
		t.Fatalf("ErrorType = %q, want provider", out.ErrorType)
	}
}

// TestRunStreamProbe_RawOutputTruncatedFramesLossless 是「有损回显 / 无损判定」双轨的
// 机械证明：RawOutput 被截断，但判定用的 Frames 必须一条不少——否则 extractor 会
// 在被截断的文本上折叠，把「流被切断」伪装成「内容完整」。
func TestRunStreamProbe_RawOutputTruncatedFramesLossless(t *testing.T) {
	const frameCount = 3000
	frames := make([]SSEFrame, 0, frameCount+1)
	for i := 0; i < frameCount; i++ {
		// 每帧 ~180B：3000 帧 ≈ 540KB，稳超 RawOutput 的 256KB 预算，同时远低于
		// streamProbeMaxTotalBytes（截断预算必须先于总预算生效，否则测不到这件事）。
		frames = append(frames, SSEFrame{Data: fmt.Sprintf(
			`{"choices":[{"index":0,"delta":{"content":"%06d%s"}}]}`, i, strings.Repeat("y", 120))})
	}
	frames = append(frames, SSEFrame{Data: "[DONE]"})
	body := sseBody(frames, true)

	srv := sseServer("text/event-stream", body)
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 10*time.Second)
	if !out.Passed {
		t.Fatalf("探针应通过; got %+v", out)
	}
	if len(out.Frames) != len(frames) {
		t.Fatalf("frames = %d, want %d（RawOutput 截断不得牵连判定输入）", len(out.Frames), len(frames))
	}
	if len(out.RawOutput) >= len(body) {
		t.Fatalf("RawOutput 未被截断（%d vs %d），截断预算失效", len(out.RawOutput), len(body))
	}
	if !strings.Contains(out.RawOutput, "内容过长已截断") {
		t.Fatalf("RawOutput 缺截断标记: %q", out.RawOutput)
	}
	// 尾段必须保住：[DONE] 在末尾，头截式截断会丢掉最有诊断价值的收尾。
	if !strings.HasSuffix(strings.TrimSpace(out.RawOutput), "data: [DONE]") {
		t.Fatalf("RawOutput 尾段丢了 [DONE]: %q", out.RawOutput[len(out.RawOutput)-120:])
	}
}

// TestRunStreamProbe_TimingStats 锁两个耗时口径存在且单调（前端诊断块直接展示它们）。
func TestRunStreamProbe_TimingStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(30 * time.Millisecond)
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 2*time.Second)
	if !out.Passed {
		t.Fatalf("探针应通过; got %+v", out)
	}
	if out.FirstFrameMS < 20 {
		t.Fatalf("FirstFrameMS = %d, want ≥20（首个数据帧之前的耗时没被记录？）", out.FirstFrameMS)
	}
	if out.DurationMS < out.FirstFrameMS {
		t.Fatalf("DurationMS = %d < FirstFrameMS = %d", out.DurationMS, out.FirstFrameMS)
	}
}

// TestRunStreamProbe_JSONFramesAreNotReformatted 帧载荷必须逐字节原样交出：
// 折叠层依赖 delta 字段名，探针若「顺手」反序列化再序列化一次，非 ASCII 转义与
// 键序变化会静默改变折叠结果（多一道无收益的失真风险）。
func TestRunStreamProbe_JSONFramesAreNotReformatted(t *testing.T) {
	payload := `{"choices":[{"index":0,"delta":{"content":"红 色\u4e2d"}}],"x": 1}`
	srv := sseServer("text/event-stream", "data: "+payload+"\n\ndata: [DONE]\n\n")
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 2*time.Second)
	if !out.Passed {
		t.Fatalf("探针应通过; got %+v", out)
	}
	if out.Frames[0].Data != payload {
		t.Fatalf("帧载荷被改写:\n got %q\nwant %q", out.Frames[0].Data, payload)
	}
	if !json.Valid([]byte(out.Frames[0].Data)) {
		t.Fatal("帧载荷不再是合法 JSON")
	}
}

// TestRunStreamProbe_OversizedSingleLineFailsRead 单帧超过行预算（1 MiB）时是 scanner
// 报错而非静默丢帧：归「读取上游响应失败」，绝不能把丢掉该帧的残缺流交给端点判通过。
func TestRunStreamProbe_OversizedSingleLineFailsRead(t *testing.T) {
	var b strings.Builder
	b.WriteString("data: {\"choices\":[{\"delta\":{\"content\":\"")
	for i := 0; i < streamProbeMaxLineBytes/1024+8; i++ {
		b.WriteString(strings.Repeat("z", 1024))
	}
	b.WriteString("\"}}]}\n\n")

	srv := sseServer("text/event-stream", b.String())
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 5*time.Second)
	if out.Passed {
		t.Fatal("超长单行流不应判通过")
	}
	if out.Summary != ProbeSummaryReadResponse {
		t.Fatalf("Summary = %q, want %q", out.Summary, ProbeSummaryReadResponse)
	}
}

// TestRunStreamProbe_DropsUpstreamAcceptEncoding 关联开启「透传请求头」时，浏览器的
// Accept-Encoding 会跟着发给上游，而 Go Transport 一旦看到调用方自己设了这个头就
// **不再透明解压**——探针会对着 gzip 字节流分帧，把好的流式渠道判成「无内容增量」。
func TestRunStreamProbe_DropsUpstreamAcceptEncoding(t *testing.T) {
	capture := &probeRequestCapture{}
	srv := sseServer("text/event-stream", sseBody([]SSEFrame{{Data: `{"choices":[{"delta":{"content":"ok"}}]}`}, {Data: "[DONE]"}}, true))
	t.Cleanup(srv.Close)

	out := RunStreamProbe(StreamProbeRequest{
		Ctx:     t.Context(),
		Client:  probeFakeClient{url: srv.URL, capture: capture},
		Timeout: 2 * time.Second,
		Headers: http.Header{"Accept-Encoding": []string{"gzip, deflate, br"}},
		Model:   "probe-model",
		Body:    []byte(`{"model":"probe-model"}`),
	})
	if !out.Passed {
		t.Fatalf("探针应通过; got %+v", out)
	}
	if got := capture.header.Get("Accept-Encoding"); got != "" {
		t.Fatalf("Accept-Encoding = %q, want 被剥掉（交给 Transport 自己处理解压）", got)
	}
}

// TestRunStreamProbe_FrameBudgetExceeded 字节预算封不住**判定输入**：每条帧只占 9 字节的
// 畸形上游能在 4 MiB 内塞进几十万帧，Frames 与重建文本一起把内存放大到几十 MB。
// 帧数预算是这道缺口的唯一封挡，超限必须按上游异常收敛而不是继续攒。
func TestRunStreamProbe_FrameBudgetExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		for i := 0; i < streamProbeMaxFrames+500; i++ {
			if r.Context().Err() != nil {
				return
			}
			if _, err := io.WriteString(w, "data: x\n\n"); err != nil {
				return
			}
			if i%500 == 0 {
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		}
	}))
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 10*time.Second)
	if out.Passed {
		t.Fatal("超帧数预算不应判通过")
	}
	if out.Summary != ProbeSummaryStreamTooLarge {
		t.Fatalf("Summary = %q, want %q", out.Summary, ProbeSummaryStreamTooLarge)
	}
	if len(out.Frames) > streamProbeMaxFrames {
		t.Fatalf("Frames = %d, want 不超预算 %d", len(out.Frames), streamProbeMaxFrames)
	}
}

// 探针不能依赖 providers 的编译期具体类型（依赖方向纪律），此处仅断言接口形状使用。
var _ providers.Provider = probeFakeClient{}
