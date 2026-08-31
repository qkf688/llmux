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

// 成功请求落库的 ChatLog 行必须带三层选路命中信息（端点/URL/分组/凭据标识），
// 供 logs 页展示「实际打到了哪个端点、用哪条 key」——S3-3 建行填充，DTO 侧另有
// 响应契约测试锁序列化键名，本用例锁「填充发生且值来自 Selection」。锁 AC-3。
func TestBalanceChat_ChatLogRecordsChannelSelection(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-t","object":"chat.completion","created":0,"model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "log-p", Type: providers.TypeOpenAI, Config: `{"base_url":"http://127.0.0.1:1/never"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	// URL 显式覆盖：EndpointURL 断言到实际出站 URL（覆盖继承链语义），而非 base_url。
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), URL: srv.URL, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "低价组", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	plain := "sk-log-key"
	cred := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain), Note: "production-key"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	mwp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "pm1", Weight: 10, Priority: 1}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

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

	var logs []models.ChatLog
	if err := models.DB.Order("id DESC").Limit(1).Find(&logs).Error; err != nil {
		t.Fatalf("query chat logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("chat logs = %d, want 1", len(logs))
	}
	log := logs[0]
	if log.EndpointProtocol != string(consts.ProtocolOpenAI) {
		t.Fatalf("EndpointProtocol = %q, want %q", log.EndpointProtocol, string(consts.ProtocolOpenAI))
	}
	if log.EndpointURL != srv.URL {
		t.Fatalf("EndpointURL = %q, want %q（必须取继承链解析后的实际出站 URL）", log.EndpointURL, srv.URL)
	}
	if log.KeyGroupName != "低价组" {
		t.Fatalf("KeyGroupName = %q, want 低价组", log.KeyGroupName)
	}
	if log.CredentialNote != "production-key" {
		t.Fatalf("CredentialNote = %q, want production-key（Note 非空用 Note）", log.CredentialNote)
	}
}

// credentialLabel 纯函数表驱动：Note 非空优先；空 Note 回退 KeyHash 前 8 位加 # 前缀
// （多 key 排查需在日志里区分命中 key，哈希标识不解密即可获得）。
func TestCredentialLabel(t *testing.T) {
	cases := []struct {
		name string
		cred models.Credential
		want string
	}{
		{"note 优先", models.Credential{Note: "prod", KeyHash: "abcdef1234567890"}, "prod"},
		{"空 note 回退 KeyHash 前 8 位", models.Credential{KeyHash: "abcdef1234567890"}, "#abcdef12"},
		{"KeyHash 不足 8 位全量", models.Credential{KeyHash: "abc"}, "#abc"},
		{"KeyHash 全空显式占位", models.Credential{}, "(unknown)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := credentialLabel(tc.cred); got != tc.want {
				t.Fatalf("credentialLabel(%+v) = %q, want %q", tc.cred, got, tc.want)
			}
		})
	}
}

const chatSuccessBody = `{"id":"chatcmpl-t","object":"chat.completion","created":0,"model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

// 组内故障转移端到端（AC-1）：同组两凭据，上游对 key1 返回 429 → 网关自动同组换
// key2 重试成功。断言：请求最终成功、上游 auth 序列 [key1, key2]（换 key 发生）、
// key1 CooldownUntil 落库非空、落库 2 条 ChatLog 各带对应 credential_note。
func TestBalanceChat_CredentialFailover_SwapKeyWithinGroup(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)

	var mu sync.Mutex
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		mu.Lock()
		auths = append(auths, auth)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if auth == "Bearer sk-key1" {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(chatSuccessBody))
	}))
	defer srv.Close()

	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "fo-p", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL)}
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
	// 创建顺序决定轮询首现：key1 先建，第一次 Select 命中 key1。
	key1 := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-key1"), KeyHash: cipher.Hash("sk-key1"), Note: "key1-note"}
	key2 := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-key2"), KeyHash: cipher.Hash("sk-key2"), Note: "key2-note"}
	for _, cred := range []*models.Credential{&key1, &key2} {
		if err := models.DB.Create(cred).Error; err != nil {
			t.Fatalf("create credential: %v", err)
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
		t.Fatalf("BalanceChat err = %v（key1 429 后应组内换 key2 成功）", err)
	}
	if result == nil {
		t.Fatal("BalanceChat result = nil")
	}

	mu.Lock()
	if len(auths) != 2 {
		mu.Unlock()
		t.Fatalf("upstream hits = %d, want 2（key1 失败后换 key2 重试）", len(auths))
	}
	if auths[0] != "Bearer sk-key1" || auths[1] != "Bearer sk-key2" {
		mu.Unlock()
		t.Fatalf("auth sequence = %v, want [Bearer sk-key1 Bearer sk-key2]（组内换 key 顺序）", auths)
	}
	mu.Unlock()

	// key1 已写冷却（组内故障转移的前提：失败的 key 凉一凉，避免反复撞同一限流）
	var key1Reload models.Credential
	if err := models.DB.First(&key1Reload, key1.ID).Error; err != nil {
		t.Fatalf("reload key1: %v", err)
	}
	if key1Reload.CooldownUntil == nil {
		t.Fatalf("key1 CooldownUntil = nil, want 429 已写冷却")
	}
	// key2 成功不应被冷却
	var key2Reload models.Credential
	if err := models.DB.First(&key2Reload, key2.ID).Error; err != nil {
		t.Fatalf("reload key2: %v", err)
	}
	if key2Reload.CooldownUntil != nil {
		t.Fatalf("key2 CooldownUntil = %v, want nil（成功凭据不应被冷却）", key2Reload.CooldownUntil)
	}

	// 两次 attempt 各建一条日志：失败行带 key1-note，成功行带 key2-note——排查可追踪
	var logs []models.ChatLog
	if err := models.DB.Order("id ASC").Limit(2).Find(&logs).Error; err != nil {
		t.Fatalf("query chat logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("chat logs = %d, want 2", len(logs))
	}
	if logs[0].CredentialNote != "key1-note" || logs[0].Status != "error" {
		t.Fatalf("first log CredentialNote=%q Status=%q, want key1-note/error", logs[0].CredentialNote, logs[0].Status)
	}
	if logs[1].CredentialNote != "key2-note" || logs[1].Status != "success" {
		t.Fatalf("second log CredentialNote=%q Status=%q, want key2-note/success", logs[1].CredentialNote, logs[1].Status)
	}
}

// 组内全部凭据不可用（AC-2）：同组两凭据均 429 → 组耗尽 → 组织级失败——
// ConsecutiveFailures 递增、BalanceChat 返回失败、两 key 均落冷却。
func TestBalanceChat_CredentialsExhausted_OrganizationFailure(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "exhaust-p", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL)}
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
	key1 := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-key1"), KeyHash: cipher.Hash("sk-key1")}
	key2 := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-key2"), KeyHash: cipher.Hash("sk-key2")}
	for _, cred := range []*models.Credential{&key1, &key2} {
		if err := models.DB.Create(cred).Error; err != nil {
			t.Fatalf("create credential: %v", err)
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
	_, err = BalanceChat(context.Background(), BalanceInput{
		Start:  time.Now(),
		Style:  string(consts.StyleOpenAI),
		Before: Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: "m1"},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err == nil {
		t.Fatal("BalanceChat err = nil, want 组内耗尽后组织级失败")
	}

	// 组耗尽时累计组织级 ConsecutiveFailures（设计定案第 5 节「层内耗尽才累计」）
	var mwpReload models.ModelWithProvider
	if err := models.DB.First(&mwpReload, mwp.ID).Error; err != nil {
		t.Fatalf("reload mwp: %v", err)
	}
	if mwpReload.ConsecutiveFailures != 1 {
		t.Fatalf("ConsecutiveFailures = %d, want 1（组耗尽补累计一次）", mwpReload.ConsecutiveFailures)
	}

	for i, key := range []models.Credential{key1, key2} {
		var got models.Credential
		if err := models.DB.First(&got, key.ID).Error; err != nil {
			t.Fatalf("reload key%d: %v", i+1, err)
		}
		if got.CooldownUntil == nil {
			t.Fatalf("key%d CooldownUntil = nil, want 冷却已写", i+1)
		}
	}
}

// 单 key 组（存量迁移形态）回归等价（AC-6）：429 组耗尽 → 组织级失败按
// **ReduceWeight**（权重减 1/3 而非移除）淘汰，ConsecutiveFailures=1 + 冷却写库。
// 断言窗口：max_retry=1 锁住「组耗尽那一刻」的池状态——若用默认 3，下一轮 retry
// 会因组内无可用凭据（唯一 key 已冷却）把候选彻底删掉，观察不到 ReduceWeight 语义。
// 顺带锁「冷却避免同请求重复轰击 429 上游」：改造前单 key 429 会每轮 retry 再打一次。
func TestBalanceChat_SingleKeyGroup_429_OrganizationSemantics(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)

	// 外层候选池只允许 1 轮：组耗尽的 ReduceWeight 中间态保留在池里可断言
	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyRequestMaxRetry).
		Update("value", "1").Error; err != nil {
		t.Fatalf("set max_retry=1: %v", err)
	}

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "single-p", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL)}
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
	key := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-only"), KeyHash: cipher.Hash("sk-only")}
	if err := models.DB.Create(&key).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	mwp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "pm1", Weight: 10, Priority: 1}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

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
	}); err == nil {
		t.Fatal("BalanceChat err = nil, want 失败")
	}

	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（冷却避免同请求重复轰击 429 上游）", hits.Load())
	}

	// 429 → ReduceWeight（减 1/3）而非 Remove：候选留池、权重降为 7（10-10/3）
	//——与改造前 handleNonOKProviderResponse 的 429 分支语义一致
	weight, ok := pool.WeightItems[mwp.ID]
	if !ok {
		t.Fatalf("WeightItems[%d] removed, want 7（429 语义是 ReduceWeight 而非移除）", mwp.ID)
	}
	if weight != 7 {
		t.Fatalf("WeightItems[%d] = %d, want 7（10 - 10/3）", mwp.ID, weight)
	}

	var mwpReload models.ModelWithProvider
	if err := models.DB.First(&mwpReload, mwp.ID).Error; err != nil {
		t.Fatalf("reload mwp: %v", err)
	}
	if mwpReload.ConsecutiveFailures != 1 {
		t.Fatalf("ConsecutiveFailures = %d, want 1", mwpReload.ConsecutiveFailures)
	}

	var keyReload models.Credential
	if err := models.DB.First(&keyReload, key.ID).Error; err != nil {
		t.Fatalf("reload key: %v", err)
	}
	if keyReload.CooldownUntil == nil {
		t.Fatalf("key CooldownUntil = nil, want 冷却已写")
	}
}

// 5xx 全组耗尽 → Remove 组织级语义（与 429 的 ReduceWeight 区分，AC-2 另一半）：
// 候选从池中移除（RemoveWeight+RemovePriority）+ ConsecutiveFailures=1 +
// key 冷却 reason=http_500。5xx 是服务端持续故障信号，不留池（与现状 5xx 分支一致）。
func TestBalanceChat_ServerErrors_RemoveOrganizationSemantics(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"upstream boom"}`))
	}))
	defer srv.Close()

	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "five-p", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL)}
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
	key := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, "sk-5xx"), KeyHash: cipher.Hash("sk-5xx")}
	if err := models.DB.Create(&key).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	mwp := models.ModelWithProvider{ModelID: model.ID, ProviderID: provider.ID, ProviderModel: "pm1", Weight: 10, Priority: 1}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

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
	}); err == nil {
		t.Fatal("BalanceChat err = nil, want 失败")
	}

	// 5xx → Remove（候选彻底移出池），与 429 的 ReduceWeight（留池降权）语义区分
	if _, ok := pool.WeightItems[mwp.ID]; ok {
		t.Fatalf("WeightItems[%d] still present, want removed（5xx 语义是 RemoveWeight）", mwp.ID)
	}
	if _, ok := pool.PriorityItems[mwp.ID]; ok {
		t.Fatalf("PriorityItems[%d] still present, want removed（5xx 语义是 RemovePriority）", mwp.ID)
	}

	var mwpReload models.ModelWithProvider
	if err := models.DB.First(&mwpReload, mwp.ID).Error; err != nil {
		t.Fatalf("reload mwp: %v", err)
	}
	if mwpReload.ConsecutiveFailures != 1 {
		t.Fatalf("ConsecutiveFailures = %d, want 1", mwpReload.ConsecutiveFailures)
	}

	var keyReload models.Credential
	if err := models.DB.First(&keyReload, key.ID).Error; err != nil {
		t.Fatalf("reload key: %v", err)
	}
	if keyReload.CooldownUntil == nil {
		t.Fatalf("key CooldownUntil = nil, want 冷却已写")
	}
	if keyReload.CooldownReason != "http_500" {
		t.Fatalf("CooldownReason = %q, want http_500", keyReload.CooldownReason)
	}
}
