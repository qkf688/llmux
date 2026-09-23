package providerapi

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// 本文件锁认证方式真值单一来源（Stage 20 派生化）：运行时真值永远在 config
// （anthropic 读 auth_type、openai 系读 openai_auth_type），顶层 AuthType 是**派生
// 展示值、不落库**。此前顶层列双写且无一致性约束，直接 PUT 可造出
// 「列显 bearer、实际发 x-api-key」的漂移——本文件即该漂移的回归钉。

// authDeriveBody 构造指定 type/config 的全量结构化写入体。config 经 json.Marshal +
// strconv.Quote 两层正规转义进外层 JSON，不手写反斜杠（嵌套 JSON 手写转义必错）。
func authDeriveBody(name, providerType string, config map[string]any) string {
	cfg, err := json.Marshal(config)
	if err != nil {
		panic(err)
	}
	return `{
		"name":"` + name + `",
		"type":"` + providerType + `",
		"config":` + strconv.Quote(string(cfg)) + `,
		"console":"",
		"proxy":"",
		"protocols":["` + providerType + `"],
		"endpoints":[{"protocol":"` + providerType + `","url":"","enabled":true}],
		"groups":[{"name":"默认组","weight":1,"models":"","source":"inline","inline_keys":[{"id":0,"key":"sk-test-ABCD1234"}]}]
	}`
}

// createAndGetAuthType 建供应商后 GET 详情，返回详情响应体与新建 ID。
func createAndGetAuthType(t *testing.T, name, providerType string, config map[string]any) (string, uint) {
	t.Helper()
	createBody := createProviderViaHandler(t, authDeriveBody(name, providerType, config))
	id := uint(gjson.Get(createBody, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("create missing ID: %s", createBody)
	}
	return getProviderViaHandler(t, id), id
}

// TestProviderAuthType_DerivedRejectsDrift 钉来源文档的可验姿势（AC-4）：
// PUT 传漂移顶层 auth_type:"bearer" 而 config 内是 x-api-key，GET 顶层必须是
// config 真值——此前顶层列被写成 bearer，正是双写漂移本体（改前实测红）。
func TestProviderAuthType_DerivedRejectsDrift(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	cfg := map[string]any{"base_url": "https://api.example.com", "auth_type": "x-api-key"}
	body, id := createAndGetAuthType(t, "p-auth-drift", "anthropic", cfg)
	if got := gjson.Get(body, "data.AuthType").String(); got != "x-api-key" {
		t.Fatalf("创建后 AuthType = %q, want %q（派生自 config.auth_type）。body=%s", got, "x-api-key", body)
	}

	driftBody := strings.Replace(authDeriveBody("p-auth-drift", "anthropic", cfg),
		`"proxy":"",`, `"proxy":"","auth_type":"bearer",`, 1)
	updateProviderViaHandler(t, id, driftBody)

	got := getProviderViaHandler(t, id)
	if v := gjson.Get(got, "data.AuthType").String(); v != "x-api-key" {
		t.Fatalf("PUT 漂移后 AuthType = %q, want %q（顶层真值单源在 config，漂移输入不得生效）。body=%s", v, "x-api-key", got)
	}
}

// TestProviderAuthType_DerivedForOpenAIDialect 钉 AC-5：openai 系顶层 AuthType 派生自
// config.openai_auth_type（此前该列恒 null，真值只藏在 config 里）。
func TestProviderAuthType_DerivedForOpenAIDialect(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	body, _ := createAndGetAuthType(t, "p-auth-openai", "openai",
		map[string]any{"base_url": "https://api.example.com", "openai_auth_type": "x-api-key"})
	if got := gjson.Get(body, "data.AuthType").String(); got != "x-api-key" {
		t.Fatalf("openai 系 AuthType = %q, want %q（派生自 config.openai_auth_type）。body=%s", got, "x-api-key", body)
	}
}

// TestProviderAuthType_NullWhenUnset 钉三态形状：config 无对应键时顶层键存在且为
// null（不是缺键、也不是回填类型默认——「未设置」与「显式设置」必须可区分）。
func TestProviderAuthType_NullWhenUnset(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	body, _ := createAndGetAuthType(t, "p-auth-unset", "anthropic",
		map[string]any{"base_url": "https://api.example.com"})
	got := gjson.Get(body, "data.AuthType")
	if !got.Exists() {
		t.Fatalf("AuthType 键缺失，want 存在且为 null（三态字段禁 omitempty）。body=%s", body)
	}
	if got.Type != gjson.Null {
		t.Fatalf("AuthType = %s, want null（config 无键 = 未设置，不得回填默认值）。body=%s", got.Raw, body)
	}
}

// TestProviderAuthType_NotPersisted 钉 AC-6 的持久化半边（审查回批后判据收紧）：
// 双写面物理消失靠两条判据——① schema 无 auth_type 列（gorm:"-" + 启动迁移
// dropLegacyProviderAuthType；若有人恢复落列，AutoMigrate 建列后此断言红）；
// ② Create/Update 全流程 200——无列库上任何残留写点都会 SQL 报错（no such column
// 使 Update 500），比断 ORM 读回字段值硬（gorm:"-" 字段本就不会被扫描，恒 nil）。
func TestProviderAuthType_NotPersisted(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	cfg := map[string]any{"base_url": "https://api.example.com", "auth_type": "x-api-key"}
	_, id := createAndGetAuthType(t, "p-auth-nopersist", "anthropic", cfg)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
	c.Request = httptest.NewRequest("PUT", "/providers/"+strconv.FormatUint(uint64(id), 10),
		strings.NewReader(authDeriveBody("p-auth-nopersist", "anthropic", cfg)))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateProvider(c)
	if w.Code != 200 {
		t.Fatalf("UpdateProvider status=%d body=%s（无列库上残留 auth_type 写点会在此 SQL 报错）", w.Code, w.Body.String())
	}

	// 判据用表名字符串：gorm:"-" 字段不在 schema，HasColumn(&models.Provider{}) 恒 false。
	if models.DB.Migrator().HasColumn("providers", "auth_type") {
		t.Fatal("providers 表仍有 auth_type 列：派生字段不落库，双写面应随迁移物理消失（gorm:\"-\" + dropLegacyProviderAuthType）")
	}
}
