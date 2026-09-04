package chat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// 流式组合态 e2e（缺口①，Stream: true）：入站 style 决定**客户端协议格式**，
// 端点协议决定**上游格式**——转换方向恒为「上游协议 → 客户端格式」。
// 与 TestBalanceChat_MultiProtocolEndpoints_*（非流）对照锁 AC：非流用例断言出站
// body 形状，本组用例断言回写流到客户端的 SSE 事件形状（handler/v1 仅 io.Copy
// 搬运，service 层 result.Response.Body 即客户端收到的字节流）。

// newStreamSSEServer 返回按协议返回固定 SSE 流的 mock 上游。
// 响应头必须 text/event-stream：TransformProviderResponse 以此区分流式/非流式分支，
// 否则 SSE 会被当非流式 JSON 解析而失败（流式判定契约，见 transform_provider_response.go）。
func newStreamSSEServer(t *testing.T, sse string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sse))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// setupStreamChannelProvider 建「1 provider + 1 端点 + 1 分组 + 1 凭据 + 1 关联」。
// 端点协议经 epProtocol 显式指定（决定上游格式），URL 显式覆盖（打哪个 server 可断言端点选中）。
// 复用与既有 chat_channel_integration_test.go 相同的行构造模式，仅收敛样板。
func setupStreamChannelProvider(t *testing.T, cipher *credentialcrypto.Cipher, name, providerType string, epProtocol string, epURL string) (models.ModelWithProvider, models.Provider) {
	t.Helper()
	model := models.Model{Name: name}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model %s: %v", name, err)
	}
	provider := models.Provider{Name: "st-" + name, Type: providerType, Config: `{"base_url":"http://127.0.0.1:1/never"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: epProtocol, URL: epURL, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	plain := "sk-stream-key"
	cred := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain)}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	mwp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "pm1", Weight: 10, Priority: 1}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	return mwp, provider
}

// runStreamBalance 以 Stream: true 跑 BalanceChat，返回客户端应收到的字节流
// （result.Response.Body 即 handler io.Copy 搬运给客户端的原流）。
// modelName 必须与 mwp 关联的模型名一致（真实路径中 Before.Model 就是查询
// ModelWithProvider 的键），保证 Before 与池状态同源。
func runStreamBalance(t *testing.T, style string, modelName string, raw []byte, mwp models.ModelWithProvider) []byte {
	t.Helper()
	pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{mwp})
	if err != nil {
		t.Fatalf("build candidate pool: %v", err)
	}
	result, err := BalanceChat(context.Background(), BalanceInput{
		Start:  time.Now(),
		Style:  style,
		Before: Before{Model: modelName, Stream: true, raw: raw},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: modelName},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err != nil {
		t.Fatalf("BalanceChat style=%s err = %v", style, err)
	}
	defer result.Response.Body.Close()
	body, err := io.ReadAll(result.Response.Body)
	if err != nil {
		t.Fatalf("read stream body: %v", err)
	}
	return body
}

// ① 透传流式（AC-1）：入站 style 与端点协议同协议时 SSE 原样到达客户端——逐 byte
// 相等（转换旁路不改透传字节）。openai 入站选中 openai 端点、anthropic 入站选中
// anthropic 端点，分别用各自协议的上游 SSE 锁「原样」。
func TestBalanceChat_Stream_PassthroughByEndpoint(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	const openaiSSE = `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"pm1","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}` + "\n\n" +
		`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"pm1","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}` + "\n\n" +
		`data: [DONE]` + "\n\n"

	openaiSrv, openaiHits := newStreamSSEServer(t, openaiSSE)

	// ①a openai 入站 → openai 端点（透传选中 openai 端点）
	mwpOA, _ := setupStreamChannelProvider(t, cipher, "stream", providers.TypeOpenAI, string(consts.ProtocolOpenAI), openaiSrv.URL)
	got := string(runStreamBalance(t, string(consts.StyleOpenAI), "stream", []byte(`{"model":"stream","stream":true,"messages":[{"role":"user","content":"hi"}]}`), mwpOA))
	if got != openaiSSE {
		t.Fatalf("openai passthrough stream =\n%q\nwant 上游 SSE 原样:\n%q", got, openaiSSE)
	}
	if openaiHits.Load() != 1 {
		t.Fatalf("openai endpoint hits = %d, want 1", openaiHits.Load())
	}

	// ①b anthropic 入站 → anthropic 端点（透传选中 anthropic 端点）
	anthropicSSE := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_123\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"pm1\",\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	anthropicSrv, anthropicHits := newStreamSSEServer(t, anthropicSSE)

	mwpAn, _ := setupStreamChannelProvider(t, cipher, "stream2", providers.TypeAnthropic, string(consts.ProtocolAnthropic), anthropicSrv.URL)
	gotAn := string(runStreamBalance(t, string(consts.StyleAnthropic), "stream2", []byte(`{"model":"stream2","stream":true,"max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`), mwpAn))
	if gotAn != anthropicSSE {
		t.Fatalf("anthropic passthrough stream =\n%q\nwant 上游 SSE 原样:\n%q", gotAn, anthropicSSE)
	}
	if anthropicHits.Load() != 1 {
		t.Fatalf("anthropic endpoint hits = %d, want 1", anthropicHits.Load())
	}
}

// ② 转换流式（AC-2/AC-3）：入站 style 与端点协议不同 → 上游 SSE 实时转换为**客户端
// 协议** SSE 回写。断言只锁「协议特征事件存在且类型正确」——转换细节（id/usage/事件
// 顺序）已由 golden_test.go 逐 byte 钉死，此处避免双源漂移。
func TestBalanceChat_Stream_TransformByEndpoint(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	// 上游 openai SSE（用于 anthropic 入站 / responses 入站两条转换链路）
	openaiUpstream := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"pm1","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}` + "\n\n" +
		`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"pm1","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}` + "\n\n" +
		`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"pm1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}` + "\n\n" +
		`data: [DONE]` + "\n\n"
	openaiSrv, _ := newStreamSSEServer(t, openaiUpstream)

	// 上游 anthropic SSE（用于 openai 入站 → anthropic 端点转换链路）
	anthropicUpstream := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_123\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"pm1\",\"stop_reason\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	anthropicSrv, _ := newStreamSSEServer(t, anthropicUpstream)

	type streamCase struct {
		name          string
		style         string
		raw           []byte
		epProtocol    string
		epURL         string
		providerType  string
		modelName     string
		wantSubstrs   []string // 客户端收到流中必须出现的协议特征子串
		wantNotSubstr string   // 不应出现（如反向协议特征），可空
	}

	cases := []streamCase{
		{
			// ②a openai 入站 → anthropic 端点：上游 anthropic SSE 转成 openai SSE 回写
			name:         "openai_in_anthropic_endpoint",
			style:        string(consts.StyleOpenAI),
			raw:          []byte(`{"model":"m7","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
			epProtocol:   string(consts.ProtocolAnthropic),
			epURL:        anthropicSrv.URL,
			providerType: providers.TypeAnthropic,
			modelName:    "m7",
			wantSubstrs: []string{
				`"object":"chat.completion.chunk"`,
				`"delta":{`,
				"data: [DONE]",
			},
			wantNotSubstr: "event: message_start",
		},
		{
			// ②b anthropic 入站 → openai 端点：上游 openai SSE 转成 anthropic SSE 回写
			name:         "anthropic_in_openai_endpoint",
			style:        string(consts.StyleAnthropic),
			raw:          []byte(`{"model":"m8","stream":true,"max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`),
			epProtocol:   string(consts.ProtocolOpenAI),
			epURL:        openaiSrv.URL,
			providerType: providers.TypeOpenAI,
			modelName:    "m8",
			wantSubstrs: []string{
				"event: message_start",
				`"type":"content_block_delta"`,
				"event: message_delta",
				"event: message_stop",
			},
			wantNotSubstr: "data: [DONE]",
		},
		{
			// ②c responses 入站 → openai 端点：上游 openai SSE 转成 responses SSE 回写
			name:         "responses_in_openai_endpoint",
			style:        string(consts.StyleOpenAIRes),
			raw:          []byte(`{"model":"m9","stream":true,"input":"hi"}`),
			epProtocol:   string(consts.ProtocolOpenAI),
			epURL:        openaiSrv.URL,
			providerType: providers.TypeOpenAI,
			modelName:    "m9",
			wantSubstrs: []string{
				"event: response.created",
				`"type":"response.output_text.delta"`,
				"event: response.completed",
			},
			wantNotSubstr: "data: [DONE]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mwp, _ := setupStreamChannelProvider(t, cipher, tc.modelName, tc.providerType, tc.epProtocol, tc.epURL)
			got := string(runStreamBalance(t, tc.style, tc.modelName, tc.raw, mwp))

			for _, sub := range tc.wantSubstrs {
				if !strings.Contains(got, sub) {
					t.Fatalf("client stream missing %q:\n%s", sub, got)
				}
			}
			if tc.wantNotSubstr != "" && strings.Contains(got, tc.wantNotSubstr) {
				t.Fatalf("client stream unexpectedly contains %q:\n%s", tc.wantNotSubstr, got)
			}
		})
	}
}
