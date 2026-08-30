package chat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
)

// 多协议端点供应商（勾选 openai + anthropic）：请求按**选中端点协议**判定透传/转换——同协议
// 透传（body 原样进对应端点），入站 wire 不在勾选集合时转换并选中主协议端点
// （ProtocolOfType(Provider.Type)）。锁 AC-3。
func TestBalanceChat_MultiProtocolEndpoints_PassthroughAndTransformByEndpoint(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	var oaHits, anHits atomic.Int32
	var oaBodies, anBodies []string
	var bodyMu sync.Mutex
	recordBody := func(bodies *[]string, body string) {
		bodyMu.Lock()
		defer bodyMu.Unlock()
		*bodies = append(*bodies, body)
	}
	openaiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oaHits.Add(1)
		body, _ := io.ReadAll(r.Body)
		recordBody(&oaBodies, string(body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-t","object":"chat.completion","created":0,"model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer openaiSrv.Close()
	anthropicSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anHits.Add(1)
		body, _ := io.ReadAll(r.Body)
		recordBody(&anBodies, string(body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_01","type":"message","role":"assistant","model":"pm1","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer anthropicSrv.Close()

	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	// base_url 是永不生效的兜底：两端点的 URL 均显式覆盖（URL 覆盖继承链语义，
	// 请求打哪个 server 可精确断言端点的选中结果）。
	provider := models.Provider{Name: "dual", Type: providers.TypeOpenAI, Config: `{"base_url":"http://127.0.0.1:1/never"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	epOA := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), URL: openaiSrv.URL, Enabled: true}
	epAn := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolAnthropic), URL: anthropicSrv.URL, Enabled: true}
	for _, ep := range []*models.Endpoint{&epOA, &epAn} {
		if err := models.DB.Create(ep).Error; err != nil {
			t.Fatalf("create endpoint: %v", err)
		}
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	cred := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-any"), KeyHash: cipher.Hash("sk-any")}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	mwp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "pm1", Weight: 10, Priority: 1}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	// ① openai 入站：wire 命中勾选协议 → 透传选中 openai 端点
	runBalanceForStyle := func(style, raw string) {
		t.Helper()
		pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{mwp})
		if err != nil {
			t.Fatalf("build candidate pool: %v", err)
		}
		result, err := BalanceChat(context.Background(), BalanceInput{
			Start:  time.Now(),
			Style:  style,
			Before: Before{Model: "m1", Stream: false, raw: []byte(raw)},
			ProvidersWithMeta: ProvidersWithMeta{
				CandidatePool: pool,
				Model:         &models.Model{Name: "m1"},
			},
			ReqMeta: models.ReqMeta{Header: http.Header{}},
		})
		if err != nil {
			t.Fatalf("BalanceChat style=%s err = %v", style, err)
		}
		if result.ProviderName != "dual" {
			t.Fatalf("style=%s ProviderName = %q, want dual", style, result.ProviderName)
		}
	}

	runBalanceForStyle(string(consts.StyleOpenAI), `{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)
	if oaHits.Load() != 1 || anHits.Load() != 0 {
		t.Fatalf("after openai request: oaHits=%d anHits=%d, want 1/0（透传选中 openai 端点）", oaHits.Load(), anHits.Load())
	}
	// 透传 = body 原样：messages 结构原样到达 openai 端点（未被 shape 变换）。
	bodyMu.Lock()
	oaBody := oaBodies[len(oaBodies)-1]
	bodyMu.Unlock()
	if !strings.Contains(oaBody, `"messages":[{"role":"user","content":"hi"}]`) {
		t.Fatalf("openai passthrough body = %q, want 原样 messages 结构", oaBody)
	}

	// ② anthropic 入站：wire 命中勾选协议 → 透传选中 anthropic 端点
	runBalanceForStyle(string(consts.StyleAnthropic), `{"model":"m1","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`)
	if oaHits.Load() != 1 || anHits.Load() != 1 {
		t.Fatalf("after anthropic request: oaHits=%d anHits=%d, want 1/1（透传选中 anthropic 端点）", oaHits.Load(), anHits.Load())
	}
	// anthropic 端点收到 anthropic 形状 body（max_tokens 顶层键 + content 原样字符串）。
	// 若取数点退化按 Provider.Type（openai）判定，此请求会被转换、body 失去 anthropic 特征。
	bodyMu.Lock()
	anBody := anBodies[len(anBodies)-1]
	bodyMu.Unlock()
	if !strings.Contains(anBody, `"max_tokens":64`) || !strings.Contains(anBody, `"messages":[{"role":"user","content":"hi"}]`) {
		t.Fatalf("anthropic passthrough body = %q, want anthropic 原样结构（max_tokens 顶层键 + content 字符串）", anBody)
	}

	// ③ responses 入站：wire=openai-res 不在勾选集合 → 转换路径选中主协议端点（openai）
	runBalanceForStyle(string(consts.StyleOpenAIRes), `{"model":"m1","input":"hi"}`)
	if oaHits.Load() != 2 || anHits.Load() != 1 {
		t.Fatalf("after responses request: oaHits=%d anHits=%d, want 2/1（转换路径选中主协议端点 openai）", oaHits.Load(), anHits.Load())
	}
	// 转换产物是 openai 形状：body 不再含 responses 的 "input" 键，而是 messages。
	bodyMu.Lock()
	oaBody = oaBodies[len(oaBodies)-1]
	bodyMu.Unlock()
	if strings.Contains(oaBody, `"input"`) || !strings.Contains(oaBody, `"messages"`) {
		t.Fatalf("responses transform body = %q, want openai 形状（去 input 键、含 messages）", oaBody)
	}
}

// 多分组供应商：白名单过滤后分组级加权选择 → 命中分组的凭据明文作为上游鉴权 key；
// 端点 URL 显式覆盖时请求打覆盖 URL 而非 Provider.Config.base_url。锁 AC-4。
func TestBalanceChat_KeyGroupWhitelist_CredentialPlainKeyAndURLOverride(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	var mu sync.Mutex
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auths = append(auths, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-t","object":"chat.completion","created":0,"model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	// base_url 指向永不生效地址：端点 URL 显式覆盖后请求必须打 srv。
	provider := models.Provider{Name: "multi", Type: providers.TypeOpenAI, Config: `{"base_url":"http://127.0.0.1:1/never"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), URL: srv.URL, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}

	// g1 白名单 [m1]（命中）、g2 白名单 [m2]（不命中）——白名单过滤决定分组，
	// 不使用「命中分组读 Authorization」之外的随机性。
	g1 := models.KeyGroup{ProviderID: provider.ID, Name: "g1", Weight: 10, Models: "m1"}
	g2 := models.KeyGroup{ProviderID: provider.ID, Name: "g2", Weight: 1, Models: "m2"}
	for _, g := range []*models.KeyGroup{&g1, &g2} {
		if err := models.DB.Create(g).Error; err != nil {
			t.Fatalf("create key group: %v", err)
		}
	}
	for _, gc := range []struct {
		g      models.KeyGroup
		plain  string
		status string
	}{
		{g1, "sk-group1-key", "active"},
		{g2, "sk-group2-key", "active"},
	} {
		enc := mustEncrypt(t, cipher, gc.plain)
		cred := models.Credential{GroupID: &gc.g.ID, Key: enc, KeyHash: cipher.Hash(gc.plain)}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential for %s: %v", gc.plain, err)
		}
	}

	mwp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "pm1", Weight: 10, Priority: 1}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{mwp})
	if err != nil {
		t.Fatalf("build candidate pool: %v", err)
	}
	result, err := BalanceChat(context.Background(), BalanceInput{
		Start:  time.Now(),
		Style:  string(consts.StyleOpenAI),
		Before: Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: "m1"},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err != nil {
		t.Fatalf("BalanceChat err = %v", err)
	}
	if result.ProviderName != "multi" {
		t.Fatalf("ProviderName = %q, want multi", result.ProviderName)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(auths) != 1 {
		t.Fatalf("upstream hits = %d, want 1（仅 g1 命中白名单）", len(auths))
	}
	if auths[0] != "Bearer sk-group1-key" {
		t.Fatalf("Authorization = %q, want %q（命中分组的凭据明文 key 作为鉴权）", auths[0], "Bearer sk-group1-key")
	}
	// URL 覆盖：请求到达 srv（端点 URL）而非 base_url 的 never 地址——端点 URL 覆盖继承链。
}

// 组内多凭据轮询推进（跨请求进程级状态）：同一供应商同一分组连续两次请求必须
// 依次选中 key1/key2（rrstate 设计「选择即推进」，与虚拟模型轮询单例对齐）。
// 每请求新建 Selector（指针归零）会让两次都命中 key1——本用例锁定轮询真正生效。
func TestBalanceChat_SameGroupMultipleCredentials_RoundRobin(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	// 默认选路器是进程级单例：用例开头复位指针，避免受前序用例推进位置影响。
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)

	var mu sync.Mutex
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auths = append(auths, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-t","object":"chat.completion","created":0,"model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "rr-provider", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL)}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	// 创建顺序决定 ID ASC：key1 先建，轮询首现 key1。
	for _, plain := range []string{"sk-key1", "sk-key2"} {
		enc := mustEncrypt(t, cipher, plain)
		cred := models.Credential{GroupID: &grp.ID, Key: enc, KeyHash: cipher.Hash(plain)}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential %s: %v", plain, err)
		}
	}
	mwp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "pm1", Weight: 10, Priority: 1}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	runRequest := func() {
		t.Helper()
		pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{mwp})
		if err != nil {
			t.Fatalf("build candidate pool: %v", err)
		}
		if _, err := BalanceChat(context.Background(), BalanceInput{
			Start:  time.Now(),
			Style:  string(consts.StyleOpenAI),
			Before: Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
			ProvidersWithMeta: ProvidersWithMeta{
				CandidatePool: pool,
				Model:         &models.Model{Name: "m1"},
			},
			ReqMeta: models.ReqMeta{Header: http.Header{}},
		}); err != nil {
			t.Fatalf("BalanceChat err = %v", err)
		}
	}

	runRequest()
	runRequest()

	mu.Lock()
	defer mu.Unlock()
	if len(auths) != 2 {
		t.Fatalf("upstream hits = %d, want 2", len(auths))
	}
	if auths[0] != "Bearer sk-key1" || auths[1] != "Bearer sk-key2" {
		t.Fatalf("auth sequence = %v, want [Bearer sk-key1 Bearer sk-key2]（组内凭据轮询必须逐请求推进）", auths)
	}
}
