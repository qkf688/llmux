package testapi

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/service/protocolprobe"
	"github.com/tidwall/gjson"
)

// 本文件锁「协议支持检测」端点的端到端契约：三条协议各自的 URL 解析、双模式判定、
// 错误信封指纹、以及总评里被点名的协议。全程只读——用例跑完后不产生任何配置写入。

type protocolHit struct {
	Path   string
	Model  string
	Stream bool
	Auth   string
}

// protocolUpstream 假的多协议网关：三条路径各自回**自己方言**的响应与错误信封
// （anthropic 路径回 Anthropic 错误形状、openai 系路径回 OpenAI 形状），
// 模型名是协议探针用的假名时回 404 ——这正是真实网关该有的形态。
// authMode 用于制造「某条路径不支持流式」等退化场景。
func protocolUpstream(hits *[]protocolHit, mu *sync.Mutex, openaiStreamCompliant bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		auth := r.Header.Get("Authorization")
		if auth == "" {
			auth = r.Header.Get("x-api-key")
		}
		mu.Lock()
		*hits = append(*hits, protocolHit{
			Path:   r.URL.Path,
			Model:  gjson.GetBytes(body, "model").String(),
			Stream: bytes.Contains(body, []byte(`"stream":true`)),
			Auth:   auth,
		})
		mu.Unlock()

		switch r.URL.Path {
		case "/v1/chat/completions":
			if gjson.GetBytes(body, "model").String() == protocolprobe.ProbeModelName {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":{"message":"The model does not exist","type":"invalid_request_error","param":"model","code":"model_not_found"}}`))
				return
			}
			if bytes.Contains(body, []byte(`"stream":true`)) {
				writeChatStream(w, openaiStreamCompliant)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","model":"m",
				"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
				"usage":{"prompt_tokens":5,"completion_tokens":2}}`))
		case "/v1/responses":
			if gjson.GetBytes(body, "model").String() == protocolprobe.ProbeModelName {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":{"message":"The model does not exist","type":"invalid_request_error","code":"model_not_found"}}`))
				return
			}
			if bytes.Contains(body, []byte(`"stream":true`)) {
				writeResponsesStream(w)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","status":"completed","error":null,
				"output":[{"id":"msg_1","type":"message","content":[{"type":"output_text","text":"ok"}]}],
				"usage":{"input_tokens":5,"output_tokens":2}}`))
		case "/v1/messages":
			if gjson.GetBytes(body, "model").String() == protocolprobe.ProbeModelName {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"type":"error","error":{"type":"not_found_error","message":"model: not found"},"request_id":"req_1"}`))
				return
			}
			if bytes.Contains(body, []byte(`"stream":true`)) {
				writeAnthropicStream(w)
				return
			}
			w.Header().Set("anthropic-ratelimit-requests-limit", "60")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-x",
				"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","stop_sequence":null,
				"usage":{"input_tokens":5,"output_tokens":2,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"not found","type":"invalid_request_error"}}`))
		}
	}
}

func writeChatStream(w http.ResponseWriter, compliant bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = w.Write([]byte(`data: {"id":"chunk-1","object":"chat.completion.chunk","created":0,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}` + "\n\n"))
	if compliant {
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}
	// 不合规：发了内容但没有终止事件 → 判 noncompliant（不是 failed）。
}

func writeResponsesStream(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = w.Write([]byte("event: response.created\n" + `data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}` + "\n\n"))
	_, _ = w.Write([]byte("event: response.output_text.delta\n" + `data: {"type":"response.output_text.delta","delta":"ok"}` + "\n\n"))
	_, _ = w.Write([]byte("event: response.completed\n" + `data: {"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","error":null,"output":[{"id":"msg_1","type":"message","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2}}}` + "\n\n"))
}

func writeAnthropicStream(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = w.Write([]byte("event: message_start\n" + `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"usage":{"input_tokens":5,"output_tokens":0}}}` + "\n\n"))
	_, _ = w.Write([]byte("event: content_block_start\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n"))
	_, _ = w.Write([]byte("event: content_block_delta\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}` + "\n\n"))
	_, _ = w.Write([]byte("event: content_block_stop\n" + `data: {"type":"content_block_stop","index":0}` + "\n\n"))
	_, _ = w.Write([]byte("event: message_stop\n" + `data: {"type":"message_stop"}` + "\n\n"))
}

// seedProtocolFixture 建「供应商（主类型 openai）+ openai 端点 + 号池分组凭据 + 关联」。
// 只配 openai 一条端点：另两条协议必须走「继承 Base URL」，正好覆盖 url_source=inherited 分支。
func seedProtocolFixture(t *testing.T, upstreamURL string, extraEndpoints ...string) uint {
	t.Helper()
	cipher := setupTestAPICipher(t)
	provider := seedProviderRowWithConfig(t, "p-proto", "openai",
		`{"base_url":"`+upstreamURL+`/v1","openai_auth_type":"bearer"}`)
	seedEndpointRow(t, provider.ID, "openai", upstreamURL+"/v1")
	for i := 0; i+1 < len(extraEndpoints); i += 2 {
		seedEndpointRow(t, provider.ID, extraEndpoints[i], extraEndpoints[i+1])
	}
	_, _ = seedPoolGroup(t, cipher, provider.ID, "池组", "", "sk-pool-1")
	return seedAssociation(t, provider.ID, 1, "gpt-4.1").ID
}

func callProtocolsTest(t *testing.T, associationID uint) *httptest.ResponseRecorder {
	t.Helper()
	id := strconv.FormatUint(uint64(associationID), 10)
	c, w := testsupport.NewTestContext("GET", "/api/test/protocols/"+id)
	c.Params = gin.Params{{Key: "id", Value: id}}
	TestProtocolsHandler(c)
	return w
}

var protocolWantPaths = []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"}

// 端到端主线用例：三条协议各跑双模式 + 指纹，回包矩阵与总评都要对。
// 断言分四层，缺一层就有一类缺陷能溜过去：① URL 解析（已配端点 / 继承 Base URL）；
// ② 双模式判定；③ 指纹同族与原生头；④ 总评标名与置信度。
func TestTestProtocolsHandler_MatrixVerdictAndRecommendation(t *testing.T) {
	testsupport.InitTestDB(t)
	var hits []protocolHit
	var mu sync.Mutex
	srv := httptest.NewServer(protocolUpstream(&hits, &mu, true))
	t.Cleanup(srv.Close)
	associationID := seedProtocolFixture(t, srv.URL)

	w := callProtocolsTest(t, associationID)
	if code := gjson.Get(w.Body.String(), "code").Int(); code != 200 {
		t.Fatalf("envelope code = %d, body=%s", code, w.Body.String())
	}
	if !gjson.Get(w.Body.String(), "data.passed").Bool() {
		t.Fatalf("data.passed = false, body=%s", w.Body.String())
	}
	protocols := gjson.Get(w.Body.String(), "data.protocols").Array()
	if len(protocols) != 3 {
		t.Fatalf("protocols 长度 = %d, want 3, body=%s", len(protocols), w.Body.String())
	}
	for i, want := range []string{"openai", "responses", "anthropic"} {
		entry := protocols[i]
		if got := entry.Get("protocol").String(); got != want {
			t.Fatalf("protocols[%d].protocol = %q, want %q（顺序必须固定）", i, got, want)
		}
		if !entry.Get("non_stream.passed").Bool() {
			t.Fatalf("%s 非流式未通过: %s", want, entry.Get("non_stream").Raw)
		}
		if !entry.Get("stream.passed").Bool() {
			t.Fatalf("%s 流式未通过: %s", want, entry.Get("stream").Raw)
		}
		if !strings.HasSuffix(entry.Get("request_url").String(), protocolWantPaths[i]) {
			t.Fatalf("%s request_url = %q, want 以 %s 结尾", want, entry.Get("request_url").String(), protocolWantPaths[i])
		}
	}
	// openai 已配端点（source=endpoint），另两条未配 → 走继承 Base URL。
	if src := protocols[0].Get("url_source").String(); src != "endpoint" {
		t.Fatalf("openai url_source = %q, want endpoint", src)
	}
	for _, i := range []int{1, 2} {
		if src := protocols[i].Get("url_source").String(); src != "inherited" {
			t.Fatalf("%s url_source = %q, want inherited", protocols[i].Get("protocol").String(), src)
		}
	}
	// 指纹：三条都应与自身协议同族（假上游按各自方言回错误）；anthropic 侧还带回原生头。
	for i := range protocols {
		if !protocols[i].Get("fingerprint.same_family").Bool() {
			t.Fatalf("%s 指纹不同族: %s", protocols[i].Get("protocol").String(), protocols[i].Get("fingerprint").Raw)
		}
	}
	if got := protocols[2].Get("fingerprint.family").String(); got != "anthropic" {
		t.Fatalf("anthropic fingerprint.family = %q, want anthropic", got)
	}
	if heads := protocols[2].Get("fingerprint.native_headers").Array(); len(heads) == 0 {
		t.Fatalf("anthropic 未采到任何原生响应头: %s", protocols[2].Get("fingerprint").Raw)
	}
	// 字段命中：anthropic 原生形状 8/8，openai/responses 的假上游只给了精简 usage。
	if miss := protocols[2].Get("fields.miss").Array(); len(miss) != 0 {
		t.Fatalf("anthropic fields.miss = %v, want 空", miss)
	}
	if hit := protocols[2].Get("fields.hit").Array(); len(hit) != 8 {
		t.Fatalf("anthropic fields.hit = %d, want 8", len(hit))
	}
	if protocols[0].Get("id_prefix").String() != "chatcmpl" {
		t.Fatalf("openai id_prefix = %q, want chatcmpl", protocols[0].Get("id_prefix").String())
	}
	// 总评：三条都同族 → medium（分不出后端家族），按字段命中数点名 anthropic。
	if got := gjson.Get(w.Body.String(), "data.recommendation.protocol").String(); got != "anthropic" {
		t.Fatalf("recommendation.protocol = %q, want anthropic", got)
	}
	if got := gjson.Get(w.Body.String(), "data.recommendation.confidence").String(); got != "medium" {
		t.Fatalf("recommendation.confidence = %q, want medium", got)
	}
	if !protocols[2].Get("recommended").Bool() {
		t.Fatalf("anthropic 行未被标记 recommended: %s", protocols[2].Raw)
	}
	if notes := gjson.Get(w.Body.String(), "data.recommendation.notes").Array(); len(notes) != 3 {
		t.Fatalf("recommendation.notes = %d 条, want 3 条固定免责", len(notes))
	}

	mu.Lock()
	defer mu.Unlock()
	// 指纹探针必须真的发了假模型名：否则这三条请求等于白打。
	probePerPath := map[string]int{}
	authPerPath := map[string]string{}
	for _, h := range hits {
		if h.Model == protocolprobe.ProbeModelName {
			probePerPath[h.Path]++
		}
		if prev, ok := authPerPath[h.Path]; ok && prev != h.Auth {
			t.Fatalf("%s 两次请求用了不同凭据（%q / %q）——同组同凭据被破坏", h.Path, prev, h.Auth)
		}
		authPerPath[h.Path] = h.Auth
	}
	for _, path := range protocolWantPaths {
		if probePerPath[path] != 1 {
			t.Fatalf("%s 指纹探针 = %d 次, want 1（模型名 %s）", path, probePerPath[path], protocolprobe.ProbeModelName)
		}
	}
	if authPerPath["/v1/messages"] != "sk-pool-1" {
		t.Fatalf("anthropic 路径凭据头 = %q, want sk-pool-1（x-api-key 方言）", authPerPath["/v1/messages"])
	}
	if !strings.HasPrefix(authPerPath["/v1/chat/completions"], "Bearer ") {
		t.Fatalf("openai 路径凭据头 = %q, want Bearer 前缀", authPerPath["/v1/chat/completions"])
	}
}

// 流式「回了但不合规」必须单列 noncompliant，且整条协议降级为 partial——
// 把它算成通过会把半坏的流式路径推给真实流量。
func TestTestProtocolsHandler_StreamNonCompliantIsPartial(t *testing.T) {
	testsupport.InitTestDB(t)
	var hits []protocolHit
	var mu sync.Mutex
	srv := httptest.NewServer(protocolUpstream(&hits, &mu, false))
	t.Cleanup(srv.Close)
	associationID := seedProtocolFixture(t, srv.URL)

	w := callProtocolsTest(t, associationID)
	openai := gjson.Get(w.Body.String(), "data.protocols").Array()[0]
	if got := openai.Get("stream.status").String(); got != "noncompliant" {
		t.Fatalf("openai stream.status = %q, want noncompliant；detail=%s", got, openai.Get("stream.detail").String())
	}
	if openai.Get("stream.passed").Bool() {
		t.Fatalf("openai 流式不合规却被判通过: %s", openai.Get("stream").Raw)
	}
	if got := openai.Get("verdict").String(); got != "partial" {
		t.Fatalf("openai verdict = %q, want partial", got)
	}
	if !gjson.Get(w.Body.String(), "data.passed").Bool() {
		t.Fatalf("另有两条协议可用，data.passed 应为 true，body=%s", w.Body.String())
	}
	if got := gjson.Get(w.Body.String(), "data.recommendation.protocol").String(); got == "openai" {
		t.Fatalf("部分支持的协议不应被总评点名：%s", gjson.Get(w.Body.String(), "data.recommendation").Raw)
	}
	actions := gjson.Get(w.Body.String(), "data.recommendation.actions").Array()
	found := false
	for _, a := range actions {
		if strings.Contains(a.String(), "部分支持") {
			found = true
		}
	}
	if !found {
		t.Fatalf("总评未提示「部分支持」: %v", actions)
	}
}

// 路径不存在（404）要判不可用而不是「检测失败」：这是上游只实现一种协议时的正常形态。
func TestTestProtocolsHandler_MissingProtocolPathIsUnusable(t *testing.T) {
	testsupport.InitTestDB(t)
	var hits []protocolHit
	var mu sync.Mutex
	full := protocolUpstream(&hits, &mu, true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"not found","type":"invalid_request_error"}}`))
			return
		}
		full(w, r)
	}))
	t.Cleanup(srv.Close)
	associationID := seedProtocolFixture(t, srv.URL)

	w := callProtocolsTest(t, associationID)
	protocols := gjson.Get(w.Body.String(), "data.protocols").Array()
	for _, i := range []int{1, 2} {
		entry := protocols[i]
		if got := entry.Get("verdict").String(); got != "unusable" {
			t.Fatalf("%s verdict = %q, want unusable", entry.Get("protocol").String(), got)
		}
		if code := entry.Get("non_stream.status_code").Int(); code != 404 {
			t.Fatalf("%s status_code = %d, want 404", entry.Get("protocol").String(), code)
		}
	}
	if got := protocols[0].Get("verdict").String(); got != "usable" {
		t.Fatalf("openai verdict = %q, want usable", got)
	}
	if !gjson.Get(w.Body.String(), "data.passed").Bool() {
		t.Fatalf("openai 可用，data.passed 应为 true，body=%s", w.Body.String())
	}
	if got := gjson.Get(w.Body.String(), "data.recommendation.confidence").String(); got != "high" {
		t.Fatalf("唯一可用协议时 confidence = %q, want high", got)
	}
}
