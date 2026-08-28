package pools

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// createPoolViaHandler 走 CreatePool handler 并返回响应体。
func createPoolViaHandler(t *testing.T, body string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/pools", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	CreatePool(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// updatePoolViaHandler 走 UpdatePool handler 并返回响应体。
func updatePoolViaHandler(t *testing.T, id uint, body string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
	c.Request = httptest.NewRequest("PUT", "/pools/"+strconv.FormatUint(uint64(id), 10), strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdatePool(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// listPoolsViaHandler 走 GetPools handler 并返回响应体。
func listPoolsViaHandler(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/pools", nil)

	GetPools(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// deletePoolViaHandler 走 DeletePool handler 并返回响应体与状态码
// （删除路径有 BadRequest 分支，需断言非 200 响应）。
func deletePoolViaHandler(t *testing.T, id uint) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
	c.Request = httptest.NewRequest("DELETE", "/pools/"+strconv.FormatUint(uint64(id), 10), nil)

	DeletePool(c)

	return w.Body.String(), w.Code
}

// seedPoolWithCredential 建号池并挂一条指定状态凭据，返回号池 ID。
func seedPoolWithCredential(t *testing.T, name string, status string) uint {
	t.Helper()
	body := createPoolViaHandler(t, `{"name":"`+name+`"}`)
	id := gjson.Get(body, "data.ID").Uint()
	if id == 0 {
		t.Fatalf("create 响应缺 ID, body=%s", body)
	}

	poolID := uint(id)
	cred := &models.Credential{Key: "enc-key", KeyHash: "hash-" + name, PoolID: &poolID, Status: status}
	if err := repository.Default().Credential.Create(t.Context(), cred); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
	return poolID
}

// TestGetPools_ListItems_StatsShape 覆盖 AC-1 + AC-5：
// 号池列表项必须含直返字段（ID/Name/Note）与统计字段（KeyCount/StatusCounts/ReferencedBy），
// 统计数字精确；空号池统计字段仍存在且为 0（防 map nil 序列化 null 断层）。
func TestGetPools_ListItems_StatsShape(t *testing.T) {
	testsupport.InitTestDB(t)
	seedPoolWithCredential(t, "主池", models.CredentialStatusActive)
	seedPoolWithCredential(t, "备用池", models.CredentialStatusDisabled)

	// 引用「主池」：key_groups.PoolID 指向它
	mainID := gjson.Get(listPoolsViaHandler(t), "data.0.ID").Uint()
	group := &models.KeyGroup{ProviderID: 1, Name: "默认组", PoolID: uintPtr(uint(mainID))}
	if err := repository.Default().KeyGroup.Create(t.Context(), group); err != nil {
		t.Fatalf("seed key group: %v", err)
	}

	body := listPoolsViaHandler(t)

	// 两个号池都在
	if got := gjson.Get(body, "data.#").Int(); got != 2 {
		t.Fatalf("列表数量 = %d, want 2, body=%s", got, body)
	}

	// 第一项：直返字段 + 统计字段（AC-1 键存在性）
	first := "data.0."
	for _, key := range []string{"ID", "Name", "Note", "KeyCount", "StatusCounts", "ReferencedBy"} {
		if !gjson.Get(body, first+key).Exists() {
			t.Fatalf("%s 键缺失, body=%s", first+key, body)
		}
	}
	if got := gjson.Get(body, first+"KeyCount").Int(); got != 1 {
		t.Fatalf("主池 KeyCount = %d, want 1", got)
	}
	if got := gjson.Get(body, first+"ReferencedBy").Int(); got != 1 {
		t.Fatalf("主池 ReferencedBy = %d, want 1", got)
	}
	// StatusCounts 是 map，active 计数为 1
	if got := gjson.Get(body, first+"StatusCounts.active").Int(); got != 1 {
		t.Fatalf("主池 StatusCounts.active = %d, want 1", got)
	}

	// 第二项：备用池被引用数为 0、disabled 计数为 1
	second := "data.1."
	if got := gjson.Get(body, second+"ReferencedBy").Int(); got != 0 {
		t.Fatalf("备用池 ReferencedBy = %d, want 0", got)
	}
	if got := gjson.Get(body, second+"StatusCounts.disabled").Int(); got != 1 {
		t.Fatalf("备用池 StatusCounts.disabled = %d, want 1", got)
	}
}

// TestGetPools_EmptyPool_StatsZero 覆盖 AC-1 边界：无凭据的空号池
// 统计字段必须是 0 而非缺失（handler 显式置空 map 保证键存在）。
func TestGetPools_EmptyPool_StatsZero(t *testing.T) {
	testsupport.InitTestDB(t)
	createPoolViaHandler(t, `{"name":"空池"}`)

	body := listPoolsViaHandler(t)

	if got := gjson.Get(body, "data.0.KeyCount").Int(); got != 0 {
		t.Fatalf("空池 KeyCount = %d, want 0", got)
	}
	if got := gjson.Get(body, "data.0.ReferencedBy").Int(); got != 0 {
		t.Fatalf("空池 ReferencedBy = %d, want 0", got)
	}
	if got := gjson.Get(body, "data.0.StatusCounts.#").Int(); got != 0 {
		t.Fatalf("空池 StatusCounts 应为空对象, body=%s", body)
	}
}

// TestCreatePool_NameValidation 覆盖 AC-2：name 缺失/重复均返回业务错误。
func TestCreatePool_NameValidation(t *testing.T) {
	testsupport.InitTestDB(t)

	// name 缺失
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/pools", strings.NewReader(`{"note":"no-name"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	CreatePool(c)
	if w.Code != 200 {
		t.Fatalf("BadRequest 也应走 200 信封, got %d", w.Code)
	}
	if code := gjson.Get(w.Body.String(), "code").Int(); code != 400 {
		t.Fatalf("name 缺失 code = %d, want 400, body=%s", code, w.Body.String())
	}

	// 重名
	createPoolViaHandler(t, `{"name":"重名池"}`)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("POST", "/pools", strings.NewReader(`{"name":"重名池"}`))
	c2.Request.Header.Set("Content-Type", "application/json")
	CreatePool(c2)
	if code := gjson.Get(w2.Body.String(), "code").Int(); code != 400 {
		t.Fatalf("重名 code = %d, want 400, body=%s", code, w2.Body.String())
	}
}

// TestUpdatePool_NotFoundAndNoteClear 覆盖 AC-3：
// id 不存在返回 404；note 可清空（走 UpdateFields，struct 更新会跳过零值写不进空串）。
func TestUpdatePool_NotFoundAndNoteClear(t *testing.T) {
	testsupport.InitTestDB(t)
	createPoolViaHandler(t, `{"name":"带注记池","note":"旧注记"}`)

	// 不存在 id → 404
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "9999"}}
	c.Request = httptest.NewRequest("PUT", "/pools/9999", strings.NewReader(`{"name":"x"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdatePool(c)
	if code := gjson.Get(w.Body.String(), "code").Int(); code != 404 {
		t.Fatalf("不存在 id code = %d, want 404, body=%s", code, w.Body.String())
	}

	// note 清空
	body := listPoolsViaHandler(t)
	id := gjson.Get(body, "data.0.ID").Uint()
	updatePoolViaHandler(t, uint(id), `{"name":"带注记池","note":""}`)
	after := listPoolsViaHandler(t)
	if got := gjson.Get(after, "data.0.Note").String(); got != "" {
		t.Fatalf("清空后 Note = %q, want 空串", got)
	}
}

// TestDeletePool_ReferencedGuard 覆盖 AC-4 前半：被分组引用时禁止删除，
// 号池本体与凭据都不得删除。
func TestDeletePool_ReferencedGuard(t *testing.T) {
	testsupport.InitTestDB(t)
	id := seedPoolWithCredential(t, "被引用池", models.CredentialStatusActive)
	group := &models.KeyGroup{ProviderID: 1, Name: "默认组", PoolID: uintPtr(id)}
	if err := repository.Default().KeyGroup.Create(t.Context(), group); err != nil {
		t.Fatalf("seed key group: %v", err)
	}

	body, _ := deletePoolViaHandler(t, id)
	if code := gjson.Get(body, "code").Int(); code != 400 {
		t.Fatalf("被引用删除 code = %d, want 400, body=%s", code, body)
	}

	// 号池还在
	if _, err := repository.Default().Pool.Get(t.Context(), id); err != nil {
		t.Fatalf("被引用删除不应删掉号池: %v", err)
	}
	// 凭据还在
	creds, err := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: uintPtr(id)})
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("被引用删除后凭据数 = %d, want 1", len(creds))
	}
}

// TestDeletePool_CascadeCredentials 覆盖 AC-4 后半：未引用时删除号池
// 并级联删除其下凭据（软删），响应成功。
func TestDeletePool_CascadeCredentials(t *testing.T) {
	testsupport.InitTestDB(t)
	id := seedPoolWithCredential(t, "待删池", models.CredentialStatusActive)

	body, code := deletePoolViaHandler(t, id)
	if code != 200 {
		t.Fatalf("未引用删除 status = %d, want 200, body=%s", code, body)
	}

	if _, err := repository.Default().Pool.Get(t.Context(), id); err == nil {
		t.Fatal("删除后 Get 应报错")
	}
	creds, err := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: uintPtr(id)})
	if err != nil {
		t.Fatalf("list credentials: %v", err)
	}
	if len(creds) != 0 {
		t.Fatalf("级联删除后凭据数 = %d, want 0", len(creds))
	}
}

// uintPtr 构建 *uint 指针。
func uintPtr(v uint) *uint { return &v }
