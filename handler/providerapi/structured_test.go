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
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":["` + inlineKey + `"]}]
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
	if gjson.Get(body, "data.Groups.0.InlineKeys.0").String() != "sk-test-AAAA1111" {
		t.Fatalf("InlineKeys not decrypted in response: %s", body)
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
		"groups":[{"name":"内联组","weight":1,"models":"","source":"inline","inline_keys":["sk-inline-BBBB2222"]}]
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
		"groups":[{"name":"零价组","weight":0,"models":"","source":"inline","inline_keys":["sk-zeroval-FFFF6666"]}]
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

	// 安全不变量：响应里 partial 后仍保留同一条内联 key（未被清空/覆盖）
	if gjson.Get(updated, "data.Groups.0.InlineKeys.0").String() != "sk-partial-DDDD9999" {
		t.Fatalf("InlineKeys not preserved: %s", updated)
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
