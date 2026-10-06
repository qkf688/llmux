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

// seedPool 建号池并返回 ID。内联凭据概念已删除，供应商分组一律经 KeyGroup.PoolID
// 引用号池，供应商表单自身不持有任何凭据——所以每条结构化用例都先备一个池。
func seedPool(t *testing.T, name string) uint {
	t.Helper()
	pool := &models.Pool{Name: name}
	if err := repository.Default().Pool.Create(t.Context(), pool); err != nil {
		t.Fatalf("seedPool %q: %v", name, err)
	}
	return pool.ID
}

// seedPoolCredential 往池里放一条凭据（密文 + hash），模拟号池页维护出的真实数据。
func seedPoolCredential(t *testing.T, poolID uint, plainKey string) models.Credential {
	t.Helper()
	cipher, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("credentialcrypto.New: %v", err)
	}
	enc, err := cipher.Encrypt(plainKey)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	cred := &models.Credential{
		PoolID:  poolID,
		Key:     enc,
		KeyHash: cipher.Hash(plainKey),
		Status:  models.CredentialStatusActive,
	}
	if err := repository.Default().Credential.Create(t.Context(), cred); err != nil {
		t.Fatalf("seedPoolCredential: %v", err)
	}
	return *cred
}

// structuredCreateBody 标准全量写入体：分组只带 pool_id 引用号池，不含任何凭据字段。
func structuredCreateBody(name string, poolID uint) string {
	return `{
		"name":"` + name + `",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\",\"api_key\":\"SHOULD_STRIP\",\"_schedule\":{\"protocols\":[\"openai\"]}}",
		"console":"",
		"proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"pool_id":` + strconv.FormatUint(uint64(poolID), 10) + `}]
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

// TestStructured_CreateWritesChildrenAndStripsConfig 锁 Create 的三条落库副作用：
// config 只存 adapter 字段（api_key/_schedule 已迁出）、端点写入成功、分组落库并指向
// 请求给定的号池。分组不再内嵌凭据，故响应不含任何 InlineKeys 键。
func TestStructured_CreateWritesChildrenAndStripsConfig(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolID := seedPool(t, "pool-create")
	body := createProviderViaHandler(t, structuredCreateBody("p-create", poolID))
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
	if groups[0].PoolID == nil || *groups[0].PoolID != poolID {
		t.Fatalf("group PoolID = %v, want %d（分组必须引用请求给定的号池）", groups[0].PoolID, poolID)
	}
	if got := gjson.Get(body, "data.Groups.0.PoolID").Uint(); got != uint64(poolID) {
		t.Fatalf("响应 PoolID = %d, want %d: %s", got, poolID, body)
	}
	// 供应商表单不再持有凭据：Source / InlineKeys 两键必须彻底消失，
	// 否则前端仍会按内联语义渲染出一块永不回填的凭据区。
	for _, key := range []string{"Source", "InlineKeys"} {
		if gjson.Get(body, "data.Groups.0."+key).Exists() {
			t.Fatalf("响应分组不应含 %s 键（内联凭据已删除）: %s", key, body)
		}
	}
}

// TestStructured_UpdateSwitchesPoolID 锁「换号池引用」这条路径：分组带 id 提交时走就地
// 更新，pool_id 从 A 改写到 B（原语义是「池 ↔ 内联互切」，内联删除后只剩换池引用，
// 走的仍是同一条 UpdateFields 写路径），同一次保存里的端点开关同步落库。
func TestStructured_UpdateSwitchesPoolID(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolA := seedPool(t, "pool-switch-a")
	poolB := seedPool(t, "pool-switch-b")

	created := createProviderViaHandler(t, structuredCreateBody("p-switch", poolA))
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}
	ctx := t.Context()
	groups, err := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups = %+v err=%v", groups, err)
	}
	gid := groups[0].ID

	updated := updateProviderViaHandler(t, id, `{
		"name":"p-switch",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":false}],
		"groups":[{"id":`+strconv.FormatUint(uint64(gid), 10)+`,"name":"换池组","weight":1,"pool_id":`+strconv.FormatUint(uint64(poolB), 10)+`}]
	}`)
	if code := gjson.Get(updated, "code").Int(); code != 200 {
		t.Fatalf("update code = %d, body=%s", code, updated)
	}

	after, err := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(after) != 1 {
		t.Fatalf("groups after = %+v err=%v", after, err)
	}
	if after[0].ID != gid {
		t.Fatalf("group ID = %d, want %d（换池必须就地更新，不得删光重建）", after[0].ID, gid)
	}
	if after[0].PoolID == nil || *after[0].PoolID != poolB {
		t.Fatalf("group PoolID = %v, want %d", after[0].PoolID, poolB)
	}
	if got := gjson.Get(updated, "data.Groups.0.PoolID").Uint(); got != uint64(poolB) {
		t.Fatalf("响应 PoolID = %d, want %d: %s", got, poolB, updated)
	}
	eps, err := repository.Default().Endpoint.ListByProvider(ctx, id)
	if err != nil || len(eps) != 1 {
		t.Fatalf("endpoints after = %+v err=%v", eps, err)
	}
	if eps[0].Enabled {
		t.Fatalf("endpoint enabled 落库为 true，want false")
	}
	if gjson.Get(updated, "data.Endpoints.0.Enabled").Bool() {
		t.Fatalf("响应 endpoint enabled 应为 false: %s", updated)
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

// TestStructured_CreateZeroValuesPersistCorrectly 锁定 GORM default 零值陷阱：
// Endpoint.Enabled gorm:"default:true" / KeyGroup.Weight gorm:"default:1" 在 struct
// Create 时会跳过零值（false / 0）走 DB default。创建 enabled:false 端点、weight:0 分组
// 必须落库为 false / 0。防止静默写错数据（曾导致 enabled:false 落成 true、weight:0 落成 1）。
func TestStructured_CreateZeroValuesPersistCorrectly(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolID := seedPool(t, "pool-zeroval")
	body := `{
		"name":"p-zeroval",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":false}],
		"groups":[{"name":"零价组","weight":0,"pool_id":` + strconv.FormatUint(uint64(poolID), 10) + `}]
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
	if groups[0].PoolID == nil || *groups[0].PoolID != poolID {
		t.Fatalf("group PoolID = %v, want %d", groups[0].PoolID, poolID)
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
// （无 endpoints/groups 键）时，仅更新 provider 标量字段，子表（endpoints/groups）
// 与 Protocols 原样保留，所引用号池内的凭据也不受牵连。开关类动作（模型端点/过滤/
// 拉黑切换）通过此语义复用 PUT 而无需重提完整结构。
func TestStructured_UpdatePartialKeepsChildren(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolID := seedPool(t, "pool-partial")
	seedPoolCredential(t, poolID, "sk-partial-DDDD9999")

	created := createProviderViaHandler(t, structuredCreateBody("p-partial", poolID))
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	// partial：只带顶层标量（含 config、name、开关），刻意省略 protocols/endpoints/groups 键。
	partialBody := `{
		"name":"p-partial-renamed",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
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

	// 子表保留：1 端点 + 1 分组，且分组仍指向同一个池
	ctx := t.Context()
	eps, err := repository.Default().Endpoint.ListByProvider(ctx, id)
	if err != nil || len(eps) != 1 {
		t.Fatalf("endpoints after partial = %+v err=%v", eps, err)
	}
	groups, err := repository.Default().KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups after partial = %+v err=%v", groups, err)
	}
	if groups[0].PoolID == nil || *groups[0].PoolID != poolID {
		t.Fatalf("partial update 改动了分组池引用 = %v, want %d", groups[0].PoolID, poolID)
	}
	if got := gjson.Get(updated, "data.Groups.0.PoolID").Uint(); got != uint64(poolID) {
		t.Fatalf("响应 PoolID = %d, want %d: %s", got, poolID, updated)
	}
	// 安全不变量：供应商响应不含任何凭据明文或掩码项
	if gjson.Get(updated, "data.Groups.0.InlineKeys").Exists() {
		t.Fatalf("partial update 响应不应展开凭据项: %s", updated)
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

	poolID := seedPool(t, "pool-cfg")
	created := createProviderViaHandler(t, structuredCreateBody("p-partial-cfg", poolID))
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

	poolID := seedPool(t, "pool-full-clear")
	poolStr := strconv.FormatUint(uint64(poolID), 10)
	createBody := `{
		"name":"p-full-clear",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"https://console.example.com",
		"proxy":"http://127.0.0.1:7890",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"pool_id":` + poolStr + `}]
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
		"groups":[{"name":"默认组","weight":1,"pool_id":` + poolStr + `}]
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

	poolID := seedPool(t, "pool-half")
	created := createProviderViaHandler(t, structuredCreateBody("p-half", poolID))
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
				"groups":[{"name":"默认组","weight":1,"pool_id":` + strconv.FormatUint(uint64(poolID), 10) + `}]
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

// TestStructured_UpdateGroupRequiresPoolID_400 锁分组必填号池引用：内联凭据删除后分组
// 只能挂在号池上，pool_id 缺失或为 0 一律 400。漏校验会造出「有分组、无凭据来源」的
// 僵尸分组，运行时表现为该组永远选不出凭据。
func TestStructured_UpdateGroupRequiresPoolID_400(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolID := seedPool(t, "pool-required")
	created := createProviderViaHandler(t, structuredCreateBody("p-poolreq", poolID))
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	cases := []struct {
		name  string
		group string
	}{
		{name: "pool_id key missing", group: `{"name":"默认组","weight":1}`},
		{name: "pool_id zero", group: `{"name":"默认组","weight":1,"pool_id":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updated := updateProviderViaHandler(t, id, `{
				"name":"p-poolreq",
				"type":"openai",
				"config":"{\"base_url\":\"https://api.example.com\"}",
				"console":"","proxy":"",
				"protocols":["openai"],
				"endpoints":[{"protocol":"openai","url":"","enabled":true}],
				"groups":[`+tc.group+`]
			}`)
			if code := gjson.Get(updated, "code").Int(); code != 400 {
				t.Fatalf("缺号池引用应 400, got %d body=%s", code, updated)
			}
			if msg := gjson.Get(updated, "message").String(); !strings.Contains(msg, "pool_id is required") {
				t.Fatalf("错误文案应说明 pool_id 必填, got %q", msg)
			}
		})
	}

	// 校验在进事务前完成：两次拒绝不得动到既有分组
	groups, err := repository.Default().KeyGroup.ListByProvider(t.Context(), id)
	if err != nil || len(groups) != 1 {
		t.Fatalf("被拒请求改动了分组: %+v err=%v", groups, err)
	}
}

// TestStructured_UpdateGroupRejectsUnknownPool_400 锁号池存在性校验：引用已删除/幻觉的
// pool_id 必须 400，不允许挂一个查不到凭据的池——那会让分组静默退化、空跑到期。
func TestStructured_UpdateGroupRejectsUnknownPool_400(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolID := seedPool(t, "pool-unknown-guard")
	created := createProviderViaHandler(t, structuredCreateBody("p-poolmiss", poolID))
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	updated := updateProviderViaHandler(t, id, `{
		"name":"p-poolmiss",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"pool_id":999999}]
	}`)
	if code := gjson.Get(updated, "code").Int(); code != 400 {
		t.Fatalf("引用不存在的号池应 400, got %d body=%s", code, updated)
	}
	if msg := gjson.Get(updated, "message").String(); !strings.Contains(msg, "not found") {
		t.Fatalf("错误文案应说明号池不存在, got %q", msg)
	}

	groups, err := repository.Default().KeyGroup.ListByProvider(t.Context(), id)
	if err != nil || len(groups) != 1 || groups[0].PoolID == nil || *groups[0].PoolID != poolID {
		t.Fatalf("被拒请求改动了分组: %+v err=%v", groups, err)
	}
}

// TestStructured_DeletingGroupKeepsPoolCredentials 锁本次模型变更的核心不变量：
// 凭据归号池所有，分组只是引用者。保存时删掉一个分组不得级联删掉号池本身或池内凭据——
// 同一个池常被多个供应商的分组复用，误删会连带打断别的供应商（表现为它们的分组集体空跑）。
func TestStructured_DeletingGroupKeepsPoolCredentials(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolID := seedPool(t, "pool-shared")
	cred := seedPoolCredential(t, poolID, "sk-keep-AAAA1111")
	poolStr := strconv.FormatUint(uint64(poolID), 10)

	created := createProviderViaHandler(t, `{
		"name":"p-keepcred",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[
			{"name":"组A","weight":1,"pool_id":`+poolStr+`},
			{"name":"组B","weight":1,"pool_id":`+poolStr+`}
		]
	}`)
	id := uint(gjson.Get(created, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	ctx := t.Context()
	repo := repository.Default()
	groups, err := repo.KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(groups) != 2 {
		t.Fatalf("groups = %+v err=%v", groups, err)
	}
	byName := map[string]models.KeyGroup{}
	for _, g := range groups {
		byName[g.Name] = g
	}
	groupA, ok := byName["组A"]
	if !ok {
		t.Fatalf("组A missing: %+v", groups)
	}
	dropID := byName["组B"].ID

	// 只提交组A（带 id 就地更新）→ 组B 被删除
	updated := updateProviderViaHandler(t, id, `{
		"name":"p-keepcred",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"id":`+strconv.FormatUint(uint64(groupA.ID), 10)+`,"name":"组A","weight":1,"pool_id":`+poolStr+`}]
	}`)
	if code := gjson.Get(updated, "code").Int(); code != 200 {
		t.Fatalf("update code = %d, body=%s", code, updated)
	}

	after, err := repo.KeyGroup.ListByProvider(ctx, id)
	if err != nil || len(after) != 1 {
		t.Fatalf("groups after = %+v err=%v", after, err)
	}
	if after[0].ID != groupA.ID {
		t.Fatalf("保留组 ID = %d, want %d", after[0].ID, groupA.ID)
	}
	if after[0].ID == dropID {
		t.Fatalf("组B 未被删除，删除语义失效")
	}

	// 号池与池内凭据必须原封不动
	if _, err := repo.Pool.Get(ctx, poolID); err != nil {
		t.Fatalf("删分组连带删了号池: %v", err)
	}
	creds, err := repo.Credential.ListByPools(ctx, []uint{poolID})
	if err != nil || len(creds) != 1 {
		t.Fatalf("删分组连带删了池内凭据: creds=%+v err=%v", creds, err)
	}
	if creds[0].ID != cred.ID {
		t.Fatalf("凭据 ID = %d, want %d（不得被重建）", creds[0].ID, cred.ID)
	}
}

// TestStructured_GroupSync_PreservesWhitelistAndIdentity 锁「白名单唯一写入方 = 模型同步」：
// 供应商全量保存（组带 id）不得触碰同步写入的 KeyGroup.Models，且分组就地更新、ID 稳定
// （禁止删光重建——重建会把同步白名单冲成空（空=不限）并让组 ID 漂移）。
// 组 id=0 = 新建（白名单空 = 未同步/不限）；未提交的组删除。
func TestStructured_GroupSync_PreservesWhitelistAndIdentity(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolA := seedPool(t, "pool-wl-a")
	poolB := seedPool(t, "pool-wl-b")
	created := createProviderViaHandler(t, structuredCreateBody("p-whitelist", poolA))
	providerID := uint(gjson.Get(created, "data.ID").Uint())
	if providerID == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	ctx := t.Context()
	repo := repository.Default()
	groups, err := repo.KeyGroup.ListByProvider(ctx, providerID)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups=%+v err=%v", groups, err)
	}
	gid := groups[0].ID
	// 模拟模型同步写入的白名单（上游模型名命名空间）
	if _, err := repo.KeyGroup.UpdateFields(ctx, gid, map[string]any{"models": "up-a,up-b"}); err != nil {
		t.Fatalf("seed whitelist: %v", err)
	}

	updated := updateProviderViaHandler(t, providerID, `{
		"name":"p-whitelist",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[{"id":`+strconv.FormatUint(uint64(gid), 10)+`,"name":"改名组","weight":3,"pool_id":`+strconv.FormatUint(uint64(poolB), 10)+`}]
	}`)
	if code := gjson.Get(updated, "code").Int(); code != 200 {
		t.Fatalf("update code=%d body=%s", code, updated)
	}

	after, err := repo.KeyGroup.ListByProvider(ctx, providerID)
	if err != nil || len(after) != 1 {
		t.Fatalf("groups after=%+v err=%v", after, err)
	}
	if after[0].ID != gid {
		t.Fatalf("group ID = %d, want %d（分组必须就地更新，不得删光重建）", after[0].ID, gid)
	}
	if after[0].Models != "up-a,up-b" {
		t.Fatalf("Models = %q, want %q（保存供应商不得清掉同步白名单）", after[0].Models, "up-a,up-b")
	}
	if after[0].Name != "改名组" || after[0].Weight != 3 {
		t.Fatalf("name/weight = %q/%d, want 改名组/3", after[0].Name, after[0].Weight)
	}
	if after[0].PoolID == nil || *after[0].PoolID != poolB {
		t.Fatalf("PoolID = %v, want %d", after[0].PoolID, poolB)
	}
}

// TestStructured_GroupSync_NewGroupWithoutID 锁「组 id=0/缺失 = 新建」：新建组白名单为空
// （= 未同步/不限），既有组按 id 就地更新——两种形态可混排在同一次保存里。
func TestStructured_GroupSync_NewGroupWithoutID(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolA := seedPool(t, "pool-mixed-a")
	poolB := seedPool(t, "pool-mixed-b")
	created := createProviderViaHandler(t, structuredCreateBody("p-mixed", poolA))
	providerID := uint(gjson.Get(created, "data.ID").Uint())
	ctx := t.Context()
	repo := repository.Default()
	groups, err := repo.KeyGroup.ListByProvider(ctx, providerID)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups=%+v err=%v", groups, err)
	}
	gid := groups[0].ID
	if _, err := repo.KeyGroup.UpdateFields(ctx, gid, map[string]any{"models": "up-a"}); err != nil {
		t.Fatalf("seed whitelist: %v", err)
	}

	updated := updateProviderViaHandler(t, providerID, `{
		"name":"p-mixed",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[
			{"id":`+strconv.FormatUint(uint64(gid), 10)+`,"name":"既有组","weight":1,"pool_id":`+strconv.FormatUint(uint64(poolB), 10)+`},
			{"name":"新组","weight":2,"pool_id":`+strconv.FormatUint(uint64(poolA), 10)+`}
		]
	}`)
	if code := gjson.Get(updated, "code").Int(); code != 200 {
		t.Fatalf("update code=%d body=%s", code, updated)
	}

	after, err := repo.KeyGroup.ListByProvider(ctx, providerID)
	if err != nil || len(after) != 2 {
		t.Fatalf("groups after=%+v err=%v", after, err)
	}
	byName := map[string]models.KeyGroup{}
	for _, g := range after {
		byName[g.Name] = g
	}
	old, ok := byName["既有组"]
	if !ok {
		t.Fatalf("既有组 missing: %+v", after)
	}
	if old.ID != gid || old.Models != "up-a" {
		t.Fatalf("既有组 ID/Models = %d/%q, want %d/up-a（就地更新且白名单不动）", old.ID, old.Models, gid)
	}
	if old.PoolID == nil || *old.PoolID != poolB {
		t.Fatalf("既有组 PoolID = %v, want %d", old.PoolID, poolB)
	}
	fresh, ok := byName["新组"]
	if !ok {
		t.Fatalf("新组 missing: %+v", after)
	}
	if fresh.ID == gid {
		t.Fatalf("新组 ID = %d, 不应复用既有组 ID", fresh.ID)
	}
	if fresh.Models != "" {
		t.Fatalf("新组 Models = %q, want 空（未同步 = 不限）", fresh.Models)
	}
	if fresh.PoolID == nil || *fresh.PoolID != poolA {
		t.Fatalf("新组 PoolID = %v, want %d", fresh.PoolID, poolA)
	}
}

// TestStructured_GroupSync_RejectsUnknownOrDuplicateGroupID 锁写入闸：组 id 引用
// 不存在（含跨供应商）或重复出现 → 400（errValidation），不静默当新建吞掉。
func TestStructured_GroupSync_RejectsUnknownOrDuplicateGroupID(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolID := seedPool(t, "pool-idcheck")
	poolStr := strconv.FormatUint(uint64(poolID), 10)
	created := createProviderViaHandler(t, structuredCreateBody("p-idcheck", poolID))
	providerID := uint(gjson.Get(created, "data.ID").Uint())
	ctx := t.Context()
	groups, err := repository.Default().KeyGroup.ListByProvider(ctx, providerID)
	if err != nil || len(groups) != 1 {
		t.Fatalf("groups=%+v err=%v", groups, err)
	}
	gid := groups[0].ID
	gidStr := strconv.FormatUint(uint64(gid), 10)

	for name, groupJSON := range map[string]string{
		"unknown id":   `{"id":999999,"name":"g","weight":1,"pool_id":` + poolStr + `}`,
		"duplicate id": `{"id":` + gidStr + `,"name":"g1","weight":1,"pool_id":` + poolStr + `},{"id":` + gidStr + `,"name":"g2","weight":1,"pool_id":` + poolStr + `}`,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{{Key: "id", Value: gidStr}}
			body := `{
				"name":"p-idcheck","type":"openai",
				"config":"{\"base_url\":\"https://api.example.com\"}",
				"console":"","proxy":"",
				"protocols":["openai"],
				"endpoints":[{"protocol":"openai","url":"","enabled":true}],
				"groups":[` + groupJSON + `]
			}`
			c.Request = httptest.NewRequest("PUT", "/providers/"+gidStr, strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			UpdateProvider(c)
			// httpresp 约定：错误也走 HTTP 200，业务码在响应体 code 字段
			if code := gjson.Get(w.Body.String(), "code").Int(); code != 400 {
				t.Fatalf("code = %d, want 400, body=%s", code, w.Body.String())
			}
		})
	}
}

// TestStructured_GroupSync_UpdatesPoolAndKeepsGroupID 锁一次保存里多个分组的就地更新：
// 两组各带自己的 id 提交，name 与 pool_id 都要落到同一行上（ID 不漂移）——这是前端
// 弹窗按组 id 归位的依赖。同时守住 map 更新的显式语义：weight=0 也得真写成 0。
func TestStructured_GroupSync_UpdatesPoolAndKeepsGroupID(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	poolA := seedPool(t, "pool-multi-a")
	poolB := seedPool(t, "pool-multi-b")
	poolC := seedPool(t, "pool-multi-c")
	aStr := strconv.FormatUint(uint64(poolA), 10)

	created := createProviderViaHandler(t, `{
		"name":"p-multi",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[
			{"name":"组一","weight":1,"pool_id":`+aStr+`},
			{"name":"组二","weight":1,"pool_id":`+aStr+`}
		]
	}`)
	providerID := uint(gjson.Get(created, "data.ID").Uint())
	if providerID == 0 {
		t.Fatalf("missing ID: %s", created)
	}

	ctx := t.Context()
	repo := repository.Default()
	groups, err := repo.KeyGroup.ListByProvider(ctx, providerID)
	if err != nil || len(groups) != 2 {
		t.Fatalf("groups=%+v err=%v", groups, err)
	}
	orig := map[string]uint{}
	for _, g := range groups {
		orig[g.Name] = g.ID
	}

	updated := updateProviderViaHandler(t, providerID, `{
		"name":"p-multi",
		"type":"openai",
		"config":"{\"base_url\":\"https://api.example.com\"}",
		"console":"","proxy":"",
		"protocols":["openai"],
		"endpoints":[{"protocol":"openai","url":"","enabled":true}],
		"groups":[
			{"id":`+strconv.FormatUint(uint64(orig["组一"]), 10)+`,"name":"改名一","weight":0,"pool_id":`+strconv.FormatUint(uint64(poolB), 10)+`},
			{"id":`+strconv.FormatUint(uint64(orig["组二"]), 10)+`,"name":"改名二","weight":5,"pool_id":`+strconv.FormatUint(uint64(poolC), 10)+`}
		]
	}`)
	if code := gjson.Get(updated, "code").Int(); code != 200 {
		t.Fatalf("update code=%d body=%s", code, updated)
	}

	after, err := repo.KeyGroup.ListByProvider(ctx, providerID)
	if err != nil || len(after) != 2 {
		t.Fatalf("groups after=%+v err=%v", after, err)
	}
	want := map[string]struct {
		id     uint
		weight int
		pool   uint
	}{
		"改名一": {id: orig["组一"], weight: 0, pool: poolB},
		"改名二": {id: orig["组二"], weight: 5, pool: poolC},
	}
	seen := 0
	for _, g := range after {
		w, ok := want[g.Name]
		if !ok {
			t.Fatalf("unexpected group %q: %+v", g.Name, g)
		}
		seen++
		if g.ID != w.id {
			t.Fatalf("组 %q ID = %d, want %d（就地更新不得换 id）", g.Name, g.ID, w.id)
		}
		if g.Weight != w.weight {
			t.Fatalf("组 %q weight = %d, want %d", g.Name, g.Weight, w.weight)
		}
		if g.PoolID == nil || *g.PoolID != w.pool {
			t.Fatalf("组 %q PoolID = %v, want %d", g.Name, g.PoolID, w.pool)
		}
	}
	if seen != 2 {
		t.Fatalf("匹配到 %d 个组, want 2", seen)
	}
}
