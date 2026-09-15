package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

// 契约测试打在 #4-2 的鉴权切换上（.plan/plan.md AC-1~AC-5）：
// seam = AuthAPIKey(apiKeyRepo, userRepo) 的 HTTP 外部行为 + CallerKey context 快照。
// 全程不调 credentialcrypto.SetDefault——Key 列填占位密文，若实现尝试解密必挂，
// 这本身就是「鉴权不依赖加密装配态」（AC-5）的可执行证据。

func newAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// sqlite 内存库多连接下表不可见，强制单连接。
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("raw db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&models.User{}, &models.ApiKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// newAuthTestEnv 建 admin 用户 + 挂 AuthAPIKey 保护的 /v1/ping（回显归因快照）。
func newAuthTestEnv(t *testing.T) (*gin.Engine, *repository.Repositories) {
	t.Helper()
	repos := repository.New(newAuthTestDB(t))
	if err := repos.User.Create(context.Background(), &models.User{
		Username:     "admin",
		PasswordHash: "not-used-here",
		Role:         "admin",
	}); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/v1/ping", AuthAPIKey(repos.ApiKey, repos.User), func(c *gin.Context) {
		user := CurrentUser(c)
		caller := CurrentCallerKey(c)
		c.JSON(http.StatusOK, gin.H{
			"user_id":  user.ID,
			"key_id":   caller.ID,
			"key_name": caller.Name,
		})
	})
	return engine, repos
}

func seedCallerKey(t *testing.T, repos *repository.Repositories, ownerID uint, name, plain string, enabled bool) *models.ApiKey {
	t.Helper()
	row := &models.ApiKey{
		OwnerID: ownerID,
		Name:    name,
		// 占位密文：鉴权按 hash 点查、永不解密（AC-5），此处故意不是可解密形态。
		Key:     "placeholder-not-decryptable",
		KeyHash: credentialcrypto.HashKey(plain),
		Enabled: enabled,
	}
	if err := repos.ApiKey.Create(context.Background(), row); err != nil {
		t.Fatalf("seed api key: %v", err)
	}
	return row
}

func adminUserID(t *testing.T, repos *repository.Repositories) uint {
	t.Helper()
	user, err := repos.User.FindByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("find admin: %v", err)
	}
	return user.ID
}

func doPing(t *testing.T, engine *gin.Engine, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func mustAdmin(t *testing.T, repos *repository.Repositories) uint {
	t.Helper()
	id := adminUserID(t, repos)
	if credentialcrypto.Default() != nil {
		t.Fatal("测试前提被破坏：credentialcrypto.Default() 应为未装配态（AC-5 的钉）")
	}
	return id
}

// AC-1：hash 命中且 enabled 的 key 放行，并按 OwnerID 注入 owner（AC-3 per-user 语义）。
func TestAuthAPIKey_ValidBearerPassesWithOwnerAttribution(t *testing.T) {
	engine, repos := newAuthTestEnv(t)
	ownerID := mustAdmin(t, repos)
	plain := "sk-llmux-valid-key-0001"
	row := seedCallerKey(t, repos, ownerID, "codex", plain, true)

	rec := doPing(t, engine, map[string]string{"Authorization": "Bearer " + plain})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if got := gjson.Get(body, "user_id").Uint(); got != uint64(ownerID) {
		t.Fatalf("user_id = %d, want owner %d", got, ownerID)
	}
	if got := gjson.Get(body, "key_id").Uint(); got != uint64(row.ID) {
		t.Fatalf("key_id = %d, want %d", got, row.ID)
	}
	if got := gjson.Get(body, "key_name").String(); got != "codex" {
		t.Fatalf("key_name = %q, want %q", got, "codex")
	}
}

// AC-1：无命中行 → 真实 401 + 信封 code=401 + Invalid API key。
func TestAuthAPIKey_UnknownKeyRejected(t *testing.T) {
	engine, repos := newAuthTestEnv(t)
	seedCallerKey(t, repos, mustAdmin(t, repos), "other", "sk-llmux-other-key-0001", true)

	rec := doPing(t, engine, map[string]string{"Authorization": "Bearer sk-llmux-not-in-db-key"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}
	if got := gjson.Get(rec.Body.String(), "code").Int(); got != 401 {
		t.Fatalf("envelope code = %d, want 401", got)
	}
	if got := gjson.Get(rec.Body.String(), "message").String(); got != "Invalid API key" {
		t.Fatalf("message = %q, want %q", got, "Invalid API key")
	}
}

// AC-2：命中但 enabled=false → 401，文案与无效 key 可区分。
func TestAuthAPIKey_DisabledKeyRejected(t *testing.T) {
	engine, repos := newAuthTestEnv(t)
	plain := "sk-llmux-disabled-key-001"
	seedCallerKey(t, repos, mustAdmin(t, repos), "off", plain, false)

	rec := doPing(t, engine, map[string]string{"Authorization": "Bearer " + plain})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := gjson.Get(rec.Body.String(), "message").String(); got != "API key is disabled" {
		t.Fatalf("message = %q, want %q", got, "API key is disabled")
	}
}

// AC-4：Anthropic 形态 x-api-key 单独可用。
func TestAuthAPIKey_XAPIKeyFallbackPasses(t *testing.T) {
	engine, repos := newAuthTestEnv(t)
	plain := "sk-llmux-anthropic-key-1"
	row := seedCallerKey(t, repos, mustAdmin(t, repos), "claude", plain, true)

	rec := doPing(t, engine, map[string]string{"x-api-key": plain})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if got := gjson.Get(rec.Body.String(), "key_id").Uint(); got != uint64(row.ID) {
		t.Fatalf("key_id = %d, want %d", got, row.ID)
	}
}

// AC-4：两 header 并存且各自合法时 Bearer 优先（归因必须是 Bearer 那把）。
func TestAuthAPIKey_BearerTakesPrecedenceOverXAPIKey(t *testing.T) {
	engine, repos := newAuthTestEnv(t)
	ownerID := mustAdmin(t, repos)
	bearerPlain := "sk-llmux-bearer-key-0001"
	xPlain := "sk-llmux-xapi-key-00001"
	bearerRow := seedCallerKey(t, repos, ownerID, "bearer", bearerPlain, true)
	seedCallerKey(t, repos, ownerID, "xapikey", xPlain, true)

	rec := doPing(t, engine, map[string]string{
		"Authorization": "Bearer " + bearerPlain,
		"x-api-key":     xPlain,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
	}
	if got := gjson.Get(rec.Body.String(), "key_id").Uint(); got != uint64(bearerRow.ID) {
		t.Fatalf("key_id = %d, want bearer key %d (Bearer 优先)", got, bearerRow.ID)
	}
}

// 缺 header → 401（沿用现状文案，客户端排障指向「没带头」而非「key 错」）。
func TestAuthAPIKey_MissingHeaderRejected(t *testing.T) {
	engine, repos := newAuthTestEnv(t)
	mustAdmin(t, repos)

	rec := doPing(t, engine, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := gjson.Get(rec.Body.String(), "message").String(); got != "Authorization header or x-api-key header is missing" {
		t.Fatalf("message = %q", got)
	}
}

// R5：`Bearer `（空值）在 hash 之前即拒——防空 hash 点查命中任何异常行。
func TestAuthAPIKey_EmptyBearerValueRejected(t *testing.T) {
	engine, repos := newAuthTestEnv(t)
	mustAdmin(t, repos)

	rec := doPing(t, engine, map[string]string{"Authorization": "Bearer "})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// AC-3 / R2：命中 key 但 owner 用户不存在 → 401（身份无法建立），不是 500。
func TestAuthAPIKey_OwnerMissingRejected(t *testing.T) {
	engine, repos := newAuthTestEnv(t)
	plain := "sk-llmux-orphan-key-0001"
	seedCallerKey(t, repos, 999, "orphan", plain, true)

	rec := doPing(t, engine, map[string]string{"Authorization": "Bearer " + plain})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := gjson.Get(rec.Body.String(), "message").String(); got != "api key owner not found" {
		t.Fatalf("message = %q, want %q", got, "api key owner not found")
	}
}

// authStubRepo 只覆盖 FindByHash 注入 DB 故障（系统边界桩），其余委托真 repo。
type authStubRepo struct {
	repository.ApiKeyRepo
	err error
}

func (s authStubRepo) FindByHash(_ context.Context, _ string) (*models.ApiKey, error) {
	return nil, s.err
}

// R2：非 not-found 的仓储错误 → 500（不把基础设施故障伪装成鉴权失败）。
func TestAuthAPIKey_RepoErrorReturns500(t *testing.T) {
	_, repos := newAuthTestEnv(t)
	stub := authStubRepo{ApiKeyRepo: repos.ApiKey, err: errors.New("database is locked")}
	faulty := gin.New()
	faulty.GET("/v1/ping", AuthAPIKey(stub, repos.User), func(c *gin.Context) {
		t.Fatal("鉴权失败不得进入下游 handler")
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
	req.Header.Set("Authorization", "Bearer sk-llmux-whatever-key-1")
	rec := httptest.NewRecorder()
	faulty.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body = %s", rec.Code, rec.Body.String())
	}
	// 500 不回显驱动错误串（未认证调用方拿不到内部细节）
	if body := rec.Body.String(); strings.Contains(body, "database is locked") {
		t.Fatalf("响应体泄露内部错误: %s", body)
	}
}

// ownerStubRepo 只在 GetByID 注入 DB 故障（key 已命中并过 enabled 门之后的分支）。
type ownerStubRepo struct {
	repository.UserRepo
}

func (ownerStubRepo) GetByID(_ context.Context, _ uint) (*models.User, error) {
	return nil, errors.New("disk I/O error")
}

// R2：owner 点查非 not-found 故障 → 500（与 FindByHash 故障分支同权对待）。
func TestAuthAPIKey_OwnerRepoErrorReturns500(t *testing.T) {
	_, repos := newAuthTestEnv(t)
	ownerID := mustAdmin(t, repos)
	plain := "sk-llmux-owner-fault-key-1"
	seedCallerKey(t, repos, ownerID, "ok-key", plain, true)
	gin.SetMode(gin.TestMode)
	faulty := gin.New()
	faulty.GET("/v1/ping", AuthAPIKey(repos.ApiKey, ownerStubRepo{UserRepo: repos.User}), func(c *gin.Context) {
		t.Fatal("owner 解析失败不得进入下游 handler")
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
	req.Header.Set("Authorization", "Bearer "+plain)
	rec := httptest.NewRecorder()
	faulty.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500, body = %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); strings.Contains(body, "disk I/O error") {
		t.Fatalf("响应体泄露内部错误: %s", body)
	}
}

// 门的顺序锁：enabled 门必须先于 owner 点查——disabled 且 owner 悬空时文案仍是
// disabled（若日后有人把 GetByID 挪到 enabled 门前，本用例必红）。
func TestAuthAPIKey_DisabledCheckedBeforeOwnerLookup(t *testing.T) {
	engine, repos := newAuthTestEnv(t)
	plain := "sk-llmux-disabled-orphan-key"
	seedCallerKey(t, repos, 999, "off-orphan", plain, false)

	rec := doPing(t, engine, map[string]string{"Authorization": "Bearer " + plain})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := gjson.Get(rec.Body.String(), "message").String(); got != "API key is disabled" {
		t.Fatalf("message = %q, want %q（enabled 门必须先于 owner 解析）", got, "API key is disabled")
	}
}
