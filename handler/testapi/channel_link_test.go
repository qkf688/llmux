package testapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// 本文件锁 #19 的契约：testapi 四个测试端点打上游必须走「主协议端点 + 候选分组 +
// 组内首个可用凭据」这条诊断链路，而**不是** Provider.Config。S6 起 Config 不再承载
// api_key，直用 Config 会让号池凭据的供应商恒被上游判 401（与 #21 健康检查同源）。
// 同时锁 #19 落地的响应契约：结果对象恒带 passed / group_id / group_name，前端据此把
// ✓/✕ 归位到正确的分组徽标（失败也必须在链路已选定时报出分组）。

const testCredHexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func setupTestAPICipher(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	c, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("credentialcrypto.New: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
	return c
}

func mustEncryptKey(t *testing.T, cipher *credentialcrypto.Cipher, plain string) string {
	t.Helper()
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt %s: %v", plain, err)
	}
	return enc
}

// completionHandlerByAuth 假上游 chat completion：按凭据头（OpenAI 系 Authorization /
// anthropic x-api-key）放行，未登记的一律 401——一把同时钉住「用了哪把 key」「打到哪个
// URL」「打了几次」。请求体带 "stream":true 时回 SSE chunk（react 测试走流式，回 JSON
// 会让 nsxno/react 在空 chunk 上取下标 panic）。
// allAuth 按调用顺序累积收到的凭据头，供「两次调用是否同一条 key」这类断言。
func completionHandlerByAuth(allowed map[string]string, hits *atomic.Int64, captured *atomic.Value, allAuth *atomic.Value) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		auth := r.Header.Get("Authorization")
		if auth == "" {
			auth = r.Header.Get("x-api-key")
		}
		if captured != nil {
			captured.Store(auth)
		}
		if allAuth != nil {
			if prev, _ := allAuth.Load().(string); prev == "" {
				allAuth.Store(auth)
			} else {
				allAuth.Store(prev + "|" + auth)
			}
		}
		if _, ok := allowed[auth]; !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"You didn't provide an API key."}}`))
			return
		}
		requested, _ := io.ReadAll(r.Body)
		if bytes.Contains(requested, []byte(`"stream":true`)) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, `data: {"id":"chunk-1","object":"chat.completion.chunk","created":0,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`+"\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-1","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}
}

func seedProviderRow(t *testing.T, name, typ string) models.Provider {
	t.Helper()
	// Config 刻意指向不可达主机且不含 api_key：只有走「端点 URL + 凭据明文」才可能成功，
	// 任何绕过选路的回归都会立刻炸而不是静默通过（同 providerapi 的种子姿势）。
	return seedProviderRowWithConfig(t, name, typ, `{"base_url":"https://should-not-use.example"}`)
}

// seedProviderRowWithConfig 允许测试指定 Provider.Config（如注入 openai_auth_type 方言键，
// Stage 15 ReAct 诊断方言贯通用例）；其余场景一律走 seedProviderRow 默认配置，
// 保持「Config 不承载可用凭据、不指向可达主机」的防绕过语义。
func seedProviderRowWithConfig(t *testing.T, name, typ, config string) models.Provider {
	t.Helper()
	provider := models.Provider{Name: name, Type: typ, Config: config}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	return provider
}

func seedEndpointRow(t *testing.T, providerID uint, protocol, url string) models.Endpoint {
	t.Helper()
	ep := models.Endpoint{ProviderID: providerID, Protocol: protocol, URL: url, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	return ep
}

// testapiPoolSeq 号池名序号：Pool.Name 是唯一索引，同一用例内多次建池不得撞名。
var testapiPoolSeq atomic.Int64

// seedPoolGroup 建「号池 + 引用该池的分组」——凭据只存在于池内（内联凭据概念已删除：
// 分组一律经 KeyGroup.PoolID 引用号池取凭据）。
// whitelist 为空 = 不限制；keys 为空 = 池内无凭据。
func seedPoolGroup(t *testing.T, cipher *credentialcrypto.Cipher, providerID uint, groupName, whitelist string, keys ...string) (models.Pool, models.KeyGroup) {
	t.Helper()
	pool := models.Pool{Name: fmt.Sprintf("pool-%s-%d", groupName, testapiPoolSeq.Add(1))}
	if err := models.DB.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	group := models.KeyGroup{ProviderID: providerID, Name: groupName, Weight: 1, Models: whitelist, PoolID: &pool.ID}
	if err := models.DB.Create(&group).Error; err != nil {
		t.Fatalf("create group %s: %v", groupName, err)
	}
	for _, k := range keys {
		cred := models.Credential{
			PoolID:  pool.ID,
			Key:     mustEncryptKey(t, cipher, k),
			KeyHash: cipher.Hash(k),
			Status:  models.CredentialStatusActive,
		}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential: %v", err)
		}
	}
	return pool, group
}

// seedDisabledPoolGroup 建「池内唯一凭据为 disabled」的分组（验「无可调度凭据的组被跳过」）。
func seedDisabledPoolGroup(t *testing.T, cipher *credentialcrypto.Cipher, providerID uint, name string) models.KeyGroup {
	t.Helper()
	pool, group := seedPoolGroup(t, cipher, providerID, name, "")
	cred := models.Credential{
		PoolID:  pool.ID,
		Key:     mustEncryptKey(t, cipher, "sk-disabled"),
		KeyHash: cipher.Hash("sk-disabled"),
		Status:  models.CredentialStatusDisabled,
	}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	return group
}

func seedAssociation(t *testing.T, providerID, modelID uint, providerModel string) models.ModelWithProvider {
	t.Helper()
	mwp := models.ModelWithProvider{ModelID: modelID, ProviderID: providerID, ProviderModel: providerModel}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	return mwp
}

func callProviderTest(t *testing.T, associationID uint) *httptest.ResponseRecorder {
	t.Helper()
	c, w := testsupport.NewTestContext("GET", "/api/test/"+strconv.FormatUint(uint64(associationID), 10))
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(associationID), 10)}}
	ProviderTestHandler(c)
	return w
}

func callProviderModelTest(t *testing.T, providerID uint, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/providers/"+strconv.FormatUint(uint64(providerID), 10)+"/test", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(providerID), 10)}}
	ProviderModelTestHandler(c)
	return w
}

func loadEnvelope(t *testing.T, w *httptest.ResponseRecorder) testsupport.APIEnvelope[map[string]any] {
	t.Helper()
	var payload testsupport.APIEnvelope[map[string]any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	return payload
}

// TestProviderTestHandler_UsesPoolCredentialNotProviderConfig 核心回归：分组关联号池时，
// GET /api/test/:id 必须用「端点 URL + 号池凭据明文」打上游并判通过，同时回传命中的分组。
func TestProviderTestHandler_UsesPoolCredentialNotProviderConfig(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	var captured atomic.Value
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{"Bearer sk-pool-1": ""}, &hits, &captured, nil))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-pool", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	_, group := seedPoolGroup(t, cipher, provider.ID, "池组", "", "sk-pool-1")
	mwp := seedAssociation(t, provider.ID, 1, "gpt-4.1")

	w := callProviderTest(t, mwp.ID)
	payload := loadEnvelope(t, w)
	if payload.Code != 200 {
		t.Fatalf("envelope code = %d, body=%s", payload.Code, w.Body.String())
	}
	if passed, _ := payload.Data["passed"].(bool); !passed {
		t.Fatalf("data.passed = %v, want true（号池凭据未被使用即为 #19 缺陷）；error=%v", payload.Data["passed"], payload.Data["error"])
	}
	if got, _ := captured.Load().(string); got != "Bearer sk-pool-1" {
		t.Fatalf("上游凭据头 = %q, want Bearer sk-pool-1", got)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（必须打端点 URL，不是 Config 里的占位主机）", hits.Load())
	}
	// 响应必须回传实际分组（前端按 (model, group) 归位 ✓/✕）。
	if got := gjson.Get(w.Body.String(), "data.group_id").Uint(); got != uint64(group.ID) {
		t.Fatalf("data.group_id = %d, want %d", got, group.ID)
	}
	if got := gjson.Get(w.Body.String(), "data.group_name").String(); got != "池组" {
		t.Fatalf("data.group_name = %q, want 池组", got)
	}
	if !strings.Contains(payload.Message, "chat.completion") {
		t.Fatalf("message = %q, want 上游原文", payload.Message)
	}
}

// TestProviderTestHandler_AnthropicEndpointUsesAPIKeyHeader 锁「按端点协议派生方言」：
// anthropic 端点必须走 x-api-key，用错方言等于把可用 key 发成 401。
func TestProviderTestHandler_AnthropicEndpointUsesAPIKeyHeader(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	var captured atomic.Value
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{"sk-anthropic": ""}, &hits, &captured, nil))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-ant", "anthropic")
	seedEndpointRow(t, provider.ID, "anthropic", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "池组-anthropic", "", "sk-anthropic")
	mwp := seedAssociation(t, provider.ID, 1, "claude-sonnet-4")

	w := callProviderTest(t, mwp.ID)
	payload := loadEnvelope(t, w)
	if passed, _ := payload.Data["passed"].(bool); !passed {
		t.Fatalf("data.passed = %v, want true; error=%v", payload.Data["passed"], payload.Data["error"])
	}
	if got, _ := captured.Load().(string); got != "sk-anthropic" {
		t.Fatalf("上游凭据头 = %q, want sk-anthropic（x-api-key）", got)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1", hits.Load())
	}
}

// TestProviderTestHandler_ModelNotWhitelistedIsValidationFailure 关联级按分组白名单过滤：
// 模型不在任何组白名单内 → 独立文案 + error_type=validation，且不得打上游
// （拿一个生产走不到的组测出「通过」是假信号）。
func TestProviderTestHandler_ModelNotWhitelistedIsValidationFailure(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{"Bearer sk-1": ""}, &hits, nil, nil))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-wl", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "只含别的模型", "other-model", "sk-1")
	mwp := seedAssociation(t, provider.ID, 1, "gpt-4.1")

	w := callProviderTest(t, mwp.ID)
	payload := loadEnvelope(t, w)
	if payload.Code != 200 {
		t.Fatalf("envelope code = %d, want 200（白名单未命中是一次测试结论，不是 HTTP 错误）", payload.Code)
	}
	if passed, _ := payload.Data["passed"].(bool); passed {
		t.Fatal("data.passed = true, want false")
	}
	if got := gjson.Get(w.Body.String(), "data.error_type").String(); got != "validation" {
		t.Fatalf("error_type = %q, want validation", got)
	}
	if err := gjson.Get(w.Body.String(), "data.error").String(); !strings.Contains(err, "不在任何分组白名单") || !strings.Contains(err, "自定义模型") {
		t.Fatalf("error = %q, want 含「不在任何分组白名单」与「自定义模型」两条出路指引", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0（白名单未命中不得打上游）", hits.Load())
	}
	// group_id 键恒在（未选定 = 0），前端无需区分「键缺失」与「未选定」。
	if !gjson.Get(w.Body.String(), "data.group_id").Exists() {
		t.Fatal("data.group_id 必须存在（未选定链路时为 0）")
	}
}

// TestProviderTestHandler_NoUsableCredentialDoesNotHitUpstream 组内凭据全停用 →
// error_type=auth 且不带着空凭据去打上游（避免把配置问题伪装成上游 401）。
func TestProviderTestHandler_NoUsableCredentialDoesNotHitUpstream(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{}, &hits, nil, nil))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-nokey", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	seedDisabledPoolGroup(t, cipher, provider.ID, "停用组")
	mwp := seedAssociation(t, provider.ID, 1, "gpt-4.1")

	w := callProviderTest(t, mwp.ID)
	if got := gjson.Get(w.Body.String(), "data.error_type").String(); got != "auth" {
		t.Fatalf("error_type = %q, want auth; body=%s", got, w.Body.String())
	}
	if err := gjson.Get(w.Body.String(), "data.error").String(); !strings.Contains(err, "无可用凭据") {
		t.Fatalf("error = %q, want 含「无可用凭据」", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0", hits.Load())
	}
}

// TestProviderTestHandler_SkipsGroupWithoutCredential 首个分组无可用凭据时换下一组
// （凭据层跨组），并把结果归位到实际使用的组。
func TestProviderTestHandler_SkipsGroupWithoutCredential(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	var captured atomic.Value
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{"Bearer sk-ok": ""}, &hits, &captured, nil))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-two", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	empty := seedDisabledPoolGroup(t, cipher, provider.ID, "全停用")
	_, ok := seedPoolGroup(t, cipher, provider.ID, "可用组", "", "sk-ok")
	mwp := seedAssociation(t, provider.ID, 1, "gpt-4.1")

	w := callProviderTest(t, mwp.ID)
	payload := loadEnvelope(t, w)
	if passed, _ := payload.Data["passed"].(bool); !passed {
		t.Fatalf("data.passed = %v, want true; error=%v", payload.Data["passed"], payload.Data["error"])
	}
	if got := gjson.Get(w.Body.String(), "data.group_id").Uint(); got != uint64(ok.ID) {
		t.Fatalf("data.group_id = %d, want %d（凭据不可用的组 %d 应被跳过）", got, ok.ID, empty.ID)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（跳组不产生上游请求）", hits.Load())
	}
}

// TestProviderTestHandler_DeterministicCredentialNoRRAdvance 锁诊断口径不碰生产轮询：
// 同组两条可用凭据，连测两次必须都用 ID 最小的那条（DefaultSelector 的指针不得被推进）。
func TestProviderTestHandler_DeterministicCredentialNoRRAdvance(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var all atomic.Value
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{
		"Bearer sk-first": "", "Bearer sk-second": "",
	}, nil, nil, &all))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-rr", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	pool, _ := seedPoolGroup(t, cipher, provider.ID, "两组", "")
	for _, k := range []string{"sk-first", "sk-second"} {
		cred := models.Credential{
			PoolID:  pool.ID,
			Key:     mustEncryptKey(t, cipher, k),
			KeyHash: cipher.Hash(k),
			Status:  models.CredentialStatusActive,
		}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential %s: %v", k, err)
		}
	}
	mwp := seedAssociation(t, provider.ID, 1, "gpt-4.1")

	for i := 0; i < 2; i++ {
		w := callProviderTest(t, mwp.ID)
		payload := loadEnvelope(t, w)
		if passed, _ := payload.Data["passed"].(bool); !passed {
			t.Fatalf("第 %d 次测试未通过: %v", i+1, payload.Data["error"])
		}
	}
	if got, _ := all.Load().(string); got != "Bearer sk-first|Bearer sk-first" {
		t.Fatalf("两次调用用到的凭据 = %q, want 两次都是首个可用（诊断不推进轮询）", got)
	}
}

// TestProviderTestHandler_InvalidIDIsBadRequest 非数字 ID 归 400（改造前被 GORM 判成
// 记录不存在而回 404，语义不准）。
func TestProviderTestHandler_InvalidIDIsBadRequest(t *testing.T) {
	testsupport.InitTestDB(t)
	setupTestAPICipher(t)

	w := callProviderTest(t, 424242) // 不存在的关联 ID
	if got := gjson.Get(w.Body.String(), "code").Int(); got != 404 {
		t.Fatalf("code = %d, want 404; body=%s", got, w.Body.String())
	}

	c, w2 := testsupport.NewTestContext("GET", "/api/test/not-a-number")
	c.Params = gin.Params{{Key: "id", Value: "not-a-number"}}
	ProviderTestHandler(c)
	if got := gjson.Get(w2.Body.String(), "code").Int(); got != 400 {
		t.Fatalf("code = %d, want 400; body=%s", got, w2.Body.String())
	}
	if got := gjson.Get(w2.Body.String(), "message").String(); got != "Invalid ID format" {
		t.Fatalf("message = %q, want Invalid ID format", got)
	}
}

// TestProviderModelTestHandler_AutoIgnoresWhitelist 供应商级自动口径**不**按白名单过滤：
// 白名单是上次同步的快照，按它过滤会把未同步的可用组误判成不可用。
func TestProviderModelTestHandler_AutoIgnoresWhitelist(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	var captured atomic.Value
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{"Bearer sk-x": ""}, &hits, &captured, nil))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-catalog", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	_, group := seedPoolGroup(t, cipher, provider.ID, "白名单不含它", "something-else", "sk-x")

	w := callProviderModelTest(t, provider.ID, `{"model":"brand-new-model"}`)
	payload := loadEnvelope(t, w)
	if passed, _ := payload.Data["passed"].(bool); !passed {
		t.Fatalf("data.passed = %v, want true; error=%v", payload.Data["passed"], payload.Data["error"])
	}
	if got := gjson.Get(w.Body.String(), "data.group_id").Uint(); got != uint64(group.ID) {
		t.Fatalf("data.group_id = %d, want %d", got, group.ID)
	}
	if got, _ := captured.Load().(string); got != "Bearer sk-x" {
		t.Fatalf("上游凭据头 = %q, want Bearer sk-x", got)
	}
}

// TestProviderModelTestHandler_GroupIDLocksGroup 显式 group_id 锁定该组：候选按 ID ASC
// 排在后面的组也要让位（假上游只登记第二个组的 key，走错组必然 401）。
func TestProviderModelTestHandler_GroupIDLocksGroup(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	var captured atomic.Value
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{"Bearer sk-b": ""}, &hits, &captured, nil))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-lock", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "A 组", "", "sk-a")
	_, groupB := seedPoolGroup(t, cipher, provider.ID, "B 组", "", "sk-b")

	w := callProviderModelTest(t, provider.ID, `{"model":"m1","group_id":`+strconv.FormatUint(uint64(groupB.ID), 10)+`}`)
	payload := loadEnvelope(t, w)
	if passed, _ := payload.Data["passed"].(bool); !passed {
		t.Fatalf("data.passed = %v, want true; error=%v", payload.Data["passed"], payload.Data["error"])
	}
	if got, _ := captured.Load().(string); got != "Bearer sk-b" {
		t.Fatalf("上游凭据头 = %q, want Bearer sk-b（必须锁定 B 组）", got)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（锁组后不再试别的组）", hits.Load())
	}
}

// TestProviderModelTestHandler_UpstreamFailureStillReportsGroup ✕ 也必须回传分组：
// 改造前失败走「信封 code=上游状态码、无 data」，前端拿不到分组，✕ 无法归位到徽标。
func TestProviderModelTestHandler_UpstreamFailureStillReportsGroup(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{}, &hits, nil, nil))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-fail", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	_, group := seedPoolGroup(t, cipher, provider.ID, "坏 key 组", "", "sk-dead")

	w := callProviderModelTest(t, provider.ID, `{"model":"m1"}`)
	payload := loadEnvelope(t, w)
	if payload.Code != 200 {
		t.Fatalf("envelope code = %d, want 200（一次失败的测试仍是成功的 HTTP 响应）; body=%s", payload.Code, w.Body.String())
	}
	if passed, _ := payload.Data["passed"].(bool); passed {
		t.Fatal("data.passed = true, want false")
	}
	if got := gjson.Get(w.Body.String(), "data.error_type").String(); got != "auth" {
		t.Fatalf("error_type = %q, want auth", got)
	}
	// 探针收敛到 credprobe.RunCompletionProbe 后统一口径：失败文案单一来源（「上游返回错误」
	// 而非各端点自造措辞）、细节 = 解析出的 error.message + HTTP 状态码行。
	if errText := gjson.Get(w.Body.String(), "data.error").String(); !strings.Contains(errText, "上游返回错误") ||
		!strings.Contains(errText, "You didn't provide an API key.") ||
		!strings.Contains(errText, "HTTP 状态码: 401") {
		t.Fatalf("error = %q, want 含「上游返回错误」+ 解析出的 message + 「HTTP 状态码: 401」", errText)
	}
	if got := gjson.Get(w.Body.String(), "data.group_id").Uint(); got != uint64(group.ID) {
		t.Fatalf("data.group_id = %d, want %d（失败也要能归位）", got, group.ID)
	}
	if got := gjson.Get(w.Body.String(), "data.group_name").String(); got != "坏 key 组" {
		t.Fatalf("data.group_name = %q", got)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（上游失败不跨组重试，避免 N 倍计费）", hits.Load())
	}
}

// TestProviderModelTestHandler_ForeignGroupRejected group_id 属于别家供应商 → 400，
// 不是一次失败的测试（前端会明确看到「重新拉取分组列表」的提示）。
func TestProviderModelTestHandler_ForeignGroupRejected(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{}, nil, nil, nil))
	t.Cleanup(srv.Close)

	other := seedProviderRow(t, "p-other", "openai")
	_, otherGroup := seedPoolGroup(t, cipher, other.ID, "别家组", "", "sk-x")
	provider := seedProviderRow(t, "p-mine", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)

	w := callProviderModelTest(t, provider.ID, `{"model":"m1","group_id":`+strconv.FormatUint(uint64(otherGroup.ID), 10)+`}`)
	if got := gjson.Get(w.Body.String(), "code").Int(); got != 400 {
		t.Fatalf("code = %d, want 400; body=%s", got, w.Body.String())
	}
	if msg := gjson.Get(w.Body.String(), "message").String(); !strings.Contains(msg, "不属于供应商") {
		t.Fatalf("message = %q, want 含「不属于供应商」", msg)
	}
}

// TestTestStructuredOutputHandler_NoEndpointFailsAsProviderError 链路装配失败（有分组但
// 无启用端点）在结构化输出端点上归到 error_type=provider。
//
// 该用例原为 TestTestStructuredOutputHandler_ProviderCreationFailure（锁 providers.New
// 失败分支）。#19 起客户端由 channel 诊断装配产出，「type 未注册」不再是这条链路的失败
// 方式（注册类型按端点协议派生），旧断言的「创建提供商失败」文案随之作废；这里显式保留
// 「配置不可用 → error_type=provider + passed=false」这层契约，落在新的失败方式上。
func TestTestStructuredOutputHandler_NoEndpointFailsAsProviderError(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	provider := seedProviderRow(t, "broken-provider", "openai")
	// 有分组有凭据、但一个端点都没配 → 失败点落在「无启用端点」而非「无分组」。
	_, _ = seedPoolGroup(t, cipher, provider.ID, "有凭据", "", "sk-x")
	mwp := seedAssociation(t, provider.ID, 1, "gpt-x")

	c, w := testsupport.NewTestContext("GET", "/api/test/structured-output/"+strconv.FormatUint(uint64(mwp.ID), 10))
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(mwp.ID), 10)}}
	TestStructuredOutputHandler(c)

	payload := loadEnvelope(t, w)
	if payload.Code != 200 {
		t.Fatalf("envelope code = %d, want 200; body=%s", payload.Code, w.Body.String())
	}
	if passed, _ := payload.Data["passed"].(bool); passed {
		t.Fatalf("data.passed = %v, want false", payload.Data["passed"])
	}
	if got := gjson.Get(w.Body.String(), "data.error_type").String(); got != "provider" {
		t.Fatalf("error_type = %q, want provider", got)
	}
	if got := gjson.Get(w.Body.String(), "data.error").String(); !strings.Contains(got, "无启用的 openai 协议端点") {
		t.Fatalf("error = %q, want 含「无启用的 openai 协议端点」", got)
	}
}

// TestTestReactHandler_UsesPoolCredential react 端点（SSE）同样必须走诊断装配：
// 只断言「上游收到号池凭据明文」，场景是否通过取决于假上游的回复内容，不在本用例范围内。
func TestTestReactHandler_UsesPoolCredential(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	var captured atomic.Value
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{"Bearer sk-react": ""}, &hits, &captured, nil))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-react", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	_, _ = seedPoolGroup(t, cipher, provider.ID, "react 池组", "", "sk-react")
	mwp := seedAssociation(t, provider.ID, 1, "gpt-4.1")

	w := callReactTest(t, mwp.ID)

	if hits.Load() == 0 {
		t.Fatalf("react 测试未打到上游（装配阶段就失败）；SSE=%s", w.Body.String())
	}
	if got, _ := captured.Load().(string); got != "Bearer sk-react" {
		t.Fatalf("上游凭据头 = %q, want Bearer sk-react", got)
	}
	if body := w.Body.String(); strings.Contains(body, "装配上游链路失败") || strings.Contains(body, "创建提供商失败") {
		t.Fatalf("SSE 报出装配失败: %s", body)
	}
}
