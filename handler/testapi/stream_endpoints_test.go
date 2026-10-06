package testapi

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/tidwall/gjson"
)

// 本文件锁「流式模式」的端点契约：模式只加键、不改既有判定，且回包键名是前后端契约
// （断言打在响应体 JSON 上，而不是读回的 Go 字段——见 AGENTS 编码规范）。

// streamThinkingUpstream 对流式请求回「先推理、后正文」的 OpenAI Chat SSE，
// 对非流式请求回同内容的 JSON——一把覆盖两种模式的同一条判定路径。
func streamThinkingUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if strings.Contains(string(requested), `"stream":true`) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"2^10=1024"}}]}`+"\n\n")
			_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{"reasoning_content":"，减 8 得 1016"},"finish_reason":"stop"}]}`+"\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"1016","reasoning_content":"2^10=1024，减 8 得 1016"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// jsonOnlyUpstream 模拟「吃掉 stream 参数、仍回 JSON」的兼容网关：200 + application/json。
// 这正是 .local/find-issues 里让 ReAct 测试 panic 的那类上游。
func jsonOnlyUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"我看不见流"},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// truncatedStreamUpstream 回一条有内容增量但没有 [DONE] 的流（上游中途切断）。
func truncatedStreamUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `data: {"choices":[{"index":0,"delta":{"reasoning_content":"只吐了一半"}}]}`+"\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// sseUpstream 回给定 SSE 文本的假上游（内容各用例自定义，其余同 streamThinkingUpstream）。
func sseUpstream(t *testing.T, sse string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sse)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// seedThinkingAssociation 建一条指向给定上游的 openai 关联链路。
func seedThinkingAssociation(t *testing.T, upstreamURL string) uint {
	t.Helper()
	cipher := setupTestAPICipher(t)
	provider := seedProviderRow(t, "p-stream", "openai")
	seedEndpointRow(t, provider.ID, "openai", upstreamURL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "流式组", "", "sk-stream")
	return seedAssociation(t, provider.ID, 1, "gpt-4.1").ID
}

func callAssociationEndpoint(t *testing.T, handler gin.HandlerFunc, route string, associationID uint, query string) *httptest.ResponseRecorder {
	t.Helper()
	path := fmt.Sprintf("/api/%s/%d", strings.TrimSuffix(route, "/"), associationID)
	if query != "" {
		path += "?" + query
	}
	c, w := testsupport.NewTestContext("GET", path)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(associationID), 10)}}
	handler(c)
	return w
}

// TestThinkingEndpoint_StreamMode 流式模式必须走「折叠 → 既有 extractor」这条路：
// 上游只在 delta 里给推理内容，非流式提取器原本取不到，折叠后取到即证明复用成立。
func TestThinkingEndpoint_StreamMode(t *testing.T) {
	testsupport.InitTestDB(t)
	associationID := seedThinkingAssociation(t, streamThinkingUpstream(t).URL)

	w := callAssociationEndpoint(t, TestThinkingHandler, "test/thinking", associationID, "stream=1")
	body := w.Body.String()

	if !gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("流式 thinking 应判通过; body=%s", body)
	}
	if got := gjson.Get(body, "data.mode").String(); got != testModeStream {
		t.Fatalf("data.mode = %q, want %q", got, testModeStream)
	}
	if got := gjson.Get(body, "data.thinking_text").String(); got != "2^10=1024，减 8 得 1016" {
		t.Fatalf("data.thinking_text = %q, want delta 折叠后的完整推理", got)
	}
	if got := gjson.Get(body, "data.stream.terminated_by").String(); got != "[DONE]" {
		t.Fatalf("data.stream.terminated_by = %q", got)
	}
	if gjson.Get(body, "data.stream.frames").Int() < 2 {
		t.Fatalf("data.stream.frames = %v, want ≥2; body=%s", gjson.Get(body, "data.stream.frames").Value(), body)
	}
	if !strings.Contains(gjson.Get(body, "data.raw_output").String(), "data: [DONE]") {
		t.Fatalf("流式下的 raw_output 必须是 SSE 原文（唯一排查手段）; got %q", gjson.Get(body, "data.raw_output").String())
	}
	if !strings.Contains(gjson.Get(body, "data.message").String(), "流式") {
		t.Fatalf("data.message = %q, want 标明模式", gjson.Get(body, "data.message").String())
	}
}

// TestThinkingEndpoint_NonStreamModeKeepsContract 不带 stream 参数时形状必须与改造前一致：
// mode 键新在（加键不改键），但不该冒出 stream 诊断块。
func TestThinkingEndpoint_NonStreamModeKeepsContract(t *testing.T) {
	testsupport.InitTestDB(t)
	associationID := seedThinkingAssociation(t, streamThinkingUpstream(t).URL)

	w := callAssociationEndpoint(t, TestThinkingHandler, "test/thinking", associationID, "")
	body := w.Body.String()

	if !gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("非流式 thinking 应判通过; body=%s", body)
	}
	if got := gjson.Get(body, "data.mode").String(); got != testModeNonStream {
		t.Fatalf("data.mode = %q, want %q", got, testModeNonStream)
	}
	if gjson.Get(body, "data.stream").Exists() {
		t.Fatalf("非流式不应有 data.stream 诊断块: %s", body)
	}
	if got := gjson.Get(body, "data.thinking_text").String(); got != "2^10=1024，减 8 得 1016" {
		t.Fatalf("data.thinking_text = %q", got)
	}
}

// TestThinkingEndpoint_UpstreamIgnoresStream 上游回 JSON 时必须报「不支持流式」而不是
// 「模型没返回推理内容」：前者是渠道缺陷、后者是能力结论，混起来会把人引到错误方向。
func TestThinkingEndpoint_UpstreamIgnoresStream(t *testing.T) {
	testsupport.InitTestDB(t)
	associationID := seedThinkingAssociation(t, jsonOnlyUpstream(t).URL)

	w := callAssociationEndpoint(t, TestThinkingHandler, "test/thinking", associationID, "stream=1")
	body := w.Body.String()

	if gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("非 SSE 上游不应判通过: %s", body)
	}
	if got := gjson.Get(body, "data.error").String(); !strings.Contains(got, "上游不支持流式") {
		t.Fatalf("data.error = %q, want 含「上游不支持流式」", got)
	}
	if got := gjson.Get(body, "data.error_type").String(); got != "validation" {
		t.Fatalf("data.error_type = %q, want validation", got)
	}
	if got := gjson.Get(body, "data.mode").String(); got != testModeStream {
		t.Fatalf("失败回包也要标明模式: %q", got)
	}
	// 探针在分帧之前就失败 → 没有终止事件可报，stream 诊断块必须整个缺席而非空对象。
	if gjson.Get(body, "data.stream").Exists() {
		t.Fatalf("未进入分帧阶段不应有 data.stream: %s", body)
	}
	if !strings.Contains(gjson.Get(body, "data.raw_output").String(), "我看不见流") {
		t.Fatalf("data.raw_output 应带上游原文供排查; got %q", gjson.Get(body, "data.raw_output").String())
	}
}

// TestThinkingEndpoint_TruncatedStreamIsFailure 缺终止事件的流必须判失败：
// 「无终止标记」意味着下游无法区分正常结束与被切断，放宽就等于放过这类渠道。
// 同时锁「已收到的帧不能丢」——没有它，用户分不清「一帧都没有」与「收了 N 帧后被切断」。
func TestThinkingEndpoint_TruncatedStreamIsFailure(t *testing.T) {
	testsupport.InitTestDB(t)
	associationID := seedThinkingAssociation(t, truncatedStreamUpstream(t).URL)

	w := callAssociationEndpoint(t, TestThinkingHandler, "test/thinking", associationID, "stream=1")
	body := w.Body.String()

	if gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("无终止事件的流不应判通过: %s", body)
	}
	if got := gjson.Get(body, "data.error").String(); !strings.Contains(got, "未收到终止事件") {
		t.Fatalf("data.error = %q, want 含「未收到终止事件」", got)
	}
	if got := gjson.Get(body, "data.stream.terminated_by").String(); got != "none" {
		t.Fatalf("data.stream.terminated_by = %q, want none（已进分帧阶段，该有诊断块）", got)
	}
	if got := gjson.Get(body, "data.stream.frames").Int(); got < 1 {
		t.Fatalf("data.stream.frames = %d, want ≥1（被切断前收到的帧不能丢）", got)
	}
}

// TestConnectivityEndpoint_StreamModeIsStreamSupportProbe 连通性的流式模式 = 纯流式支持探测：
// 没有领域判定，三门过即通过。这是「该渠道到底能不能流」的最低成本答案。
func TestConnectivityEndpoint_StreamModeIsStreamSupportProbe(t *testing.T) {
	testsupport.InitTestDB(t)
	associationID := seedThinkingAssociation(t, streamThinkingUpstream(t).URL)

	w := callAssociationEndpoint(t, ProviderTestHandler, "test", associationID, "stream=1")
	body := w.Body.String()

	if !gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("流式连通性应判通过; body=%s", body)
	}
	if got := gjson.Get(body, "data.mode").String(); got != testModeStream {
		t.Fatalf("data.mode = %q", got)
	}
	if got := gjson.Get(body, "data.stream.terminated_by").String(); got != "[DONE]" {
		t.Fatalf("data.stream.terminated_by = %q", got)
	}
	// 既有契约：上游原文在 message（前端 ExpandableError 直接吃它），不是 raw_output。
	if !strings.Contains(gjson.Get(body, "message").String(), "[DONE]") {
		t.Fatalf("message = %q, want SSE 原文", gjson.Get(body, "message").String())
	}
}

// TestVisionAndStructuredEndpoint_StreamMode 另两个能力端点的流式正向路径也要有端点级证据：
// 严格三门在公共流里统一执行，但「折叠产物能被各自的 extractor 判定通过」这件事
// 只在 fold 单测里证过，这里补上从 HTTP 端到端的一条。
func TestVisionAndStructuredEndpoint_StreamMode(t *testing.T) {
	cases := []struct {
		name     string
		handler  gin.HandlerFunc
		route    string
		sse      string
		wantKey  string
		wantText string
	}{
		{
			name:     "vision",
			handler:  TestVisionHandler,
			route:    "test/vision",
			sse:      `data: {"choices":[{"index":0,"delta":{"content":"The image is "}}]}` + "\n\n" + `data: {"choices":[{"index":0,"delta":{"content":"red."},"finish_reason":"stop"}]}` + "\n\n" + "data: [DONE]\n\n",
			wantKey:  "data.vision_text",
			wantText: "The image is red.",
		},
		{
			name:     "structured_output",
			handler:  TestStructuredOutputHandler,
			route:    "test/structured-output",
			sse:      `data: {"choices":[{"index":0,"delta":{"content":"{\"language\":\"zh\",\"version\":1,"}}]}` + "\n\n" + `data: {"choices":[{"index":0,"delta":{"content":"\"features\":[\"a\"],\"score\":1,\"passed\":true}"},"finish_reason":"stop"}]}` + "\n\n" + "data: [DONE]\n\n",
			wantKey:  "data.parsed.language",
			wantText: "zh",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testsupport.InitTestDB(t)
			srv := sseUpstream(t, tc.sse)
			associationID := seedThinkingAssociation(t, srv.URL)

			w := callAssociationEndpoint(t, tc.handler, tc.route, associationID, "stream=1")
			body := w.Body.String()

			if !gjson.Get(body, "data.passed").Bool() {
				t.Fatalf("流式 %s 应判通过; body=%s", tc.name, body)
			}
			if got := gjson.Get(body, tc.wantKey).String(); got != tc.wantText {
				t.Fatalf("%s = %q, want %q", tc.wantKey, got, tc.wantText)
			}
			if got := gjson.Get(body, "data.stream.terminated_by").String(); got != "[DONE]" {
				t.Fatalf("data.stream.terminated_by = %q", got)
			}
		})
	}
}

// TestThinkingEndpoint_InStreamErrorIsProviderFault 上游在流里报故障（HTTP 仍 200）时，
// error_type 必须是 provider：同一故障在非流式下走状态码归类也是 provider，
// 若流式统一报 validation，用户会以为「换个模式病因就变了」。
func TestThinkingEndpoint_InStreamErrorIsProviderFault(t *testing.T) {
	testsupport.InitTestDB(t)
	srv := sseUpstream(t, `data: {"choices":[{"index":0,"delta":{"reasoning_content":"跑了一半"}}]}`+"\n\n"+
		`data: {"error":{"message":"engine_overloaded"}}`+"\n\n"+"data: [DONE]\n\n")
	associationID := seedThinkingAssociation(t, srv.URL)

	w := callAssociationEndpoint(t, TestThinkingHandler, "test/thinking", associationID, "stream=1")
	body := w.Body.String()

	if gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("流内含 error 帧不应判通过: %s", body)
	}
	if got := gjson.Get(body, "data.error_type").String(); got != "provider" {
		t.Fatalf("data.error_type = %q, want provider（流内上游故障）", got)
	}
	if got := gjson.Get(body, "data.error").String(); !strings.Contains(got, "engine_overloaded") {
		t.Fatalf("data.error = %q, want 带上游错误原文", got)
	}
}

// TestConnectivityEndpoint_NonStreamFailureCarriesRawOutput 连通性并入公共流的直接收益：
// 失败也带上游原文（此前只有 passed:false + 归类文本，排查只能靠猜）。
func TestConnectivityEndpoint_NonStreamFailureCarriesRawOutput(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)
	provider := seedProviderRow(t, "p-badkey", "openai")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"You didn't provide an API key."}}`)
	}))
	t.Cleanup(srv.Close)
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "坏 key 组", "", "sk-nope")
	associationID := seedAssociation(t, provider.ID, 1, "gpt-4.1").ID

	w := callAssociationEndpoint(t, ProviderTestHandler, "test", associationID, "")
	body := w.Body.String()

	if gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("401 不应判通过: %s", body)
	}
	if got := gjson.Get(body, "data.error_type").String(); got != "auth" {
		t.Fatalf("data.error_type = %q, want auth", got)
	}
	if got := gjson.Get(body, "data.raw_output").String(); !strings.Contains(got, "didn't provide an API key") {
		t.Fatalf("data.raw_output = %q, want 上游错误体原文", got)
	}
	if got := gjson.Get(body, "data.mode").String(); got != testModeNonStream {
		t.Fatalf("data.mode = %q", got)
	}
}
