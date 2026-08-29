package pools

import (
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

// listCredentialsViaHandler 走 ListCredentials handler 并返回响应体与 HTTP 状态码。
func listCredentialsViaHandler(t *testing.T, poolID uint, query string) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(poolID), 10)}}
	path := "/pools/" + strconv.FormatUint(uint64(poolID), 10) + "/credentials"
	if query != "" {
		if !strings.HasPrefix(query, "?") {
			query = "?" + query
		}
		path += query
	}
	c.Request = httptest.NewRequest("GET", path, nil)
	ListCredentials(c)
	return w.Body.String(), w.Code
}

// seedPoolForCredList 建号池并返回 ID。
func seedPoolForCredList(t *testing.T, name string) uint {
	t.Helper()
	body := createPoolViaHandler(t, `{"name":"`+name+`"}`)
	id := gjson.Get(body, "data.ID").Uint()
	if id == 0 {
		t.Fatalf("create 响应缺 ID, body=%s", body)
	}
	return uint(id)
}

// seedCredential 直接写库（Key 按传入存，不加密；测试中 cipher nil 时直接当明文掩码）。
func seedCredential(t *testing.T, poolID uint, key string, status string, note string) {
	t.Helper()
	cred := &models.Credential{Key: key, KeyHash: "hash-" + key, PoolID: &poolID, Status: status, Note: note}
	if err := repository.Default().Credential.Create(t.Context(), cred); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
}

// TestListCredentials_Pagination 覆盖 AC-1 + AC-5：分页总数与切片正确，越界返回空列表。
func TestListCredentials_Pagination(t *testing.T) {
	testsupport.InitTestDB(t)
	poolID := seedPoolForCredList(t, "分页池")
	for i := 0; i < 25; i++ {
		seedCredential(t, poolID, "sk-key-"+strconv.Itoa(1000+i)+"-ABCD", models.CredentialStatusActive, "")
	}

	body, code := listCredentialsViaHandler(t, poolID, "page=1&page_size=10")
	if code != 200 {
		t.Fatalf("page1 status=%d body=%s", code, body)
	}
	if got := gjson.Get(body, "code").Int(); got != 200 {
		t.Fatalf("code = %d, want 200, body=%s", got, body)
	}
	if got := gjson.Get(body, "data.total").Int(); got != 25 {
		t.Fatalf("total = %d, want 25, body=%s", got, body)
	}
	if got := gjson.Get(body, "data.items.#").Int(); got != 10 {
		t.Fatalf("page1 items = %d, want 10, body=%s", got, body)
	}
	if got := gjson.Get(body, "data.page").Int(); got != 1 {
		t.Fatalf("page = %d, want 1", got)
	}
	if got := gjson.Get(body, "data.page_size").Int(); got != 10 {
		t.Fatalf("page_size = %d, want 10", got)
	}
	// 第二页
	body2, _ := listCredentialsViaHandler(t, poolID, "page=2&page_size=10")
	if got := gjson.Get(body2, "data.items.#").Int(); got != 10 {
		t.Fatalf("page2 items = %d, want 10, body=%s", got, body2)
	}
	// 第三页剩余 5
	body3, _ := listCredentialsViaHandler(t, poolID, "page=3&page_size=10")
	if got := gjson.Get(body3, "data.items.#").Int(); got != 5 {
		t.Fatalf("page3 items = %d, want 5, body=%s", got, body3)
	}
	// 越界页空列表但 total 仍正确
	body4, _ := listCredentialsViaHandler(t, poolID, "page=99&page_size=10")
	if got := gjson.Get(body4, "data.items.#").Int(); got != 0 {
		t.Fatalf("越界页 items = %d, want 0, body=%s", got, body4)
	}
	if got := gjson.Get(body4, "data.total").Int(); got != 25 {
		t.Fatalf("越界页 total = %d, want 25, body=%s", got, body4)
	}
}

// TestListCredentials_EmptyPool 覆盖 AC-5：空池返回空列表与 total=0
func TestListCredentials_EmptyPool(t *testing.T) {
	testsupport.InitTestDB(t)
	poolID := seedPoolForCredList(t, "空凭据池")
	body, _ := listCredentialsViaHandler(t, poolID, "")
	if got := gjson.Get(body, "data.total").Int(); got != 0 {
		t.Fatalf("空池 total = %d, want 0, body=%s", got, body)
	}
	if got := gjson.Get(body, "data.items.#").Int(); got != 0 {
		t.Fatalf("空池 items = %d, want 0, body=%s", got, body)
	}
	// gjson 需 items 键存在且为数组
	if !gjson.Get(body, "data.items").Exists() {
		t.Fatalf("空池 data.items 键缺失, body=%s", body)
	}
}

// TestListCredentials_StatusFilter 覆盖 AC-2：status 筛选
func TestListCredentials_StatusFilter(t *testing.T) {
	testsupport.InitTestDB(t)
	poolID := seedPoolForCredList(t, "状态池")
	seedCredential(t, poolID, "sk-active-1-ABCD", models.CredentialStatusActive, "")
	seedCredential(t, poolID, "sk-disabled-1-ABCD", models.CredentialStatusDisabled, "")
	seedCredential(t, poolID, "sk-error-1-ABCD", models.CredentialStatusError, "")
	seedCredential(t, poolID, "sk-active-2-ABCD", models.CredentialStatusActive, "")

	body, _ := listCredentialsViaHandler(t, poolID, "status=active")
	if got := gjson.Get(body, "data.total").Int(); got != 2 {
		t.Fatalf("active total = %d, want 2, body=%s", got, body)
	}
	if got := gjson.Get(body, "data.items.#").Int(); got != 2 {
		t.Fatalf("active items = %d, want 2, body=%s", got, body)
	}
	for i := 0; i < int(gjson.Get(body, "data.items.#").Int()); i++ {
		if got := gjson.Get(body, "data.items."+strconv.Itoa(i)+".Status").String(); got != models.CredentialStatusActive {
			t.Fatalf("items[%d].Status = %q, want active, body=%s", i, got, body)
		}
	}

	// status=all 返回全部
	bodyAll, _ := listCredentialsViaHandler(t, poolID, "status=all")
	if got := gjson.Get(bodyAll, "data.total").Int(); got != 4 {
		t.Fatalf("all total = %d, want 4, body=%s", got, bodyAll)
	}

	// 非法 status 400
	bodyBad, _ := listCredentialsViaHandler(t, poolID, "status=invalid")
	if code := gjson.Get(bodyBad, "code").Int(); code != 400 {
		t.Fatalf("非法 status code = %d, want 400, body=%s", code, bodyBad)
	}
}

// TestListCredentials_Search 覆盖 AC-3：q 关键词搜索 Note
func TestListCredentials_Search(t *testing.T) {
	testsupport.InitTestDB(t)
	poolID := seedPoolForCredList(t, "搜索池")
	seedCredential(t, poolID, "sk-key1-ABCD", models.CredentialStatusActive, "alpha-note")
	seedCredential(t, poolID, "sk-key2-ABCD", models.CredentialStatusActive, "beta-note")
	seedCredential(t, poolID, "sk-key3-ABCD", models.CredentialStatusActive, "alpha-beta")

	body, _ := listCredentialsViaHandler(t, poolID, "q=alpha")
	if got := gjson.Get(body, "data.total").Int(); got != 2 {
		t.Fatalf("q=alpha total = %d, want 2, body=%s", got, body)
	}
	// q 为空不过滤
	body2, _ := listCredentialsViaHandler(t, poolID, "")
	if got := gjson.Get(body2, "data.total").Int(); got != 3 {
		t.Fatalf("q空 total = %d, want 3, body=%s", got, body2)
	}
	// 命中数为 0 的关键词
	body3, _ := listCredentialsViaHandler(t, poolID, "q=nonexistent")
	if got := gjson.Get(body3, "data.total").Int(); got != 0 {
		t.Fatalf("q=nonexistent total = %d, want 0, body=%s", got, body3)
	}
	if got := gjson.Get(body3, "data.items.#").Int(); got != 0 {
		t.Fatalf("q=nonexistent items = %d, want 0, body=%s", got, body3)
	}
}

// TestListCredentials_Mask_NoLeak 覆盖 AC-4：掩码形状且不泄漏明文/密文
func TestListCredentials_Mask_NoLeak(t *testing.T) {
	testsupport.InitTestDB(t)
	poolID := seedPoolForCredList(t, "掩码池")
	// 长度 >8 的 key，掩码应为 前3 + **** + 后4
	longKey := "sk-proto-abcdef1234567890ab12"
	seedCredential(t, poolID, longKey, models.CredentialStatusActive, "")

	body, _ := listCredentialsViaHandler(t, poolID, "")

	// 键存在性：KeyMasked 存在，Key/key/KeyHash 等敏感键缺失
	if !gjson.Get(body, "data.items.0.KeyMasked").Exists() {
		t.Fatalf("KeyMasked 键缺失, body=%s", body)
	}
	if gjson.Get(body, "data.items.0.Key").Exists() {
		t.Fatalf("不应返回 Key 明文/密文, body=%s", body)
	}
	if gjson.Get(body, "data.items.0.key").Exists() {
		t.Fatalf("不应返回 key 小写, body=%s", body)
	}
	if gjson.Get(body, "data.items.0.KeyHash").Exists() {
		t.Fatalf("不应返回 KeyHash, body=%s", body)
	}
	expected := longKey[:3] + "****" + longKey[len(longKey)-4:]
	if got := gjson.Get(body, "data.items.0.KeyMasked").String(); got != expected {
		t.Fatalf("KeyMasked = %q, want %q, body=%s", got, expected, body)
	}
	// 响应体中不应出现明文 key 子串（密文也不会等同明文，但测试用明文种子可直接判）
	if strings.Contains(body, longKey) {
		t.Fatalf("响应体泄漏明文 key, body=%s", body)
	}

	// 短 key 退化为 ****
	poolID2 := seedPoolForCredList(t, "短掩码池")
	seedCredential(t, poolID2, "ab", models.CredentialStatusActive, "")
	body2, _ := listCredentialsViaHandler(t, poolID2, "")
	if got := gjson.Get(body2, "data.items.0.KeyMasked").String(); got != "****" {
		t.Fatalf("短 key KeyMasked = %q, want ****, body=%s", got, body2)
	}
}

// TestListCredentials_Mask_WithEncryptedKey 覆盖 AC-4 加密路径：密文解密后掩码，响应仍不含密文
func TestListCredentials_Mask_WithEncryptedKey(t *testing.T) {
	testsupport.InitTestDB(t)
	// 生成临时 cipher 并注入 Default（InitTestDB 后注入，不影响库初始化）
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	c, err := credentialcrypto.New(hexKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })

	poolID := seedPoolForCredList(t, "加密掩码池")
	plain := "sk-encrypted-key-12345678AB12"
	enc, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	seedCredential(t, poolID, enc, models.CredentialStatusActive, "")

	body, _ := listCredentialsViaHandler(t, poolID, "")
	if gjson.Get(body, "data.items.0.Key").Exists() {
		t.Fatalf("加密路径不应返回 Key, body=%s", body)
	}
	if strings.Contains(body, enc) {
		t.Fatalf("响应体泄漏密文, body=%s", body)
	}
	if strings.Contains(body, plain) {
		t.Fatalf("响应体泄漏明文, body=%s", body)
	}
	expected := plain[:3] + "****" + plain[len(plain)-4:]
	if got := gjson.Get(body, "data.items.0.KeyMasked").String(); got != expected {
		t.Fatalf("加密后 KeyMasked = %q, want %q, body=%s", got, expected, body)
	}
}

// TestListCredentials_PoolNotFound 覆盖池不存在 404
func TestListCredentials_PoolNotFound(t *testing.T) {
	testsupport.InitTestDB(t)
	body, _ := listCredentialsViaHandler(t, 9999, "")
	if code := gjson.Get(body, "code").Int(); code != 404 {
		t.Fatalf("不存在池 code = %d, want 404, body=%s", code, body)
	}
}

// TestListCredentials_InvalidPagination 覆盖 AC-5 非法分页 400
func TestListCredentials_InvalidPagination(t *testing.T) {
	testsupport.InitTestDB(t)
	poolID := seedPoolForCredList(t, "分页校验池")
	seedCredential(t, poolID, "sk-key-ABCD", models.CredentialStatusActive, "")

	body, _ := listCredentialsViaHandler(t, poolID, "page=0")
	if code := gjson.Get(body, "code").Int(); code != 400 {
		t.Fatalf("page=0 code = %d, want 400, body=%s", code, body)
	}
	body2, _ := listCredentialsViaHandler(t, poolID, "page_size=0")
	if code := gjson.Get(body2, "code").Int(); code != 400 {
		t.Fatalf("page_size=0 code = %d, want 400, body=%s", code, body2)
	}
	body3, _ := listCredentialsViaHandler(t, poolID, "page=abc")
	if code := gjson.Get(body3, "code").Int(); code != 400 {
		t.Fatalf("page=abc code = %d, want 400, body=%s", code, body3)
	}
}
