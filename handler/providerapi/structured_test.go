package providerapi

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

const testCredHexKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func setupProviderCrypto(t *testing.T) {
	t.Helper()
	c, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("credentialcrypto.New: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
}

func structuredCreateBody(name string, inlineKey string) string {
	return `{
		"name":"` + name + `",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\",\"api_key\":\"SHOULD_STRIP\",\"_schedule\":{\"protocols\":[\"openai\"]}}",
		"console":"",
		"proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[{"id":0,"key":"` + inlineKey + `"}]}]
	}`
}

func createProviderViaHandler(t *testing.T, body string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	CreateProvider(c)
	if w.Code != 200 {
		t.Fatalf("CreateProvider status = %d, body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

func updateProviderViaHandler(t *testing.T, id uint, body string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
	c.Request = httptest.NewRequest("PUT", "/providers/"+strconv.FormatUint(uint64(id), 10), strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateProvider(c)
	if w.Code != 200 {
		t.Fatalf("UpdateProvider status = %d, body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

func TestStructured_CreateWritesFourTablesAndStripsConfig(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	body := createProviderViaHandler(t, structuredCreateBody("p-create", "sk-test-AAAA1111"))
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("envelope code = %s, body=%s", gjson.Get(body, "code").Raw, body)
	}
	id := uint(gjson.Get(body, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID, body=%s", body)
	}

	cfg := gjson.Get(body, "data.Config").String()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(cfg), &parsed); err != nil {
		t.Fatalf("config json: %v", err)
	}
	if _, ok := parsed["api_key"]; ok {
		t.Fatalf("config still has api_key: %s", cfg)
	}
	if _, ok := parsed["_schedule"]; ok {
		t.Fatalf("config still has _schedule: %s", cfg)
	}
	if parsed["base_url"] != "https://api.example.com" {
		t.Fatalf("base_url = %v", parsed["base_url"])
	}

	ctx := t.Context()
	eps, err := repository.Default().Endpoint.ListByProvider(ctx, id)
	if err != nil || len(eps) != 1 || eps[0].Protocol != "openai" {
		t.Fatalf("endpoints = %+v err=%v", eps, err)
	}
	groups, err := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups = %+v err=%v", groups, err)
	}
	creds, err := repository.Default().Credential.ListByGroups(ctx, []uint{groups[0].ID}, nil)
	if err != nil || len(creds) != 1 {
		t.Fatalf("creds = %+v err=%v", creds, err)
	}
	if got := gjson.Get(body, "data.Groups.0.InlineKeys.0.KeyMasked").String(); got != "sk-****1111" {
		t.Fatalf("InlineKeys item masked = %q, want sk-****1111 (plaintext must not leak): %s", got, body)
	}
	if gjson.Get(body, "data.Groups.0.InlineKeys.0.ID").Uint() == 0 {
		t.Fatalf("InlineKeys item missing ID: %s", body)
	}
	if strings.Contains(body, "sk-test-AAAA1111") {
		t.Fatalf("detail response leaked plaintext inline key: %s", body)
	}
}

func TestStructured_UpdatePoolToInlineClearsPoolID(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	ctx := t.Context()
	pool := &models.Pool{Name: "pool-for-provider"}
	if err := repository.Default().Pool.Create(ctx, pool); err != nil {
		t.Fatalf("create pool: %v", err)
	}

	createBody := `{
		"name":"p-pool",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"",
		"proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"池组","weight":2,"models":"","source":"pool","pool_id":` + strconv.FormatUint(uint64(pool.ID), 10) + `}]
	}`
	created := createProviderViaHandler(t, createBody)
	id := uint(gjson.Get(created, "data.ID").Uint())
	if gjson.Get(created, "data.Groups.0.PoolID").Type == gjson.Null {
		t.Fatalf("expected PoolID set on create: %s", created)
	}

	updateBody := `{
		"name":"p-pool",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"",
		"proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":false}],
		"groups":[{"name":"内联组","weight":1,"models":"","source":"inline","inline_keys":[{"id":0,"key":"sk-inline-BBBB2222"}]}]
	}`
	updated := updateProviderViaHandler(t, id, updateBody)
	code := gjson.Get(updated, "code")
	if !code.Exists() || code.Int() != 200 {
		t.Fatalf("update envelope code=%v body=%s", code.Raw, updated)
	}
	poolID := gjson.Get(updated, "data.Groups.0.PoolID")
	if !poolID.Exists() {
		t.Fatalf("PoolID key missing: %s", updated)
	}
	if poolID.Type != gjson.Null {
		t.Fatalf("PoolID = %s, want null", poolID.Raw)
	}
	if gjson.Get(updated, "data.Groups.0.Source").String() != "inline" {
		t.Fatalf("Source = %s", gjson.Get(updated, "data.Groups.0.Source").Raw)
	}
	if gjson.Get(updated, "data.Endpoints.0.Enabled").Bool() {
		t.Fatalf("Enabled should be false after update")
	}

	groups, _ := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if len(groups) != 1 || groups[0].PoolID != nil {
		t.Fatalf("db group PoolID = %+v", groups[0].PoolID)
	}
}

func TestStructured_CreateRejectsMissingSchedule(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers", strings.NewReader(`{
		"name":"bare","type":"openai","config":"{\"base_url\":\"https://x\"}","console":"","proxy":""
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	CreateProvider(c)

	var payload testsupport.APIEnvelope[any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Code != 400 {
		t.Fatalf("code = %d, want 400, body=%s", payload.Code, w.Body.String())
	}
}

// TestStructured_CreateInlineWithoutCipher_Returns400 锁定哨兵错误：加密未配置且提交
// inline 组时回 400（客户端可修复的配置缺失），而非 500（服务器故障）。防止 RunInTx
// 把业务校验错统一吞成 InternalServerError。
func TestStructured_CreateInlineWithoutCipher_Returns400(t *testing.T) {
	testsupport.InitTestDB(t)
	credentialcrypto.SetDefault(nil)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers", strings.NewReader(structuredCreateBody("p-nocipher", "sk-ctest-EEEE5555")))
	c.Request.Header.Set("Content-Type", "application/json")
	CreateProvider(c)

	if w.Code != 200 {
		t.Fatalf("status = %d（应为 200 信封内含 400）, body=%s", w.Code, w.Body.String())
	}
	var payload testsupport.APIEnvelope[any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 400 {
		t.Fatalf("code = %d, want 400, body=%s", payload.Code, w.Body.String())
	}
}

// TestStructured_CreateZeroValuesPersistCorrectly 锁定 GORM default 零值陷阱：
// Endpoint.Enabled gorm:"default:true" / KeyGroup.Weight gorm:"default:1" 在 struct
// Create 时会跳过零值（false / 0）走 DB default。创建 enabled:false 端点、weight:0 分组
// 必须落库为 false / 0。防止静默写错数据（曾导致 enabled:false 落成 true、weight:0 落成 1）。
func TestStructured_CreateZeroValuesPersistCorrectly(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	body := `{
		"name":"p-zeroval",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"",
		"proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":false}],
		"groups":[{"name":"零价组","weight":0,"models":"","source":"inline","inline_keys":[{"id":0,"key":"sk-zeroval-FFFF6666"}]}]
	}`
	created := createProviderViaHandler(t, body)
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	ctx := t.Context()
	eps, err := repository.Default().Endpoint.ListByProvider(ctx, id)
	if err != nil || len(eps) != 1 {
		t.Fatalf("endpoints=%+v err=%v", eps, err)
	}
	if eps[0].Enabled {
		t.Fatalf("endpoint enabled = true, want false（GORM default:true 零值陷阱未修）")
	}
	groups, err := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups=%+v err=%v", groups, err)
	}
	if groups[0].Weight != 0 {
		t.Fatalf("group weight = %d, want 0（GORM default:1 零值陷阱未修）", groups[0].Weight)
	}

	// 响应体也必须反映真实值（enabled=false、weight=0）
	if gjson.Get(created, "data.Endpoints.0.Enabled").Bool() {
		t.Fatalf("响应端点 enabled 应为 false: %s", created)
	}
	if gjson.Get(created, "data.Groups.0.Weight").Int() != 0 {
		t.Fatalf("响应分组 weight 应为 0: %s", created)
	}
}

// TestStructured_UpdatePartialKeepsChildren 锁定 partial-update：PUT 只带顶层标量
// （无 endpoints/groups 键）时，仅更新 provider 标量字段，子表（endpoints/groups/
// credentials）与 Protocols 原样保留。开关类动作（模型端点/过滤/拉黑切换）通过此语义
// 复用 PUT 而无需重提完整结构。防止 partial 请求被强制校验拒绝或误清空 children。
func TestStructured_UpdatePartialKeepsChildren(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	ctx := t.Context()
	created := createProviderViaHandler(t, structuredCreateBody("p-partial", "sk-partial-DDDD9999"))
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	// partial：只带顶层标量（含 config、name、开关），刻意省略 protocols/endpoints/groups 键。
	partialBody := `{
		"name":"p-partial-renamed",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"",
		"proxy":"",
		"model_endpoint":false,
		"model_filter_enabled":true,
		"blacklisted":true
	}`
	updated := updateProviderViaHandler(t, id, partialBody)
	if code := gjson.Get(updated, "code").Int(); code != 200 {
		t.Fatalf("update code = %d, body=%s", code, updated)
	}
	if gjson.Get(updated, "data.Name").String() != "p-partial-renamed" {
		t.Fatalf("Name not updated: %s", updated)
	}
	// 标量开关生效
	if gjson.Get(updated, "data.ModelEndpoint").Bool() {
		t.Fatalf("ModelEndpoint should be false: %s", updated)
	}
	if !gjson.Get(updated, "data.ModelFilterEnabled").Bool() {
		t.Fatalf("ModelFilterEnabled should be true: %s", updated)
	}
	if !gjson.Get(updated, "data.blacklisted").Bool() {
		t.Fatalf("blacklisted should be true: %s", updated)
	}
	// Protocols 保留
	if gjson.Get(updated, "data.Protocols.0").String() != "openai" {
		t.Fatalf("Protocols wiped by partial update: %s", updated)
	}

	// 子表保留：1 端点 + 1 分组 + 1 内联凭据原样
	eps, err := repository.Default().Endpoint.ListByProvider(ctx, id)
	if err != nil || len(eps) != 1 {
		t.Fatalf("endpoints after partial = %+v err=%v", eps, err)
	}
	groups, err := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups after partial = %+v err=%v", groups, err)
	}
	creds, err := repository.Default().Credential.ListByGroups(ctx, []uint{groups[0].ID}, nil)
	if err != nil || len(creds) != 1 {
		t.Fatalf("creds after partial = %+v err=%v", creds, err)
	}

	// 安全不变量：响应里 partial 后仍保留同一条内联 key 的掩码项（未被清空/覆盖），
	// 且明文绝不出现在响应里
	if got := gjson.Get(updated, "data.Groups.0.InlineKeys.0.KeyMasked").String(); got != "sk-****9999" {
		t.Fatalf("InlineKeys item masked after partial update = %q, want sk-****9999: %s", got, updated)
	}
	if strings.Contains(updated, "sk-partial-DDDD9999") {
		t.Fatalf("partial update response leaked plaintext inline key: %s", updated)
	}
}

func TestSanitizeConfig_StripsKeyAndSchedule(t *testing.T) {
	got, err := sanitizeConfig(`{"base_url":"https://x","api_key":"sk","_schedule":{"a":1},"beta":"b"}`)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["api_key"]; ok {
		t.Fatal("api_key not stripped")
	}
	if _, ok := m["_schedule"]; ok {
		t.Fatal("_schedule not stripped")
	}
	if m["base_url"] != "https://x" || m["beta"] != "b" {
		t.Fatalf("got %v", m)
	}
}

// TestStructured_UpdatePartialWithoutConfigKeepsConfig 锁 C1 回归：partial 请求
// 不带 config 键时，原 Config（base_url 等）必须原样保留。GORM struct Updates 的
// 零值跳过保护不了本场景——sanitizeConfig("") 经空 map 序列化产出 "{}"（非零值），
// 照样覆盖原值。修复后 partial 分支仅显式携带 config 时才写该列。
func TestStructured_UpdatePartialWithoutConfigKeepsConfig(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	created := createProviderViaHandler(t, structuredCreateBody("p-partial-cfg", "sk-cfg-AAAA1111"))
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	// 开关类 partial：只带 model_filter_enabled，不带 config 键（前端开关调用方的真实形态）
	updated := updateProviderViaHandler(t, id, `{"model_filter_enabled": true}`)
	if code := gjson.Get(updated, "code").Int(); code != 200 {
		t.Fatalf("update code = %d, body=%s", code, updated)
	}
	cfg := gjson.Get(updated, "data.Config").String()
	if cfg != `{"base_url":"https://api.example.com"}` {
		t.Fatalf("partial without config key overwrote Config to %q", cfg)
	}
}

// TestStructured_UpdateFullClearsProxyAndConsole 锁 W1 回归：全量更新显式传空
// console/proxy（用户在表单清空）必须落库生效。struct Updates 零值跳过会静默保留
// 旧值，全量路径改 UpdateFields(map) 后空串按显式清空写入。
func TestStructured_UpdateFullClearsProxyAndConsole(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	createBody := `{
		"name":"p-full-clear",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"https://console.example.com",
		"proxy":"http://127.0.0.1:7890",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[{"id":0,"key":"sk-clear-BBBB2222"}]}]
	}`
	created := createProviderViaHandler(t, createBody)
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	// 全量更新：children 全带，console/proxy 显式传空串（模拟用户清空表单）
	updateBody := `{
		"name":"p-full-clear",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"",
		"proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[{"id":0,"key":"sk-clear-BBBB2222"}]}]
	}`
	updated := updateProviderViaHandler(t, id, updateBody)
	if code := gjson.Get(updated, "code").Int(); code != 200 {
		t.Fatalf("update code = %d, body=%s", code, updated)
	}
	if proxy := gjson.Get(updated, "data.Proxy").String(); proxy != "" {
		t.Fatalf("clearing proxy did not persist (Proxy=%q)", proxy)
	}
	if console := gjson.Get(updated, "data.Console").String(); console != "" {
		t.Fatalf("clearing console did not persist (Console=%q)", console)
	}
}

// TestStructured_UpdateRejectsHalfChildren 锁 W2 回归：请求只带 endpoints/groups
// 其中一键（半 partial）时必须 400 拒绝，禁止静默丢弃该键。hasChildren 用 || 判定
// 「任一 children 键出现即全量意图」，缺失键交由 validateStructuredRequest 报错。
func TestStructured_UpdateRejectsHalfChildren(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	created := createProviderViaHandler(t, structuredCreateBody("p-half", "sk-half-CCCC3333"))
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	cases := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{
			name: "only endpoints without groups",
			body: `{
				"name":"p-half",
				"type":"openai",
				"protocols":["openai"],
				"endpoints":[{"protocol":"openai","url":"","enabled":true}]
			}`,
			wantMsg: "凭据分组",
		},
		{
			name: "only groups without endpoints",
			body: `{
				"name":"p-half",
				"type":"openai",
				"protocols":["openai"],
				"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[{"id":0,"key":"sk-half-CCCC3333"}]}]
			}`,
			wantMsg: "协议端点",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updated := updateProviderViaHandler(t, id, tc.body)
			if code := gjson.Get(updated, "code").Int(); code != 400 {
				t.Fatalf("half-partial should be rejected with code 400, got %d, body=%s", code, updated)
			}
			if msg := gjson.Get(updated, "message").String(); !strings.Contains(msg, tc.wantMsg) {
				t.Fatalf("error message should mention %q, got %q", tc.wantMsg, msg)
			}
		})
	}
}

// getGroupInlineKeysRawRecorder 返回完整 recorder：明文端点除 body/code 外还要断言响应头
func getGroupInlineKeysRawRecorder(t *testing.T, providerID, groupID uint) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "id", Value: strconv.FormatUint(uint64(providerID), 10)},
		{Key: "groupId", Value: strconv.FormatUint(uint64(groupID), 10)},
	}
	c.Request = httptest.NewRequest("GET", "/providers/:id/groups/:groupId/inline-keys/raw", nil)
	GetGroupInlineKeys(c)
	return w
}

func getGroupInlineKeysRawViaHandler(t *testing.T, providerID, groupID uint) (string, int) {
	t.Helper()
	w := getGroupInlineKeysRawRecorder(t, providerID, groupID)
	return w.Body.String(), w.Code
}

// TestStructured_GroupRawRevealsPlaintext 锁定受控明文端点：详情只出掩码，raw 显式
// 拉取才返回明文 keys（顺序与详情一致，按凭据 id ASC）；明文不出常规响应；响应禁缓存。
func TestStructured_GroupRawRevealsPlaintext(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	created := createProviderViaHandler(t, structuredCreateBody("p-raw", "sk-raw-AAAA1111"))
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}
	groupID := uint(gjson.Get(created, "data.Groups.0.ID").Uint())

	// 详情及其二次 GET 均不得含明文
	for _, body := range []string{created, getProviderViaHandler(t, id)} {
		if got := gjson.Get(body, "data.Groups.0.InlineKeys.0.KeyMasked").String(); got != "sk-****1111" {
			t.Fatalf("KeyMasked=%q want sk-****1111, body=%s", got, body)
		}
		if strings.Contains(body, "sk-raw-AAAA1111") {
			t.Fatalf("detail leaked plaintext: %s", body)
		}
	}

	w := getGroupInlineKeysRawRecorder(t, id, groupID)
	body, code := w.Body.String(), w.Code
	if code != 200 {
		t.Fatalf("raw status=%d body=%s", code, body)
	}
	if got := gjson.Get(body, "data.keys.0.key").String(); got != "sk-raw-AAAA1111" {
		t.Fatalf("raw keys[0].key=%q want sk-raw-AAAA1111, body=%s", got, body)
	}
	if gjson.Get(body, "data.keys.0.id").Uint() == 0 {
		t.Fatalf("raw item missing credential id (前端按 id 归位依赖它): %s", body)
	}
	// 明文响应禁缓存：落进浏览器磁盘缓存或中间代理，前端的明文生命周期守卫就形同虚设
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q want no-store, body=%s", got, body)
	}
}

// TestStructured_UpdateByIDKeepsAndReplaces 锁定写路径 id 语义（掩码回填的保存形态）：
// 按 id 保留（key 空）/ 替换（key 非空且 hash 变）/ 新增（id=0）/ 未提交 = 删除。
// 明文永不出响应，含明文 key 的请求体不因事务失败等路径泄漏。
func TestStructured_UpdateByIDKeepsAndReplaces(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	created := createProviderViaHandler(t, `{
		"name":"p-idref",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[
			{"id":0,"key":"sk-keep-AAAA1111"},
			{"id":0,"key":"sk-drop-BBBB2222"}
		]}]
	}`)
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	// 读取两条既有凭据的 id
	ctx := t.Context()
	groups, err := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups=%+v err=%v", groups, err)
	}
	creds, err := repository.Default().Credential.ListByGroups(ctx, []uint{groups[0].ID}, nil)
	if err != nil || len(creds) != 2 {
		t.Fatalf("creds=%+v err=%v", creds, err)
	}
	// ListByGroups 按 id ASC：先提交的 sk-keep 为 creds[0]，sk-drop 为 creds[1]
	keepID, dropID := creds[0].ID, creds[1].ID

	// 保存形态：keep 原样保留（key 空）、drop 不提交（删除）、新增一条、keep 也可替换
	updated := updateProviderViaHandler(t, id, `{
		"name":"p-idref",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[
			{"id":`+strconv.FormatUint(uint64(keepID), 10)+`,"key":""},
			{"id":0,"key":"sk-add-CCCC3333"}
		]}]
	}`)
	if code := gjson.Get(updated, "code").Int(); code != 200 {
		t.Fatalf("update code=%d body=%s", code, updated)
	}

	// 全量同步会重建分组：重新读取新组 ID（旧 group 已软删，凭据已迁移到新组）
	groupsAfter, err := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(groupsAfter) != 1 {
		t.Fatalf("groups after=%+v err=%v", groupsAfter, err)
	}
	newGroupID := groupsAfter[0].ID

	// DB 侧：保留 + 新增 = 2 条，drop 已删；raw 明文顺序 = [keep, add]（id ASC）
	credsAfter, err := repository.Default().Credential.ListByGroups(ctx, []uint{newGroupID}, nil)
	if err != nil || len(credsAfter) != 2 {
		t.Fatalf("creds after=%+v err=%v", credsAfter, err)
	}
	for _, c := range credsAfter {
		if c.ID == dropID {
			t.Fatalf("dropped credential %d still exists", dropID)
		}
	}
	raw, code := getGroupInlineKeysRawViaHandler(t, id, newGroupID)
	if code != 200 {
		t.Fatalf("raw status=%d body=%s", code, raw)
	}
	if got := gjson.Get(raw, "data.keys.0.key").String(); got != "sk-keep-AAAA1111" {
		t.Fatalf("raw keys[0].key=%q want sk-keep-AAAA1111（保留语义失效）", got)
	}
	if got := gjson.Get(raw, "data.keys.1.key").String(); got != "sk-add-CCCC3333" {
		t.Fatalf("raw keys[1].key=%q want sk-add-CCCC3333, body=%s", got, raw)
	}

	// 替换语义：对 keep 那条提交新明文（hash 变 → 更新密文），旧明文消失
	updated2 := updateProviderViaHandler(t, id, `{
		"name":"p-idref",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[
			{"id":`+strconv.FormatUint(uint64(keepID), 10)+`,"key":"sk-replaced-DDDD4444"},
			{"id":0,"key":"sk-add-CCCC3333"}
		]}]
	}`)
	if code := gjson.Get(updated2, "code").Int(); code != 200 {
		t.Fatalf("update2 code=%d body=%s", code, updated2)
	}
	// 全量同步再次重建分组：用最新组 ID 拉 raw
	groupsAfter2, err := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(groupsAfter2) != 1 {
		t.Fatalf("groups after2=%+v err=%v", groupsAfter2, err)
	}
	raw2, code := getGroupInlineKeysRawViaHandler(t, id, groupsAfter2[0].ID)
	if code != 200 {
		t.Fatalf("raw2 status=%d body=%s", code, raw2)
	}
	if got := gjson.Get(raw2, "data.keys.0.key").String(); got != "sk-replaced-DDDD4444" {
		t.Fatalf("raw2 keys[0].key=%q want sk-replaced-DDDD4444（替换语义失效）", got)
	}
	if got := gjson.Get(raw2, "data.keys.1.key").String(); got != "sk-add-CCCC3333" {
		t.Fatalf("raw2 keys[1].key=%q want sk-add-CCCC3333, body=%s", got, raw2)
	}
	if strings.Contains(raw2, "sk-keep-AAAA1111") || strings.Contains(updated2, "sk-keep-AAAA1111") {
		t.Fatalf("替换后旧明文泄漏: %s", raw2)
	}
}

// TestStructured_UpdateReferencesMissingCredential_400 锁定 id 引用守卫：提交引用
// 本供应商不存在的凭据 id 必须 400（防越权/幻觉引用写入空凭证）。
func TestStructured_UpdateReferencesMissingCredential_400(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	created := createProviderViaHandler(t, structuredCreateBody("p-badref", "sk-badref-AAAA1111"))
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	updated := updateProviderViaHandler(t, id, `{
		"name":"p-badref",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[
			{"id":999999,"key":""}
		]}]
	}`)
	if code := gjson.Get(updated, "code").Int(); code != 400 {
		t.Fatalf("boss reference should be rejected with 400, got %d body=%s", code, updated)
	}
}

// TestStructured_GroupRaw_PoolGroupAndCrossProvider 锁定 raw 端点边界：
// pool 来源分组无内联明文（400）、越组访问不暴露存在性（404）。
func TestStructured_GroupRaw_PoolGroupAndCrossProvider(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	ctx := t.Context()
	pool := &models.Pool{Name: "pool-for-raw"}
	if err := repository.Default().Pool.Create(ctx, pool); err != nil {
		t.Fatalf("create pool: %v", err)
	}
	created := createProviderViaHandler(t, `{
		"name":"p-rawpool",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"池组","weight":1,"models":"","source":"pool","pool_id":`+strconv.FormatUint(uint64(pool.ID), 10)+`}]
	}`)
	id := uint(gjson.Get(created, "data.ID").Uint())
	poolGroupID := uint(gjson.Get(created, "data.Groups.0.ID").Uint())

	// pool 来源分组：无内联明文，raw 400
	body, code := getGroupInlineKeysRawViaHandler(t, id, poolGroupID)
	if code != 200 || gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("pool group raw should be 400, status=%d body=%s", code, body)
	}

	// 另一个无该分组的 provider：raw 404（不暴露分组存在性）
	created2 := createProviderViaHandler(t, structuredCreateBody("p-raw2", "sk-raw2-AAAA1111"))
	id2 := uint(gjson.Get(created2, "data.ID").Uint())
	body2, code2 := getGroupInlineKeysRawViaHandler(t, id2, poolGroupID)
	if code2 != 200 || gjson.Get(body2, "code").Int() != 404 {
		t.Fatalf("cross-provider raw should be 404, status=%d body=%s", code2, body2)
	}
}

// TestStructured_GroupRaw_ProviderNotFound_404 锁定供应商不存在时 raw 走 404 信封
// （先校验供应商再校验分组，不给「分组存在但供应商不存在」的旁路探测）。
func TestStructured_GroupRaw_ProviderNotFound_404(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	created := createProviderViaHandler(t, structuredCreateBody("p-raw404", "sk-raw404-AAAA1111"))
	groupID := uint(gjson.Get(created, "data.Groups.0.ID").Uint())

	body, code := getGroupInlineKeysRawViaHandler(t, 999999, groupID)
	if code != 200 || gjson.Get(body, "code").Int() != 404 {
		t.Fatalf("provider 不存在 raw 应 404 信封, status=%d body=%s", code, body)
	}
}

// TestStructured_GroupRaw_CipherNotConfigured_500 锁定加密未配置时 raw 直接 HTTP 500：
// 明文拉取没有降级路径，绝不能回退成掩码或空数组让前端误判「该组无 key」。
func TestStructured_GroupRaw_CipherNotConfigured_500(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	created := createProviderViaHandler(t, structuredCreateBody("p-rawnocipher", "sk-rawnocipher-AAAA1111"))
	id := uint(gjson.Get(created, "data.ID").Uint())
	groupID := uint(gjson.Get(created, "data.Groups.0.ID").Uint())

	credentialcrypto.SetDefault(nil)
	body, code := getGroupInlineKeysRawViaHandler(t, id, groupID)
	if code != 500 {
		t.Fatalf("cipher 未配置 raw 应 HTTP 500, got %d body=%s", code, body)
	}
	if strings.Contains(body, "sk-rawnocipher-AAAA1111") {
		t.Fatalf("500 响应泄漏明文: %s", body)
	}
}

// TestStructured_GroupRaw_DecryptFailure_500 锁定「任一条解密失败整组拒绝」：明文替换是
// 覆盖动作，部分成功会让前端把无法显示的条目静默替换掉。同时验证分工——详情路径对
// 损坏密文退化为 **** 掩码（仍 200 可用、可按 id 保留），只有 raw 拒绝。
func TestStructured_GroupRaw_DecryptFailure_500(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	created := createProviderViaHandler(t, `{
		"name":"p-rawfail",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[
			{"id":0,"key":"sk-rawfail-AAAA1111"},
			{"id":0,"key":"sk-rawfail-BBBB2222"}
		]}]
	}`)
	id := uint(gjson.Get(created, "data.ID").Uint())
	groupID := uint(gjson.Get(created, "data.Groups.0.ID").Uint())

	// 损坏第一条密文（非 hex(nonce‖ciphertext)，GCM 解不开）
	ctx := t.Context()
	creds, err := repository.Default().Credential.ListByGroups(ctx, []uint{groupID}, nil)
	if err != nil || len(creds) != 2 {
		t.Fatalf("creds=%+v err=%v", creds, err)
	}
	if _, err := repository.Default().Credential.UpdateFields(ctx, creds[0].ID, map[string]any{"key": "not-a-valid-ciphertext"}); err != nil {
		t.Fatalf("corrupt credential: %v", err)
	}

	body, code := getGroupInlineKeysRawViaHandler(t, id, groupID)
	if code != 500 {
		t.Fatalf("解密失败 raw 应 HTTP 500（整组拒绝）, got %d body=%s", code, body)
	}
	// 整组拒绝：另一条能解开的明文也不得出现在响应里
	if strings.Contains(body, "sk-rawfail-BBBB2222") || strings.Contains(body, "sk-rawfail-AAAA1111") {
		t.Fatalf("整组拒绝却返回了部分明文: %s", body)
	}

	// 详情路径不受影响：损坏条退化 **** 掩码，正常条照常掩码，仍 200
	detailBody := getProviderViaHandler(t, id)
	if gjson.Get(detailBody, "code").Int() != 200 {
		t.Fatalf("详情应仍 200: %s", detailBody)
	}
	if got := gjson.Get(detailBody, "data.Groups.0.InlineKeys.0.KeyMasked").String(); got != "****" {
		t.Fatalf("损坏密文应退化 ****，got %q body=%s", got, detailBody)
	}
	if got := gjson.Get(detailBody, "data.Groups.0.InlineKeys.1.KeyMasked").String(); got != "sk-****2222" {
		t.Fatalf("正常条掩码应不受影响，got %q body=%s", got, detailBody)
	}
}

// TestStructured_RejectsDuplicateReferenceID_400 锁定同一凭据被引用两次必须 400：
// 一条凭据只能归属一个分组位置，重复引用会让保留/替换语义二义。
func TestStructured_RejectsDuplicateReferenceID_400(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	created := createProviderViaHandler(t, structuredCreateBody("p-dupref", "sk-dupref-AAAA1111"))
	id := uint(gjson.Get(created, "data.ID").Uint())
	ctx := t.Context()
	groups, err := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups=%+v err=%v", groups, err)
	}
	creds, err := repository.Default().Credential.ListByGroups(ctx, []uint{groups[0].ID}, nil)
	if err != nil || len(creds) != 1 {
		t.Fatalf("creds=%+v err=%v", creds, err)
	}
	credID := strconv.FormatUint(uint64(creds[0].ID), 10)

	updated := updateProviderViaHandler(t, id, `{
		"name":"p-dupref",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[
			{"id":`+credID+`,"key":""},
			{"id":`+credID+`,"key":""}
		]}]
	}`)
	if code := gjson.Get(updated, "code").Int(); code != 400 {
		t.Fatalf("重复引用应 400, got %d body=%s", code, updated)
	}
	if msg := gjson.Get(updated, "message").String(); !strings.Contains(msg, "重复引用") {
		t.Fatalf("错误文案应说明重复引用, got %q", msg)
	}
}

// TestStructured_RejectsDuplicatePlaintext_400 锁定同组重复明文 400（对齐号池「同池重复
// KeyHash 400」）：静默落两条同 KeyHash 凭据会让组内轮询给同一 key 双份流量权重。
// 错误文案不得回显明文 key（防敏感值经 message 外泄到浏览器/日志）。
func TestStructured_RejectsDuplicatePlaintext_400(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/providers", strings.NewReader(`{
		"name":"p-dupplain",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[
			{"id":0,"key":"sk-dupplain-AAAA1111"},
			{"id":0,"key":" sk-dupplain-AAAA1111 "}
		]}]
	}`))
	c.Request.Header.Set("Content-Type", "application/json")
	CreateProvider(c)

	body := w.Body.String()
	if code := gjson.Get(body, "code").Int(); code != 400 {
		t.Fatalf("重复明文应 400, got %d body=%s", code, body)
	}
	if strings.Contains(body, "sk-dupplain-AAAA1111") {
		t.Fatalf("错误响应不得回显明文 key: %s", body)
	}
	// 未落库：供应商不存在（校验在事务前拦截）
	if exists, err := repository.Default().Provider.ExistsByName(t.Context(), "p-dupplain"); err != nil || exists {
		t.Fatalf("重复明文被拒后不应创建供应商: exists=%v err=%v", exists, err)
	}
}
