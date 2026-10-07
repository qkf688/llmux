package credentials

import (
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

const testCredHexKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// setupCipherForCredTest 注入测试用 cipher 并在用例结束复位默认值。
// 掩码/解密类断言必须打在真实密文上——cipher 未配置时 MaskStored 退化为 `****`，
// 测出的形状与生产不一致。
func setupCipherForCredTest(t *testing.T) {
	t.Helper()
	c, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
}

// invoke 直接调用 handler（不经 gin 路由树）：params 为路径参数、body 为空串时不带请求体。
// 各端点只差 (method, target, params, fn)，用一个调用器比八个近同形 helper 更少漂移。
func invoke(t *testing.T, method, target string, params gin.Params, body string, fn func(*gin.Context)) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = params
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, target, rdr)
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	fn(c)
	return w.Body.String(), w.Code
}

// credentialParams 构造 :id 路径参数。
func credentialParams(id uint) gin.Params {
	return gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
}

// seedPool 直接写库建号池（凭据域测试不依赖号池 handler）。
func seedPool(t *testing.T, name string) uint {
	t.Helper()
	pool := &models.Pool{Name: name}
	if err := repository.Default().Pool.Create(t.Context(), pool); err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	return pool.ID
}

// seedCredential 直接写库建归属 poolID 号池的凭据，返回 ID。
// plain 传明文、落密文（与生产写路径同形态）；status/failCount/cooldownReason 用于
// 构造被判停的凭据（本域最关键的场景：状态机判停后的人工恢复）。
func seedCredential(t *testing.T, poolID uint, plain, status string, failCount int, cooldownReason string) uint {
	t.Helper()
	if credentialcrypto.Default() == nil {
		setupCipherForCredTest(t)
	}
	enc, err := credentialcrypto.Default().Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt seed credential: %v", err)
	}
	cred := &models.Credential{
		PoolID:         poolID,
		Key:            enc,
		KeyHash:        credentialcrypto.Default().Hash(plain),
		Status:         status,
		FailCount:      failCount,
		CooldownReason: cooldownReason,
	}
	if err := repository.Default().Credential.Create(t.Context(), cred); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
	return cred.ID
}

// seedPoolMember 在既有号池下挂一条凭据，返回凭据 ID。
func seedPoolMember(t *testing.T, poolID uint, plain, status string) uint {
	t.Helper()
	return seedCredential(t, poolID, plain, status, 0, "")
}

// seedPoolCredential 建池并挂一条凭据，返回 (poolID, credentialID)。
func seedPoolCredential(t *testing.T, poolName, plain, status string) (uint, uint) {
	t.Helper()
	poolID := seedPool(t, poolName)
	return poolID, seedPoolMember(t, poolID, plain, status)
}

// reloadCredential 读回凭据实体（断言落库字段）。
func reloadCredential(t *testing.T, id uint) models.Credential {
	t.Helper()
	cred, err := repository.Default().Credential.Get(t.Context(), id)
	if err != nil {
		t.Fatalf("reload credential %d: %v", id, err)
	}
	return *cred
}

// markCredentialCooling 给既有凭据写冷却截止时间（status 保持 active，贴合
// 「冷却不占状态位」的状态机）。走 UpdateFields 而非 struct Update——三态指针字段
// 清/写都得用 map 显式给值，与生产写路径同款（见 pools.md 三态字段约定）。
func markCredentialCooling(t *testing.T, credID uint, until time.Time) {
	t.Helper()
	if _, err := repository.Default().Credential.UpdateFields(t.Context(), credID, map[string]any{
		"cooldown_until":  until,
		"cooldown_reason": "http_429",
	}); err != nil {
		t.Fatalf("mark credential %d cooling: %v", credID, err)
	}
}

// initCredentialsTestDB 凭据域测试统一前置：独立 DB + cipher。
func initCredentialsTestDB(t *testing.T) {
	t.Helper()
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
}

// serveJSON 经真实 gin 引擎发一次请求（用于路由注册顺序类断言——只有走路由树才测得出
// 静态段与 :id 的匹配优先级）。
func serveJSON(t *testing.T, r *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// itoa 测试用：uint → 十进制字符串（拼路径与查询串）。
func itoa(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

// dataID 从响应体取 data.ID（0 = 缺失）。
func dataID(t *testing.T, body string) uint {
	t.Helper()
	id := gjson.Get(body, "data.ID").Uint()
	if id == 0 {
		t.Fatalf("响应缺 data.ID body=%s", body)
	}
	return uint(id)
}
