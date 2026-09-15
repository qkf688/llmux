package apikeys

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/middleware"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

// —— 测试夹具 ——————————————————————————————

func setTestCipher(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	hexKey := hex.EncodeToString([]byte("0123456789012345678901234567890a"))
	cipher, err := credentialcrypto.New(hexKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	prev := credentialcrypto.Default()
	credentialcrypto.SetDefault(cipher)
	t.Cleanup(func() { credentialcrypto.SetDefault(prev) })
	return cipher
}

// newApiKeyTestRouter 挂真实路由 + 注入登录用户（AuthJWT 属中间件层职责，
// 本契约测试锁 apikeys 域自身的编排与响应形状）。
func newApiKeyTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	testsupport.InitTestDB(t)
	engine := gin.New()
	api := engine.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUser, &models.User{Model: gorm.Model{ID: 1}, Username: "admin"})
	})
	Register(api)
	return engine
}

func doJSON(t *testing.T, engine *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// —— AC-1 创建：密文+hash 入库、明文回显、no-store ————————

func TestCreateApiKey_StoresCipherEchoesPlaintextNoStore(t *testing.T) {
	cipher := setTestCipher(t)
	engine := newApiKeyTestRouter(t)

	rec := doJSON(t, engine, http.MethodPost, "/api/api-keys", `{"name":"codex 专用","value":"my-custom-key-value"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store（受控明文禁缓存）", got)
	}
	body := rec.Body.String()
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("信封 code != 200: %s", body)
	}
	if got := gjson.Get(body, "data.api_key").String(); got != "sk-llmux-my-custom-key-value" {
		t.Fatalf("回显明文 = %q（应补前缀原样明文）", got)
	}
	if !gjson.Get(body, "data.id").Exists() || gjson.Get(body, "data.id").Uint() == 0 {
		t.Fatalf("data.id 缺失: %s", body)
	}
	if gjson.Get(body, "data.name").String() != "codex 专用" || !gjson.Get(body, "data.enabled").Bool() {
		t.Fatalf("data.name/enabled 不符: %s", body)
	}
	if !gjson.Get(body, "data.created_at").Exists() {
		t.Fatalf("data.created_at 缺失: %s", body)
	}

	// 存储层形状：库内是密文 + 正确 hash，无明文
	var stored models.ApiKey
	if err := models.DB.First(&stored).Error; err != nil {
		t.Fatalf("读回存储行: %v", err)
	}
	if stored.Key == "sk-llmux-my-custom-key-value" {
		t.Fatal("库里存了明文")
	}
	if plain, err := cipher.Decrypt(stored.Key); err != nil || plain != "sk-llmux-my-custom-key-value" {
		t.Fatalf("密文不可还原: %v %q", err, plain)
	}
	if stored.KeyHash != cipher.Hash("sk-llmux-my-custom-key-value") {
		t.Fatal("KeyHash 非 SHA-256(明文)")
	}
	if stored.OwnerID != 1 {
		t.Fatalf("OwnerID = %d, want 1（per-user 归属取登录用户）", stored.OwnerID)
	}
}

// —— AC-2 校验与自动生成 ————————————————————————

func TestCreateApiKey_RejectsTooShortAfterPrefix(t *testing.T) {
	setTestCipher(t)
	engine := newApiKeyTestRouter(t)

	rec := doJSON(t, engine, http.MethodPost, "/api/api-keys", `{"name":"短","value":"ab"}`)
	body := rec.Body.String()
	if gjson.Get(body, "code").Int() != http.StatusBadRequest {
		t.Fatalf("code = %s, want 400（补前缀后仍 <16 拒绝）: %s", gjson.Get(body, "code"), body)
	}
	if gjson.Get(body, "data").Exists() {
		t.Fatalf("拒绝时不应回 data: %s", body)
	}
}

func TestCreateApiKey_GeneratesWhenValueOmitted(t *testing.T) {
	engine := newApiKeyTestRouter(t)
	setTestCipher(t)

	rec := doJSON(t, engine, http.MethodPost, "/api/api-keys", `{"name":"随机"}`)
	body := rec.Body.String()
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("省略 value 应自动生成: %s", body)
	}
	key := gjson.Get(body, "data.api_key").String()
	if !strings.HasPrefix(key, "sk-llmux-") || len(key) < 16 {
		t.Fatalf("生成值不合规: %q", key)
	}
}

// —— AC-3 撞库 409（真实 HTTP 状态码） ————————————————

func TestCreateApiKey_DuplicateValueReturnsHTTP409(t *testing.T) {
	setTestCipher(t)
	engine := newApiKeyTestRouter(t)

	first := doJSON(t, engine, http.MethodPost, "/api/api-keys", `{"name":"a","value":"sk-llmux-collision-value-001"}`)
	if first.Code != http.StatusOK {
		t.Fatalf("首次创建失败: %s", first.Body.String())
	}
	second := doJSON(t, engine, http.MethodPost, "/api/api-keys", `{"name":"b","value":"sk-llmux-collision-value-001"}`)
	if second.Code != http.StatusConflict {
		t.Fatalf("撞库应真实 HTTP 409，实得 %d: %s", second.Code, second.Body.String())
	}
	if code := gjson.Get(second.Body.String(), "code").Int(); code != http.StatusConflict {
		t.Fatalf("信封 code = %d, want 409", code)
	}

	// name 重名不受影响（方案 §5：name 不唯一）
	third := doJSON(t, engine, http.MethodPost, "/api/api-keys", `{"name":"a","value":"sk-llmux-collision-value-002"}`)
	if third.Code != http.StatusOK {
		t.Fatalf("同名不同值应成功，实得 %d: %s", third.Code, third.Body.String())
	}
}

// —— AC-4 列表明文回显 + 解密失败降级 ——————————————————

func TestListApiKeys_PlaintextWithMaskFallback(t *testing.T) {
	setTestCipher(t)
	engine := newApiKeyTestRouter(t)

	doJSON(t, engine, http.MethodPost, "/api/api-keys", `{"name":"good","value":"sk-llmux-list-good-value-1"}`)
	doJSON(t, engine, http.MethodPost, "/api/api-keys", `{"name":"bad","value":"sk-llmux-list-bad-value-22"}`)
	// 篡改第二行密文，模拟密钥轮换后不可解（降级路径）
	var bad models.ApiKey
	if err := models.DB.Where("name = ?", "bad").First(&bad).Error; err != nil {
		t.Fatalf("取 bad 行: %v", err)
	}
	if err := models.DB.Model(&bad).Update("api_key", "deadbeef").Error; err != nil {
		t.Fatalf("篡改密文: %v", err)
	}

	rec := doJSON(t, engine, http.MethodGet, "/api/api-keys", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("列表也必须 no-store，实得 %q", got)
	}
	items := gjson.Get(rec.Body.String(), "data").Array()
	if len(items) != 2 {
		t.Fatalf("列表长度 = %d, want 2: %s", len(items), rec.Body.String())
	}
	for _, item := range items {
		for _, key := range []string{"id", "name", "api_key", "enabled", "created_at"} {
			if !item.Get(key).Exists() {
				t.Fatalf("列表项缺键 %q: %s", key, item.Raw)
			}
		}
	}
	if items[0].Get("api_key").String() != "sk-llmux-list-good-value-1" {
		t.Fatalf("正常行应回明文: %s", items[0].Raw)
	}
	if items[1].Get("api_key").String() != "****" {
		t.Fatalf("解密失败行应降级 ****（同 MaskStored 口径）: %s", items[1].Raw)
	}
}

// —— AC-5 改名/启停/删除 ——————————————————————————

func createKeyViaHandler(t *testing.T, engine *gin.Engine, name, value string) uint {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"name": name, "value": value})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rec := doJSON(t, engine, http.MethodPost, "/api/api-keys", string(payload))
	if rec.Code != http.StatusOK {
		t.Fatalf("创建失败: %d %s", rec.Code, rec.Body.String())
	}
	return uint(gjson.Get(rec.Body.String(), "data.id").Uint())
}

func TestUpdateApiKey_RenameAndToggle(t *testing.T) {
	setTestCipher(t)
	engine := newApiKeyTestRouter(t)

	id := createKeyViaHandler(t, engine, "旧名", "sk-llmux-update-target-001")
	path := "/api/api-keys/" + strconv.FormatUint(uint64(id), 10)
	rec := doJSON(t, engine, http.MethodPatch, path, `{"name":"新名","enabled":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH HTTP %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("PATCH 回显含明文，也必须 no-store，实得 %q", got)
	}
	body := rec.Body.String()
	if gjson.Get(body, "data.name").String() != "新名" || gjson.Get(body, "data.enabled").Bool() {
		t.Fatalf("更新回显不符: %s", body)
	}
	// 密文不因改名/启停变化（值不变）
	if gjson.Get(body, "data.api_key").String() != "sk-llmux-update-target-001" {
		t.Fatalf("值应保持不变: %s", body)
	}

	empty := doJSON(t, engine, http.MethodPatch, path, `{}`)
	if gjson.Get(empty.Body.String(), "code").Int() != http.StatusBadRequest {
		t.Fatalf("空更新体应 400 语义: %s", empty.Body.String())
	}

	missing := doJSON(t, engine, http.MethodPatch, "/api/api-keys/9999", `{"name":"x-name-long-enough"}`)
	if gjson.Get(missing.Body.String(), "code").Int() != http.StatusNotFound {
		t.Fatalf("不存在 id 应 404 语义: %s", missing.Body.String())
	}
}

func TestDeleteApiKey_Hard(t *testing.T) {
	setTestCipher(t)
	engine := newApiKeyTestRouter(t)

	id := createKeyViaHandler(t, engine, "将删", "sk-llmux-delete-target-01")
	path := "/api/api-keys/" + strconv.FormatUint(uint64(id), 10)
	rec := doJSON(t, engine, http.MethodDelete, path, "")
	if rec.Code != http.StatusOK || gjson.Get(rec.Body.String(), "code").Int() != 200 {
		t.Fatalf("DELETE HTTP %d: %s", rec.Code, rec.Body.String())
	}
	again := doJSON(t, engine, http.MethodDelete, path, "")
	if gjson.Get(again.Body.String(), "code").Int() != http.StatusNotFound {
		t.Fatalf("重复删除应 404 语义: %s", again.Body.String())
	}
	// 硬删：同值可重建（唯一索引不被软删行挡死）
	rebuilt := doJSON(t, engine, http.MethodPost, "/api/api-keys", `{"name":"重建","value":"sk-llmux-delete-target-01"}`)
	if rebuilt.Code != http.StatusOK {
		t.Fatalf("删后同值重建应成功: %d %s", rebuilt.Code, rebuilt.Body.String())
	}
}
