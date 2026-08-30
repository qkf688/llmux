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

const testCredHexKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func setupCipherForCredTest(t *testing.T) {
	t.Helper()
	c, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
}

func createCredentialViaHandler(t *testing.T, poolID uint, body string) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(poolID), 10)}}
	c.Request = httptest.NewRequest("POST", "/pools/"+strconv.FormatUint(uint64(poolID), 10)+"/credentials", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	CreateCredential(c)
	return w.Body.String(), w.Code
}

func getCredentialViaHandler(t *testing.T, poolID, credID uint) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(poolID), 10)}, {Key: "credId", Value: strconv.FormatUint(uint64(credID), 10)}}
	c.Request = httptest.NewRequest("GET", "/pools/"+strconv.FormatUint(uint64(poolID), 10)+"/credentials/"+strconv.FormatUint(uint64(credID), 10), nil)
	GetCredential(c)
	return w.Body.String(), w.Code
}

func getCredentialRawViaHandler(t *testing.T, poolID, credID uint) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(poolID), 10)}, {Key: "credId", Value: strconv.FormatUint(uint64(credID), 10)}}
	c.Request = httptest.NewRequest("GET", "/pools/"+strconv.FormatUint(uint64(poolID), 10)+"/credentials/"+strconv.FormatUint(uint64(credID), 10)+"/raw", nil)
	GetCredentialRaw(c)
	return w.Body.String(), w.Code
}

func updateCredentialViaHandler(t *testing.T, poolID, credID uint, body string) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(poolID), 10)}, {Key: "credId", Value: strconv.FormatUint(uint64(credID), 10)}}
	c.Request = httptest.NewRequest("PATCH", "/pools/"+strconv.FormatUint(uint64(poolID), 10)+"/credentials/"+strconv.FormatUint(uint64(credID), 10), strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateCredential(c)
	return w.Body.String(), w.Code
}

func deleteCredentialViaHandler(t *testing.T, poolID, credID uint) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(poolID), 10)}, {Key: "credId", Value: strconv.FormatUint(uint64(credID), 10)}}
	c.Request = httptest.NewRequest("DELETE", "/pools/"+strconv.FormatUint(uint64(poolID), 10)+"/credentials/"+strconv.FormatUint(uint64(credID), 10), nil)
	DeleteCredential(c)
	return w.Body.String(), w.Code
}

func createPoolAndGetID(t *testing.T, name string) uint {
	t.Helper()
	body := createPoolViaHandler(t, `{"name":"`+name+`"}`)
	id := gjson.Get(body, "data.ID").Uint()
	if id == 0 {
		t.Fatalf("create pool缺ID body=%s", body)
	}
	return uint(id)
}

// TestCreateCredential_Success_MaskNoLeak 覆盖：创建成功返回掩码且不泄漏明文/密文/KeyHash
func TestCreateCredential_Success_MaskNoLeak(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "创建掩码池")
	plain := "sk-test-create-1234567890AB12"

	body, _ := createCredentialViaHandler(t, poolID, `{"key":"`+plain+`","note":"首条"}`)
	if code := gjson.Get(body, "code").Int(); code != 200 {
		t.Fatalf("code=%d want 200 body=%s", code, body)
	}
	if !gjson.Get(body, "data.KeyMasked").Exists() {
		t.Fatalf("KeyMasked缺失 body=%s", body)
	}
	if gjson.Get(body, "data.Key").Exists() || gjson.Get(body, "data.key").Exists() || gjson.Get(body, "data.KeyHash").Exists() {
		t.Fatalf("不应返回Key/KeyHash body=%s", body)
	}
	expected := plain[:3] + "****" + plain[len(plain)-4:]
	if got := gjson.Get(body, "data.KeyMasked").String(); got != expected {
		t.Fatalf("KeyMasked=%q want %q body=%s", got, expected, body)
	}
	if strings.Contains(body, plain) {
		// 注意 body 含掩码片段，但不应含完整明文
		// 完整明文比对已排除掩码巧合：掩码含**** 与明文不同
		t.Fatalf("响应泄漏完整明文 body=%s", body)
	}
	// 密文不应出现在响应
	credID := gjson.Get(body, "data.ID").Uint()
	saved, err := repository.Default().Credential.Get(t.Context(), uint(credID))
	if err != nil {
		t.Fatalf("get saved: %v", err)
	}
	if saved.Key == plain {
		t.Fatal("库中Key应为密文")
	}
	if strings.Contains(body, saved.Key) {
		t.Fatalf("响应泄漏密文 body=%s", body)
	}
	if gjson.Get(body, "data.Note").String() != "首条" {
		t.Fatalf("Note回显不符 body=%s", body)
	}
	if gjson.Get(body, "data.Status").String() != models.CredentialStatusActive {
		t.Fatalf("默认Status应为active body=%s", body)
	}
}

// TestMaskKey_ShortKeyNoLeak 短 key（≤8）掩码必须完全退化为 ****，不泄漏任何明文片段。
func TestMaskKey_ShortKeyNoLeak(t *testing.T) {
	for _, s := range []string{"", "a", "ab12", "abcde", "abcdefgh", "abcdefghi"} {
		if s == "abcdefghi" {
			continue // 9 字符走标准掩码，见下方断言
		}
		if got := maskKey(s); got != "****" {
			t.Fatalf("maskKey(%q) = %q, want ****（不应泄漏明文）", s, got)
		}
	}
	if got := maskKey("abcdefghi"); got != "abc****fghi" {
		t.Fatalf("maskKey(abcdefghi) = %q, want abc****fghi", got)
	}
	if got := maskKey("sk-test-create-1234567890AB12"); !strings.Contains(got, "****") || strings.Contains(got, "sk-test-create-1234567890AB12") {
		t.Fatalf("长 key 掩码异常: %q", got)
	}
}

// TestCreateCredential_DedupWithinPool 覆盖：同池重复 KeyHash 400，跨池同明文允许
func TestCreateCredential_DedupWithinPool(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolA := createPoolAndGetID(t, "去重池A")
	poolB := createPoolAndGetID(t, "去重池B")
	plain := "sk-dedup-key-9999ABCD"

	body1, _ := createCredentialViaHandler(t, poolA, `{"key":"`+plain+`"}`)
	if gjson.Get(body1, "code").Int() != 200 {
		t.Fatalf("首次创建应成功 body=%s", body1)
	}
	body2, _ := createCredentialViaHandler(t, poolA, `{"key":"`+plain+`"}`)
	if code := gjson.Get(body2, "code").Int(); code != 400 {
		t.Fatalf("同池重复应400 got %d body=%s", code, body2)
	}
	// 跨池同明文允许
	body3, _ := createCredentialViaHandler(t, poolB, `{"key":"`+plain+`"}`)
	if gjson.Get(body3, "code").Int() != 200 {
		t.Fatalf("跨池同key应成功 body=%s", body3)
	}
}

// TestCreateCredential_Validation 覆盖：空key 400、pool不存在 404、cipher未配置500
func TestCreateCredential_Validation(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "校验池")

	body, _ := createCredentialViaHandler(t, poolID, `{"key":"   "}`)
	if gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("空key应400 body=%s", body)
	}
	body2, _ := createCredentialViaHandler(t, 9999, `{"key":"sk-x-ABCD"}`)
	if gjson.Get(body2, "code").Int() != 404 {
		t.Fatalf("不存在池应404 body=%s", body2)
	}
	// cipher 未配置分支
	credentialcrypto.SetDefault(nil)
	body3, _ := createCredentialViaHandler(t, poolID, `{"key":"sk-no-cipher-ABCD"}`)
	if code := gjson.Get(body3, "code").Int(); code != 500 {
		t.Fatalf("无cipher应500 got %d body=%s", code, body3)
	}
	// 恢复避免污染 t.Cleanup 已注册的恢复，这里重设
	setupCipherForCredTest(t)
}

// TestGetCredential_CrossPoolGuard 覆盖：单条获取成功、不存在404、越池404
func TestGetCredential_CrossPoolGuard(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolA := createPoolAndGetID(t, "查询池A")
	poolB := createPoolAndGetID(t, "查询池B")
	body, _ := createCredentialViaHandler(t, poolA, `{"key":"sk-get-1234ABCD"}`)
	credID := uint(gjson.Get(body, "data.ID").Uint())

	got, _ := getCredentialViaHandler(t, poolA, credID)
	if gjson.Get(got, "code").Int() != 200 {
		t.Fatalf("同池查询应200 body=%s", got)
	}
	if !gjson.Get(got, "data.KeyMasked").Exists() {
		t.Fatalf("KeyMasked缺失 body=%s", got)
	}
	// 越池
	got2, _ := getCredentialViaHandler(t, poolB, credID)
	if gjson.Get(got2, "code").Int() != 404 {
		t.Fatalf("越池应404 body=%s", got2)
	}
	// 不存在 cred
	got3, _ := getCredentialViaHandler(t, poolA, 9999)
	if gjson.Get(got3, "code").Int() != 404 {
		t.Fatalf("不存在cred应404 body=%s", got3)
	}
	// 不存在池
	got4, _ := getCredentialViaHandler(t, 9999, credID)
	if gjson.Get(got4, "code").Int() != 404 {
		t.Fatalf("不存在池应404 body=%s", got4)
	}
}

// TestGetCredentialRaw_Decrypt 覆盖：raw端点返回明文，越池404
func TestGetCredentialRaw_Decrypt(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "raw池")
	plain := "sk-raw-decrypt-ABCD1234"
	body, _ := createCredentialViaHandler(t, poolID, `{"key":"`+plain+`"}`)
	credID := uint(gjson.Get(body, "data.ID").Uint())

	rawBody, _ := getCredentialRawViaHandler(t, poolID, credID)
	if gjson.Get(rawBody, "code").Int() != 200 {
		t.Fatalf("raw应200 body=%s", rawBody)
	}
	if got := gjson.Get(rawBody, "data.key").String(); got != plain {
		t.Fatalf("raw key=%q want %q body=%s", got, plain, rawBody)
	}
	// 越池
	poolB := createPoolAndGetID(t, "raw池B")
	rawBody2, _ := getCredentialRawViaHandler(t, poolB, credID)
	if gjson.Get(rawBody2, "code").Int() != 404 {
		t.Fatalf("raw越池应404 body=%s", rawBody2)
	}
}

// TestUpdateCredential_NoteAndStatus 覆盖：note可清空、status合法切换、非法status 400、无字段400、越池404
func TestUpdateCredential_NoteAndStatus(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "更新池")
	body, _ := createCredentialViaHandler(t, poolID, `{"key":"sk-update-ABCD1234","note":"旧注记"}`)
	credID := uint(gjson.Get(body, "data.ID").Uint())

	// 清空 note
	body2, _ := updateCredentialViaHandler(t, poolID, credID, `{"note":""}`)
	if gjson.Get(body2, "code").Int() != 200 {
		t.Fatalf("清空note应200 body=%s", body2)
	}
	if got := gjson.Get(body2, "data.Note").String(); got != "" {
		t.Fatalf("清空后Note=%q want 空串 body=%s", got, body2)
	}

	// 切 status
	body3, _ := updateCredentialViaHandler(t, poolID, credID, `{"status":"disabled"}`)
	if gjson.Get(body3, "code").Int() != 200 {
		t.Fatalf("切disabled应200 body=%s", body3)
	}
	if got := gjson.Get(body3, "data.Status").String(); got != models.CredentialStatusDisabled {
		t.Fatalf("Status=%q want disabled", got)
	}

	// 非法 status
	body4, _ := updateCredentialViaHandler(t, poolID, credID, `{"status":"invalid"}`)
	if gjson.Get(body4, "code").Int() != 400 {
		t.Fatalf("非法status应400 body=%s", body4)
	}
	// 无字段
	body5, _ := updateCredentialViaHandler(t, poolID, credID, `{}`)
	if gjson.Get(body5, "code").Int() != 400 {
		t.Fatalf("无字段应400 body=%s", body5)
	}
	// 越池
	poolB := createPoolAndGetID(t, "更新池B")
	body6, _ := updateCredentialViaHandler(t, poolB, credID, `{"note":"越池"}`)
	if gjson.Get(body6, "code").Int() != 404 {
		t.Fatalf("越池更新应404 body=%s", body6)
	}
}

// TestDeleteCredential 覆盖：单删成功后再次查询404，越池404
func TestDeleteCredential(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolA := createPoolAndGetID(t, "删除池A")
	poolB := createPoolAndGetID(t, "删除池B")
	body, _ := createCredentialViaHandler(t, poolA, `{"key":"sk-del-ABCD1234"}`)
	credID := uint(gjson.Get(body, "data.ID").Uint())

	delBody, _ := deleteCredentialViaHandler(t, poolA, credID)
	if gjson.Get(delBody, "code").Int() != 200 {
		t.Fatalf("删除应200 body=%s", delBody)
	}
	got, _ := getCredentialViaHandler(t, poolA, credID)
	if gjson.Get(got, "code").Int() != 404 {
		t.Fatalf("删除后查询应404 body=%s", got)
	}
	// 越池删除应404且不删
	body2, _ := createCredentialViaHandler(t, poolA, `{"key":"sk-del2-ABCD1234"}`)
	credID2 := uint(gjson.Get(body2, "data.ID").Uint())
	delBody2, _ := deleteCredentialViaHandler(t, poolB, credID2)
	if gjson.Get(delBody2, "code").Int() != 404 {
		t.Fatalf("越池删除应404 body=%s", delBody2)
	}
	got2, _ := getCredentialViaHandler(t, poolA, credID2)
	if gjson.Get(got2, "code").Int() != 200 {
		t.Fatalf("越池误删 body=%s", got2)
	}
}
