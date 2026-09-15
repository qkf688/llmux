package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/virtualmodel"
)

// #6 出站头四层优先级链的 e2e 契约（方案 §3）：BuildHeaders 单测只钉到第三层，
// 第四层（providers 权威覆写）发生在 BuildReq——断言必须打在 httptest 上游**实际
// 收到**的头上（AGENTS §5：契约测试打序列化层，不测 Go 字段）。

// echoHeadersServer 记录各上游实际收到的请求头并回 200（合法 openai 非流形状）。
type echoHeadersServer struct {
	server   *httptest.Server
	mu       sync.Mutex
	received http.Header
}

func newEchoHeadersServer(t *testing.T) *echoHeadersServer {
	t.Helper()
	echo := &echoHeadersServer{}
	echo.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		echo.mu.Lock()
		echo.received = r.Header.Clone()
		echo.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-h","object":"chat.completion","created":0,"model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(echo.server.Close)
	return echo
}

func (e *echoHeadersServer) header(key string) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.received.Get(key)
}

func runKeyHeadersAttempt(t *testing.T, echo *echoHeadersServer, providerName string) singleProviderAttemptResult {
	t.Helper()
	providerRow := models.Provider{Name: providerName, Type: providers.TypeOpenAI}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	enabled := true
	mwp := models.ModelWithProvider{
		ProviderID:      providerRow.ID,
		ProviderModel:   "pm1",
		Status:          &enabled,
		WithHeader:      &enabled, // 入站透传开：让第一层参与撞名裁决
		CustomerHeaders: map[string]string{"X-Relay": "association-relay"},
	}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	source := http.Header{}
	source.Set("User-Agent", "browser/1.0")
	source.Set("X-Relay", "inbound-relay")
	source.Set("Cookie", "session-inbound") // 白名单外：入站值必须被挡
	source.Set("X-Trace", "trace-in")

	inbound := Before{
		Model:  "m1",
		Stream: false,
		raw:    []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`),
	}
	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-authoritative"}`, echo.server.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	result := executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:               context.Background(),
		Style:             string(consts.StyleOpenAI),
		Before:            inbound,
		RealModelName:     "m1",
		ReqMeta:           models.ReqMeta{Header: source, Headers: map[string]string{"User-Agent": "codex_cli_rs/0.1", "X-Relay": "key-relay", "Authorization": "Bearer key-level-loses"}},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		Selection:         selectionForEndpoint(string(consts.ProtocolOpenAI)),
		ChatModel:         chatModel,
		Client:            echo.server.Client(),
	}, make(chan models.ChatLog, 8))
	return result
}

// AC-4：四层各赢一次——key 覆盖入站透传、关联级覆盖 key、providers 权威覆写
// Authorization（key 级同名头被凭据压回）、白名单继续挡入站 Cookie（key 级不配它）。
func TestExecuteSingleProviderAttempt_KeyHeadersFourLayerPrecedence(t *testing.T) {
	initChatRecordTestDB(t)
	echo := newEchoHeadersServer(t)

	result := runKeyHeadersAttempt(t, echo, "p-keyhdr")
	if !result.Success {
		t.Fatalf("attempt should succeed against 200 echo server")
	}
	defer result.Response.Body.Close()

	if got := echo.header("User-Agent"); got != "codex_cli_rs/0.1" {
		t.Fatalf("上游收到 User-Agent = %q, want key 级覆盖入站透传", got)
	}
	if got := echo.header("X-Relay"); got != "association-relay" {
		t.Fatalf("上游收到 X-Relay = %q, want 关联级赢撞名（方案 §3）", got)
	}
	if got := echo.header("Authorization"); got != "Bearer sk-authoritative" {
		t.Fatalf("上游收到 Authorization = %q, want providers 权威覆写（第四层最高，key 级 Authorization 是死通道）", got)
	}
	if got := echo.header("Cookie"); got != "" {
		t.Fatalf("上游收到 Cookie = %q, want 空（白名单未放行 + key 级未配置，两层都不该送出）", got)
	}
	if got := echo.header("X-Trace"); got != "trace-in" {
		t.Fatalf("上游收到 X-Trace = %q, want 入站透传存活", got)
	}
}

// AC-5：虚拟模型路径共用同一装配点——balanceChatVirtual → retryLoop → attempt
// 零特判，ReqMeta.Headers 整包透传后照常注入上游。
func TestBalanceChatVirtual_KeyHeadersAppliedOnVirtualPath(t *testing.T) {
	virtualmodel.ResetSingletonForTest()
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	echo := newEchoHeadersServer(t)

	enabled := true
	real := models.Model{Name: "real-hdr"}
	if err := models.DB.Create(&real).Error; err != nil {
		t.Fatalf("create real model: %v", err)
	}
	virtual := models.VirtualModel{Name: "vm-keyhdr", Strategy: "priority", Enabled: &enabled}
	if err := models.DB.Create(&virtual).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}
	provider := models.Provider{Name: "vh-provider", Type: providers.TypeOpenAI, Config: `{"base_url":"http://127.0.0.1:1/never"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), URL: echo.server.URL, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	plain := "sk-vh-key"
	cred := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain)}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	mwp := models.ModelWithProvider{ModelID: real.ID, ProviderID: provider.ID, ProviderModel: "pm1", Status: &enabled, Weight: 10, Priority: 1}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	if err := models.DB.Create(&models.VirtualModelMapping{VirtualModelID: virtual.ID, RealModelID: real.ID, Enabled: &enabled}).Error; err != nil {
		t.Fatalf("create mapping: %v", err)
	}

	callerID := uint(21)
	reqMeta := models.ReqMeta{
		Header:     http.Header{},
		ApiKeyID:   &callerID,
		ApiKeyName: "vm-key",
		Headers:    map[string]string{"User-Agent": "claude-cli/2.0", "X-App": "cli"},
	}
	result, err := balanceChatVirtual(context.Background(), BalanceInput{
		Start:  time.Now(),
		Style:  string(consts.StyleOpenAI),
		Before: Before{Model: virtual.Name, raw: []byte(`{"model":"vm-keyhdr","messages":[{"role":"user","content":"hi"}]}`)},
		ProvidersWithMeta: ProvidersWithMeta{
			IsVirtualModel:    true,
			VirtualModelID:    virtual.ID,
			VirtualModelName:  virtual.Name,
			VirtualStrategy:   virtual.Strategy,
			OrderedRealModels: []virtualmodel.OrderedRealModel{{Model: real}},
		},
		ReqMeta: reqMeta,
	})
	if err != nil {
		t.Fatalf("balanceChatVirtual: %v", err)
	}
	defer result.Response.Body.Close()

	if got := echo.header("User-Agent"); got != "claude-cli/2.0" {
		t.Fatalf("虚拟路径上游收到 User-Agent = %q, want key 级头照常注入（装配点唯一，零特判）", got)
	}
	if got := echo.header("X-App"); got != "cli" {
		t.Fatalf("虚拟路径上游收到 X-App = %q, want cli", got)
	}
	// 归因快照与 key 头同链到达：日志行仍带调用方信息（#4-3 不变量在本片不回归）
	var got0 models.ChatLog
	if err := models.DB.Where("provider_name = ?", "vh-provider").First(&got0).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got0.ApiKeyID == nil || *got0.ApiKeyID != callerID {
		t.Fatalf("虚拟路径日志归因丢失: ApiKeyID = %v", got0.ApiKeyID)
	}
}
