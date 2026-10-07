package credentials

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// TestListCredentials_FilterByOwnership 锁「归属只是列表过滤条件」这条域边界：
// 凭据一律归属号池，pool_id 是唯一的归属切口，不带过滤即全量。
func TestListCredentials_FilterByOwnership(t *testing.T) {
	initCredentialsTestDB(t)
	poolA, _ := seedPoolCredential(t, "列表池A", "sk-poolA-ABCD1234", models.CredentialStatusActive)
	poolB, _ := seedPoolCredential(t, "列表池B", "sk-poolB-ABCD1234", models.CredentialStatusActive)

	all, code := invoke(t, "GET", "/credentials", nil, "", ListCredentials)
	if code != 200 || gjson.Get(all, "data.total").Int() != 2 {
		t.Fatalf("不带过滤应回全量 2 条 body=%s", all)
	}

	byPool, _ := invoke(t, "GET", "/credentials?pool_id="+itoa(poolA), nil, "", ListCredentials)
	if gjson.Get(byPool, "data.total").Int() != 1 {
		t.Fatalf("pool_id 过滤 total=%d want 1 body=%s", gjson.Get(byPool, "data.total").Int(), byPool)
	}
	if got := gjson.Get(byPool, "data.items.0.PoolID").Uint(); got != uint64(poolA) {
		t.Fatalf("pool_id 过滤回错行 PoolID=%d body=%s", got, byPool)
	}

	// 另一个池的凭据不得泄漏进过滤结果
	if strings.Contains(byPool, "sk-poolB") {
		t.Fatalf("pool_id 过滤泄漏他池凭据: %s", byPool)
	}
	// PoolID 是普通 uint，响应恒为数值且键必存在（不再是可空指针）
	poolKey := gjson.Get(byPool, "data.items.0.PoolID")
	if !poolKey.Exists() || poolKey.Type != gjson.Number {
		t.Fatalf("PoolID 应为数值且键存在, got %s", poolKey.Raw)
	}
	_ = poolB
}

// TestListCredentials_StatusSearchPaginationMask 覆盖状态筛选、备注搜索、分页与掩码不泄漏。
func TestListCredentials_StatusSearchPaginationMask(t *testing.T) {
	initCredentialsTestDB(t)
	poolID := seedPool(t, "筛选池")
	activeID := seedPoolMember(t, poolID, "sk-active-ABCD1234", models.CredentialStatusActive)
	seedPoolMember(t, poolID, "sk-disabled-ABCD1234", models.CredentialStatusDisabled)
	if _, err := repository.Default().Credential.UpdateFields(t.Context(), activeID,
		map[string]any{"note": "alpha-note"}); err != nil {
		t.Fatal(err)
	}

	byStatus, _ := invoke(t, "GET", "/credentials?pool_id="+itoa(poolID)+"&status=disabled", nil, "", ListCredentials)
	if gjson.Get(byStatus, "data.total").Int() != 1 {
		t.Fatalf("status 过滤 total=%d want 1 body=%s", gjson.Get(byStatus, "data.total").Int(), byStatus)
	}
	if got := gjson.Get(byStatus, "data.items.0.Status").String(); got != models.CredentialStatusDisabled {
		t.Fatalf("status 过滤回错行 Status=%q body=%s", got, byStatus)
	}

	// status=all 视同不过滤（前端筛选下拉「全部」直传）
	allStatus, _ := invoke(t, "GET", "/credentials?pool_id="+itoa(poolID)+"&status=all", nil, "", ListCredentials)
	if gjson.Get(allStatus, "data.total").Int() != 2 {
		t.Fatalf("status=all total=%d want 2 body=%s", gjson.Get(allStatus, "data.total").Int(), allStatus)
	}

	byQ, _ := invoke(t, "GET", "/credentials?pool_id="+itoa(poolID)+"&q=alpha", nil, "", ListCredentials)
	if gjson.Get(byQ, "data.total").Int() != 1 {
		t.Fatalf("q 搜索 total=%d want 1 body=%s", gjson.Get(byQ, "data.total").Int(), byQ)
	}

	page, _ := invoke(t, "GET", "/credentials?pool_id="+itoa(poolID)+"&page=1&page_size=1", nil, "", ListCredentials)
	if gjson.Get(page, "data.total").Int() != 2 || gjson.Get(page, "data.items.#").Int() != 1 {
		t.Fatalf("分页 total/items 不符 body=%s", page)
	}

	// 掩码：出服务端掩码，且不含明文/密文/hash 键
	plain := "sk-active-ABCD1234"
	if got := gjson.Get(byQ, "data.items.0.KeyMasked").String(); got != credentialcrypto.MaskKey(plain) {
		t.Fatalf("KeyMasked=%q want %q body=%s", got, credentialcrypto.MaskKey(plain), byQ)
	}
	for _, forbidden := range []string{"KeyHash", "key_hash", `"Key"`, `"key"`} {
		if strings.Contains(byQ, forbidden) {
			t.Fatalf("列表响应不应含 %s: body=%s", forbidden, byQ)
		}
	}
}

// TestListCredentials_InvalidFilters 非法状态/非法归属过滤值一律 400（不静默按不过滤处理）。
func TestListCredentials_InvalidFilters(t *testing.T) {
	initCredentialsTestDB(t)
	for _, query := range []string{"?status=bogus", "?pool_id=abc"} {
		t.Run(query, func(t *testing.T) {
			body, _ := invoke(t, "GET", "/credentials"+query, nil, "", ListCredentials)
			if gjson.Get(body, "code").Int() != 400 {
				t.Fatalf("应回 400 body=%s", body)
			}
		})
	}
}

// TestCreateCredential_Success_MaskNoLeak 创建成功：回掩码、不泄漏明文/密文/hash，
// 且创建响应与随后 GET 的字段完全一致（视图单源，两条路径不得漂移）。
func TestCreateCredential_Success_MaskNoLeak(t *testing.T) {
	initCredentialsTestDB(t)
	poolID := seedPool(t, "创建池")
	plain := "sk-create-1234567890ABCD"

	body, _ := invoke(t, "POST", "/credentials", nil,
		`{"pool_id":`+itoa(poolID)+`,"key":"`+plain+`","note":"首条"}`, CreateCredential)
	if code := gjson.Get(body, "code").Int(); code != 200 {
		t.Fatalf("code=%d want 200 body=%s", code, body)
	}
	credID := dataID(t, body)
	if got := gjson.Get(body, "data.KeyMasked").String(); got != credentialcrypto.MaskKey(plain) {
		t.Fatalf("KeyMasked=%q want %q", got, credentialcrypto.MaskKey(plain))
	}
	if gjson.Get(body, "data.Key").Exists() || gjson.Get(body, "data.KeyHash").Exists() {
		t.Fatalf("不应返回明文/KeyHash body=%s", body)
	}
	if got := gjson.Get(body, "data.PoolID").Uint(); got != uint64(poolID) {
		t.Fatalf("PoolID=%d want %d body=%s", got, poolID, body)
	}

	getBody, _ := invoke(t, "GET", "/credentials/"+itoa(credID), credentialParams(credID), "", GetCredential)
	for _, k := range []string{"ID", "PoolID", "Status", "Note", "KeyMasked", "CooldownUntil",
		"CooldownReason", "FailCount", "LastUsedAt", "LastProbeAt", "TotalRequests", "TotalErrors",
		"TotalTokens", "CreatedAt", "UpdatedAt"} {
		if createV, getV := gjson.Get(body, "data."+k).Raw, gjson.Get(getBody, "data."+k).Raw; createV != getV {
			t.Fatalf("字段 %s 创建响应 %s != 重新 GET %s", k, createV, getV)
		}
	}
}

// TestCreateCredential_Validation 覆盖：缺 pool_id 400、池不存在 404、空 key 400、
// 池内重复 key 400、跨池同 key 允许。
func TestCreateCredential_Validation(t *testing.T) {
	initCredentialsTestDB(t)
	poolA := seedPool(t, "校验池A")
	poolB := seedPool(t, "校验池B")
	plain := "sk-dup-ABCD1234"

	if body, _ := invoke(t, "POST", "/credentials", nil, `{"key":"`+plain+`"}`, CreateCredential); gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("缺 pool_id 应 400 body=%s", body)
	}
	if body, _ := invoke(t, "POST", "/credentials", nil, `{"pool_id":9999,"key":"sk-x-ABCD1234"}`, CreateCredential); gjson.Get(body, "code").Int() != 404 {
		t.Fatalf("池不存在应 404 body=%s", body)
	}
	if body, _ := invoke(t, "POST", "/credentials", nil, `{"pool_id":`+itoa(poolA)+`,"key":"  "}`, CreateCredential); gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("空 key 应 400 body=%s", body)
	}

	first, _ := invoke(t, "POST", "/credentials", nil, `{"pool_id":`+itoa(poolA)+`,"key":"`+plain+`"}`, CreateCredential)
	if dataID(t, first) == 0 {
		t.Fatalf("首次创建失败 body=%s", first)
	}
	if dup, _ := invoke(t, "POST", "/credentials", nil, `{"pool_id":`+itoa(poolA)+`,"key":"`+plain+`"}`, CreateCredential); gjson.Get(dup, "code").Int() != 400 {
		t.Fatalf("池内重复 key 应 400 body=%s", dup)
	}
	// 跨池同 key 合法（同一把 key 复用进另一个池）
	if crossPool, _ := invoke(t, "POST", "/credentials", nil, `{"pool_id":`+itoa(poolB)+`,"key":"`+plain+`"}`, CreateCredential); gjson.Get(crossPool, "code").Int() != 200 {
		t.Fatalf("跨池同 key 应允许 body=%s", crossPool)
	}
}

// TestUpdateCredential_RecoversStopsCredential 本域存在的意义：凭据被判停
// （连续鉴权失败 → temp_unsched）后能人工恢复——切回 active 并清掉连败计数与判停原因。
func TestUpdateCredential_RecoversStopsCredential(t *testing.T) {
	initCredentialsTestDB(t)
	poolID := seedPool(t, "恢复池")
	credID := seedCredential(t, poolID, "sk-recover-ABCD1234",
		models.CredentialStatusTempUnsched, 3, "auth_fail")

	body, _ := invoke(t, "PATCH", "/credentials/"+itoa(credID), credentialParams(credID),
		`{"status":"active"}`, UpdateCredential)
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("恢复应 200 body=%s", body)
	}
	if got := gjson.Get(body, "data.Status").String(); got != models.CredentialStatusActive {
		t.Fatalf("Status=%q want active body=%s", got, body)
	}
	if got := gjson.Get(body, "data.FailCount").Int(); got != 0 {
		t.Fatalf("FailCount=%d want 0（恢复须清连败计数）body=%s", got, body)
	}
	if got := gjson.Get(body, "data.CooldownReason").String(); got != "" {
		t.Fatalf("CooldownReason=%q want 空（恢复须清判停原因）body=%s", got, body)
	}

	stored := reloadCredential(t, credID)
	if stored.Status != models.CredentialStatusActive || stored.FailCount != 0 || stored.CooldownReason != "" {
		t.Fatalf("落库字段未按状态机约定重置: %+v", stored)
	}
}

// TestUpdateCredential_RecoveryClearsCooldown 锁「恢复 = 立刻可用」（Stage 40）：
// 冷却中的凭据点启用必须连 cooldown_until 一起清——选路按 CooldownUntil 剔除，
// 只清失败计数会留下「状态是启用、实际仍用不了」的假象。
func TestUpdateCredential_RecoveryClearsCooldown(t *testing.T) {
	initCredentialsTestDB(t)
	poolID := seedPool(t, "冷却恢复池")
	credID := seedCredential(t, poolID, "sk-cool-recover-ABCD1234",
		models.CredentialStatusActive, 2, "http_429")
	markCredentialCooling(t, credID, time.Now().Add(10*time.Minute))

	body, _ := invoke(t, "PATCH", "/credentials/"+itoa(credID), credentialParams(credID),
		`{"status":"active"}`, UpdateCredential)
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("恢复应 200 body=%s", body)
	}
	// Exists() 区分「键存在且为 null」与「键缺失」——三态指针禁 omitempty，
	// 键必须在且显式为 null（前端据此判恢复是否生效）。
	if v := gjson.Get(body, "data.CooldownUntil"); !v.Exists() || v.Type != gjson.Null {
		t.Fatalf("CooldownUntil=%v (exists=%v) want 键存在且为 null body=%s", v.Value(), v.Exists(), body)
	}
	if stored := reloadCredential(t, credID); stored.CooldownUntil != nil {
		t.Fatalf("落库 CooldownUntil=%v want nil（恢复须解除冷却）", stored.CooldownUntil)
	}
}

// TestUpdateCredential_NoteAndValidation 覆盖 note 更新（可清空）、无字段 400、
// 非法 status 400、凭据不存在 404，并锁「更新备注不动归属」。
func TestUpdateCredential_NoteAndValidation(t *testing.T) {
	initCredentialsTestDB(t)
	poolID, credID := seedPoolCredential(t, "更新池", "sk-update-ABCD1234", models.CredentialStatusActive)

	noteBody, _ := invoke(t, "PATCH", "/credentials/"+itoa(credID), credentialParams(credID),
		`{"note":"备注A"}`, UpdateCredential)
	if got := gjson.Get(noteBody, "data.Note").String(); got != "备注A" {
		t.Fatalf("Note=%q want 备注A body=%s", got, noteBody)
	}
	clearBody, _ := invoke(t, "PATCH", "/credentials/"+itoa(credID), credentialParams(credID),
		`{"note":""}`, UpdateCredential)
	if got := gjson.Get(clearBody, "data.Note").String(); got != "" {
		t.Fatalf("Note 应可清空 got=%q body=%s", got, clearBody)
	}
	if stored := reloadCredential(t, credID); stored.PoolID != poolID {
		t.Fatalf("更新备注不应改动归属: %+v", stored)
	}

	if body, _ := invoke(t, "PATCH", "/credentials/"+itoa(credID), credentialParams(credID), `{}`, UpdateCredential); gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("无字段应 400 body=%s", body)
	}
	if body, _ := invoke(t, "PATCH", "/credentials/"+itoa(credID), credentialParams(credID), `{"status":"bogus"}`, UpdateCredential); gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("非法 status 应 400 body=%s", body)
	}
	missing := uint(99999)
	if body, _ := invoke(t, "PATCH", "/credentials/"+itoa(missing), credentialParams(missing), `{"status":"active"}`, UpdateCredential); gjson.Get(body, "code").Int() != 404 {
		t.Fatalf("凭据不存在应 404 body=%s", body)
	}
}

// TestGetCredentialRaw_PlaintextNoStore 明文受控端点：回明文 + `Cache-Control: no-store`
// （明文进浏览器缓存就绕过了前端的生命周期守卫）。
func TestGetCredentialRaw_PlaintextNoStore(t *testing.T) {
	initCredentialsTestDB(t)
	_, credID := seedPoolCredential(t, "明文池", "sk-raw-ABCD1234", models.CredentialStatusActive)

	rec := rawRecorder(t, credID)
	if rec.Code != 200 {
		t.Fatalf("raw HTTP=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := gjson.Get(rec.Body.String(), "data.key").String(); got != "sk-raw-ABCD1234" {
		t.Fatalf("明文=%q body=%s", got, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q want no-store", got)
	}
}

// TestGetCredentialRaw_Failures 解密失败与 cipher 未配置都回 500，且不泄漏明文。
func TestGetCredentialRaw_Failures(t *testing.T) {
	initCredentialsTestDB(t)
	_, credID := seedPoolCredential(t, "损坏池", "sk-tamper-ABCD1234", models.CredentialStatusActive)

	stored := reloadCredential(t, credID)
	if _, err := repository.Default().Credential.UpdateFields(t.Context(), credID,
		map[string]any{"key": "aa" + stored.Key[2:]}); err != nil {
		t.Fatal(err)
	}
	rec := rawRecorder(t, credID)
	if rec.Code != 500 || gjson.Get(rec.Body.String(), "code").Int() != 500 {
		t.Fatalf("解密失败应 500 HTTP=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sk-tamper") {
		t.Fatalf("失败响应不得泄漏明文 body=%s", rec.Body.String())
	}

	// cipher 未配置：同样是 500（配置缺失不谎报成凭据问题）
	credentialcrypto.SetDefault(nil)
	if rec2 := rawRecorder(t, credID); rec2.Code != 500 {
		t.Fatalf("无 cipher 应 500 HTTP=%d body=%s", rec2.Code, rec2.Body.String())
	}
}

// TestDeleteCredential_AndNotFound 单条软删 + 不存在 404。
func TestDeleteCredential_AndNotFound(t *testing.T) {
	initCredentialsTestDB(t)
	_, credID := seedPoolCredential(t, "删除池", "sk-del-ABCD1234", models.CredentialStatusActive)

	body, _ := invoke(t, "DELETE", "/credentials/"+itoa(credID), credentialParams(credID), "", DeleteCredential)
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("删除应 200 body=%s", body)
	}
	if after, _ := invoke(t, "GET", "/credentials/"+itoa(credID), credentialParams(credID), "", GetCredential); gjson.Get(after, "code").Int() != 404 {
		t.Fatalf("软删后应按 404 body=%s", after)
	}

	missing := uint(99999)
	if body, _ := invoke(t, "DELETE", "/credentials/"+itoa(missing), credentialParams(missing), "", DeleteCredential); gjson.Get(body, "code").Int() != 404 {
		t.Fatalf("不存在应 404 body=%s", body)
	}
}

// rawRecorder 走 GetCredentialRaw 并返回完整 recorder（除 body/code 外还要断言响应头）。
func rawRecorder(t *testing.T, credID uint) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = credentialParams(credID)
	c.Request = httptest.NewRequest("GET", "/credentials/"+itoa(credID)+"/raw", nil)
	GetCredentialRaw(c)
	return w
}
