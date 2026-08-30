package pools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func batchImportViaHandler(t *testing.T, poolID uint, body string) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(poolID), 10)}}
	c.Request = httptest.NewRequest("POST", "/pools/"+strconv.FormatUint(uint64(poolID), 10)+"/credentials/batch/import", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	BatchImportCredentials(c)
	return w.Body.String(), w.Code
}

// importKeysBody 用 json.Marshal 构造请求体：key 内含 \r\n 等字符时正确转义，
// 避免手拼 JSON 把控制字符裸写进请求体。
func importKeysBody(keys ...string) string {
	b, err := json.Marshal(map[string]any{"keys": keys})
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestBatchImport_AllCreated_MaskedEcho 覆盖 AC-1：全量新 key 加密落库，
// 逐行掩码回显，不泄漏明文/密文/KeyHash，库内 KeyHash 可复算验证。
func TestBatchImport_AllCreated_MaskedEcho(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "导入池")
	keys := []string{"sk-import-1-ABCD1234", "sk-import-2-ABCD1234", "sk-import-3-ABCD1234"}

	body, code := batchImportViaHandler(t, poolID, importKeysBody(keys...))
	if code != 200 {
		t.Fatalf("HTTP %d body=%s", code, body)
	}
	if gjson.Get(body, "data.total").Int() != 3 || gjson.Get(body, "data.imported").Int() != 3 ||
		gjson.Get(body, "data.skipped").Int() != 0 || gjson.Get(body, "data.failed").Int() != 0 {
		t.Fatalf("汇总计数不符 body=%s", body)
	}
	if got := gjson.Get(body, "data.rows.#").Int(); got != 3 {
		t.Fatalf("rows 数=%d want 3 body=%s", got, body)
	}
	for i, k := range keys {
		path := fmt.Sprintf("data.rows.#(Index==%d)", i)
		if got := gjson.Get(body, path+".Status").String(); got != importStatusImported {
			t.Fatalf("row[%d].Status=%q want imported body=%s", i, got, body)
		}
		if got := gjson.Get(body, path+".Key").String(); got != maskKey(k) {
			t.Fatalf("row[%d].Key=%q want %q body=%s", i, got, maskKey(k), body)
		}
		if got := gjson.Get(body, path+".Reason").Exists(); got {
			t.Fatalf("imported 行不应有 Reason body=%s", body)
		}
		if strings.Contains(body, k) {
			t.Fatalf("响应泄漏完整明文 %q body=%s", k, body)
		}
	}
	if gjson.Get(body, "data.Key").Exists() || gjson.Get(body, "data.key").Exists() || gjson.Get(body, "data.KeyHash").Exists() {
		t.Fatalf("不应返回 Key/KeyHash 字段 body=%s", body)
	}

	// 库内形态：密文 + KeyHash 与明文 SHA-256 一致、可解密回明文
	cipher := credentialcrypto.Default()
	creds, err := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: &poolID})
	if err != nil {
		t.Fatalf("list saved: %v", err)
	}
	if len(creds) != 3 {
		t.Fatalf("库内条数=%d want 3", len(creds))
	}
	for _, k := range keys {
		var found bool
		for _, c := range creds {
			if c.KeyHash == cipher.Hash(k) {
				found = true
				if c.Key == k {
					t.Fatal("库中 Key 应为密文")
				}
				plain, err := cipher.Decrypt(c.Key)
				if err != nil {
					t.Fatalf("解密失败 %q: %v", c.Key, err)
				}
				if plain != k {
					t.Fatalf("解密回明文=%q want %q", plain, k)
				}
			}
		}
		if !found {
			t.Fatalf("库内缺 %q 的 KeyHash", k)
		}
	}
}

// TestBatchImport_InBatchDuplicate 覆盖 AC-2：批内重复首现生效，后续行
// skipped + reason=duplicate_in_batch，库内仅 1 条。
func TestBatchImport_InBatchDuplicate(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "批内去重池")
	dup := "sk-batch-dup-ABCD1234"

	body, _ := batchImportViaHandler(t, poolID, importKeysBody("sk-b1-ABCD1234", dup, dup))
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("code!=200 body=%s", body)
	}
	if gjson.Get(body, "data.imported").Int() != 2 || gjson.Get(body, "data.skipped").Int() != 1 {
		t.Fatalf("计数不符 body=%s", body)
	}
	if got := gjson.Get(body, "data.rows.#(Index==1).Status").String(); got != importStatusImported {
		t.Fatalf("首现应 imported got %q body=%s", got, body)
	}
	if got := gjson.Get(body, "data.rows.#(Index==2).Status").String(); got != importStatusSkipped {
		t.Fatalf("重复行应 skipped got %q body=%s", got, body)
	}
	if got := gjson.Get(body, "data.rows.#(Index==2).Reason").String(); got != importReasonDuplicateInBatch {
		t.Fatalf("重复行 Reason=%q want duplicate_in_batch body=%s", got, body)
	}
	creds, err := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: &poolID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("库内条数=%d want 2 body=%s", len(creds), body)
	}
}

// TestBatchImport_InPoolDuplicate 覆盖 AC-3：池内已存在 key 导入 → skipped +
// reason=duplicate_in_pool，不落库、不覆盖原行（与批内重复 reason 可区分）。
func TestBatchImport_InPoolDuplicate(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "池内去重池")
	existing := "sk-pool-dup-ABCD1234"
	body0, _ := createCredentialViaHandler(t, poolID, `{"key":"`+existing+`"}`)
	origID := gjson.Get(body0, "data.ID").Uint()

	body, _ := batchImportViaHandler(t, poolID, importKeysBody(existing, "sk-pool-new-ABCD1234"))
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("code!=200 body=%s", body)
	}
	if gjson.Get(body, "data.imported").Int() != 1 || gjson.Get(body, "data.skipped").Int() != 1 {
		t.Fatalf("计数不符 body=%s", body)
	}
	if got := gjson.Get(body, "data.rows.#(Index==0).Status").String(); got != importStatusSkipped {
		t.Fatalf("池内重复应 skipped got %q body=%s", got, body)
	}
	if got := gjson.Get(body, "data.rows.#(Index==0).Reason").String(); got != importReasonDuplicateInPool {
		t.Fatalf("池内重复 Reason=%q want duplicate_in_pool body=%s", got, body)
	}
	creds, err := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: &poolID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("库内条数=%d want 2（原行应保留、新行落库）body=%s", len(creds), body)
	}
	orig, err := repository.Default().Credential.Get(t.Context(), uint(origID))
	if err != nil {
		t.Fatalf("get orig: %v", err)
	}
	if orig.Status != "active" {
		t.Fatalf("原行被改动 Status=%q body=%s", orig.Status, body)
	}
}

// TestBatchImport_Mixed 覆盖 AC-2/3/4 叠加：新 key + 批内重复 + 池内重复 +
// 空行 + 纯空白行 + 首尾空白 key 混合批次，逐行状态与 reason 精确回显。
func TestBatchImport_Mixed(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "混合池")
	poolDup := "sk-mix-pool-ABCD1234"
	if body0, code := createCredentialViaHandler(t, poolID, `{"key":"`+poolDup+`"}`); code != 200 {
		t.Fatalf("预置池内 key 失败 body=%s", body0)
	}
	keys := []string{
		"sk-mix-new1-ABCD1234",     // 0 imported
		poolDup,                    // 1 skipped duplicate_in_pool
		"sk-mix-new1-ABCD1234",     // 2 skipped duplicate_in_batch（与 0 批内重复）
		"",                         // 3 skipped empty
		"   \t  ",                  // 4 skipped empty（纯空白）
		"  sk-mix-new2-ABCD1234  ", // 5 imported（trim 后落库）
		poolDup,                    // 6 skipped duplicate_in_batch（锁定判定顺序：批内先于池内——同 key 首现行报 pool、重复行报 batch）
	}

	body, _ := batchImportViaHandler(t, poolID, importKeysBody(keys...))
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("code!=200 body=%s", body)
	}
	if gjson.Get(body, "data.total").Int() != 7 || gjson.Get(body, "data.imported").Int() != 2 ||
		gjson.Get(body, "data.skipped").Int() != 5 || gjson.Get(body, "data.failed").Int() != 0 {
		t.Fatalf("汇总计数不符 body=%s", body)
	}
	want := []struct {
		status, reason string
	}{
		{importStatusImported, ""},
		{importStatusSkipped, importReasonDuplicateInPool},
		{importStatusSkipped, importReasonDuplicateInBatch},
		{importStatusSkipped, importReasonEmpty},
		{importStatusSkipped, importReasonEmpty},
		{importStatusImported, ""},
		{importStatusSkipped, importReasonDuplicateInBatch},
	}
	for i, w := range want {
		path := fmt.Sprintf("data.rows.#(Index==%d)", i)
		if got := gjson.Get(body, path+".Status").String(); got != w.status {
			t.Fatalf("row[%d].Status=%q want %q body=%s", i, got, w.status, body)
		}
		if got := gjson.Get(body, path+".Reason").String(); got != w.reason {
			t.Fatalf("row[%d].Reason=%q want %q body=%s", i, got, w.reason, body)
		}
	}
	if got := gjson.Get(body, "data.rows.#(Index==3).Key").String(); got != "" {
		t.Fatalf("空行 Key 应回空串 got %q body=%s", got, body)
	}
	creds, err := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: &poolID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(creds) != 3 {
		t.Fatalf("库内条数=%d want 3（预置1+新2）", len(creds))
	}
}

// TestBatchImport_TrimAndCRLF 覆盖 AC-1 的 trim 语义：首尾空白与 CRLF 粘贴
// 残留（\r\n）不改变哈希与落库，Windows 粘贴场景无需前端预处理。
func TestBatchImport_TrimAndCRLF(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "trim池")

	body, _ := batchImportViaHandler(t, poolID, importKeysBody("  sk-trim-1-ABCD1234  ", "sk-trim2-ABCD1234\r\n", "\tsk-trim-3-ABCD1234\t"))
	if gjson.Get(body, "code").Int() != 200 || gjson.Get(body, "data.imported").Int() != 3 {
		t.Fatalf("trim 导入应全成功 body=%s", body)
	}
	cipher := credentialcrypto.Default()
	creds, _ := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: &poolID})
	if len(creds) != 3 {
		t.Fatalf("库内条数=%d want 3", len(creds))
	}
	for _, plain := range []string{"sk-trim-1-ABCD1234", "sk-trim2-ABCD1234", "sk-trim-3-ABCD1234"} {
		var hit bool
		for _, c := range creds {
			if c.KeyHash == cipher.Hash(plain) {
				hit = true
			}
		}
		if !hit {
			t.Fatalf("trim 后哈希未落库: %q", plain)
		}
	}
}

// TestBatchImport_Validation 覆盖 AC-6：空 keys/缺 keys 400、超 500 上限 400 且零落库、
// 池不存在 404、cipher 未配置 500。
func TestBatchImport_Validation(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "校验池")

	// 空数组 / 缺 keys 字段
	for name, payload := range map[string]string{"空数组": `{"keys":[]}`, "缺字段": `{}`, "null": `{"keys":null}`} {
		body, _ := batchImportViaHandler(t, poolID, payload)
		if gjson.Get(body, "code").Int() != 400 {
			t.Fatalf("%s 应400 body=%s", name, body)
		}
	}

	// 超上限 501 条：400 且零落库
	tooMany := make([]string, maxCredentialImportBatch+1)
	for i := range tooMany {
		tooMany[i] = "sk-oversize-" + strconv.Itoa(i) + "-ABCD1234"
	}
	body, _ := batchImportViaHandler(t, poolID, importKeysBody(tooMany...))
	if gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("超限应400 body=%s", body)
	}
	creds, _ := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: &poolID})
	if len(creds) != 0 {
		t.Fatalf("超限请求不应落库, 库内=%d", len(creds))
	}

	// 池不存在 404
	body2, _ := batchImportViaHandler(t, 9999, importKeysBody("sk-nopool-ABCD1234"))
	if gjson.Get(body2, "code").Int() != 404 {
		t.Fatalf("不存在池应404 body=%s", body2)
	}

	// 非数组类型绑定失败 400（httpx 路径）
	body3, _ := batchImportViaHandler(t, poolID, `{"keys":"sk-not-array-ABCD1234"}`)
	if gjson.Get(body3, "code").Int() != 400 {
		t.Fatalf("keys 非数组应400 body=%s", body3)
	}

	// cipher 未配置 500（恢复原 cipher，防污染后续测试的隐式顺序依赖）
	prev := credentialcrypto.Default()
	credentialcrypto.SetDefault(nil)
	t.Cleanup(func() { credentialcrypto.SetDefault(prev) })
	body4, _ := batchImportViaHandler(t, poolID, importKeysBody("sk-nocipher-ABCD1234"))
	if gjson.Get(body4, "code").Int() != 500 {
		t.Fatalf("无 cipher 应500 body=%s", body4)
	}
}

// TestBatchImport_LimitBoundary500 覆盖 AC-6 边界：恰好 500 条全部导入成功
// （顺带压 ExistingHashes 的 IN 500 参数与循环写入）。
func TestBatchImport_LimitBoundary500(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "边界池")
	keys := make([]string, maxCredentialImportBatch)
	for i := range keys {
		keys[i] = "sk-limit-" + strconv.Itoa(i) + "-ABCD1234"
	}

	body, _ := batchImportViaHandler(t, poolID, importKeysBody(keys...))
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("500 条应200 body=%s", body)
	}
	if gjson.Get(body, "data.imported").Int() != maxCredentialImportBatch {
		t.Fatalf("imported=%d want %d body=%s", gjson.Get(body, "data.imported").Int(), maxCredentialImportBatch, body)
	}
	creds, _ := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: &poolID})
	if len(creds) != maxCredentialImportBatch {
		t.Fatalf("库内条数=%d want %d", len(creds), maxCredentialImportBatch)
	}
}

// failingEncryptCipher 包一层真实 Cipher，仅 Encrypt 必失败：
// 真实 Cipher 的 Encrypt 无业务可控失败点，以此覆盖 failed 行类契约。
type failingEncryptCipher struct {
	real *credentialcrypto.Cipher
}

func (f failingEncryptCipher) Hash(plain string) string { return f.real.Hash(plain) }
func (f failingEncryptCipher) Encrypt(string) (string, error) {
	return "", errors.New("encrypt boom")
}

// TestImportCredentialRows_EncryptFailure 覆盖 AC-5（加密异常）：单行加密失败
// 收集为 failed 行并继续处理其余行（行级事务，局部失败不回滚全部）；
// importCredentialRows 输出经 json.Marshal 断言响应体形状（与 c.JSON 共用 encoding/json）。
func TestImportCredentialRows_EncryptFailure(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "失败池")
	realCipher, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	data, err := importCredentialRows(t.Context(), repository.Default().Credential, poolID, []string{"sk-fail-1-ABCD1234", "sk-fail-2-ABCD1234"}, failingEncryptCipher{real: realCipher})
	if err != nil {
		t.Fatalf("importCredentialRows: %v", err)
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(b)
	if gjson.Get(body, "total").Int() != 2 || gjson.Get(body, "imported").Int() != 0 ||
		gjson.Get(body, "skipped").Int() != 0 || gjson.Get(body, "failed").Int() != 2 {
		t.Fatalf("汇总不符 body=%s", body)
	}
	if got := gjson.Get(body, "rows.#").Int(); got != 2 {
		t.Fatalf("rows=%d want 2 body=%s", got, body)
	}
	for i := 0; i < 2; i++ {
		path := fmt.Sprintf("rows.#(Index==%d)", i)
		if got := gjson.Get(body, path+".Status").String(); got != importStatusFailed {
			t.Fatalf("row[%d].Status=%q want failed body=%s", i, got, body)
		}
		if got := gjson.Get(body, path+".Reason").String(); got != importReasonEncryptFailed {
			t.Fatalf("row[%d].Reason=%q want encrypt_failed body=%s", i, got, body)
		}
	}
	creds, _ := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: &poolID})
	if len(creds) != 0 {
		t.Fatalf("全部加密失败不应落库, 库内=%d", len(creds))
	}
}

// failingHashesRepo 包一层真实 CredentialRepo，仅 ExistingHashes 必失败：
// 锁「查重查询异常 → 整体 error（handler 500），不做逐行兜底」的决策。
type failingHashesRepo struct {
	repository.CredentialRepo
}

func (f failingHashesRepo) ExistingHashes(context.Context, uint, []string) (map[string]bool, error) {
	return nil, errors.New("hashes boom")
}

// TestImportCredentialRows_QueryFailure 覆盖查重查询失败的整体拒绝路径。
func TestImportCredentialRows_QueryFailure(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	_, err := importCredentialRows(t.Context(), failingHashesRepo{repository.Default().Credential}, 1, []string{"sk-query-fail-ABCD1234"}, credentialcrypto.Default())
	if err == nil {
		t.Fatal("查重失败应返回 error")
	}
	if !strings.Contains(err.Error(), "existing hashes") {
		t.Fatalf("错误应带上下文: %v", err)
	}
}

// selectiveFailCipher 仅对指定明文 Encrypt 失败的 cipher：为「失败行不阻断
// 成功行落库」的正向证据提供混合注入（真实 cipher 无按 key 失败点）。
type selectiveFailCipher struct {
	real     *credentialcrypto.Cipher
	failKeys map[string]bool
}

func (s selectiveFailCipher) Hash(plain string) string { return s.real.Hash(plain) }
func (s selectiveFailCipher) Encrypt(plain string) (string, error) {
	if s.failKeys[plain] {
		return "", errors.New("encrypt boom")
	}
	return s.real.Encrypt(plain)
}

// TestImportCredentialRows_FailedRowsDoNotBlockOthers 覆盖 AC-5 正向证据
// （行级失败隔离）：单行加密失败 → 该行 failed + encrypt_failed 回显，
// 其余行照常加密落库，库内只剩成功行。
func TestImportCredentialRows_FailedRowsDoNotBlockOthers(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "隔离池")
	realCipher, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	okKey, failKey := "sk-isolate-ok-ABCD1234", "sk-isolate-fail-ABCD1234"

	data, err := importCredentialRows(t.Context(), repository.Default().Credential, poolID,
		[]string{failKey, okKey},
		selectiveFailCipher{real: realCipher, failKeys: map[string]bool{failKey: true}})
	if err != nil {
		t.Fatalf("importCredentialRows: %v", err)
	}
	b, _ := json.Marshal(data)
	body := string(b)
	if gjson.Get(body, "imported").Int() != 1 || gjson.Get(body, "failed").Int() != 1 {
		t.Fatalf("计数不符 body=%s", body)
	}
	if got := gjson.Get(body, "rows.#(Index==0).Status").String(); got != importStatusFailed {
		t.Fatalf("row0.Status=%q want failed body=%s", got, body)
	}
	if got := gjson.Get(body, "rows.#(Index==0).Reason").String(); got != importReasonEncryptFailed {
		t.Fatalf("row0.Reason=%q want encrypt_failed body=%s", got, body)
	}
	if got := gjson.Get(body, "rows.#(Index==1).Status").String(); got != importStatusImported {
		t.Fatalf("row1.Status=%q want imported body=%s", got, body)
	}
	creds, _ := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: &poolID})
	if len(creds) != 1 {
		t.Fatalf("库内条数=%d want 1（失败行不阻断成功行落库）", len(creds))
	}
	if creds[0].KeyHash != realCipher.Hash(okKey) {
		t.Fatalf("落库的应是成功行 %q", okKey)
	}
}

// failingCreateRepo 包一层真实 CredentialRepo，仅 Create 必失败：
// 锁「落库异常 → failed + reason=db_failed，其余行继续处理」的契约分支
// （真实 SQLite 下 Create 几乎不会失败，只能注入）。
type failingCreateRepo struct {
	repository.CredentialRepo
}

func (f failingCreateRepo) Create(context.Context, *models.Credential) error {
	return errors.New("create boom")
}

// TestImportCredentialRows_CreateFailure 覆盖 AC-5 落库异常分支：全部行
// Create 失败 → 逐行 failed + reason=db_failed，不中断回显（行级失败隔离）。
func TestImportCredentialRows_CreateFailure(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "落库失败池")

	data, err := importCredentialRows(t.Context(), failingCreateRepo{repository.Default().Credential}, poolID,
		[]string{"sk-dbfail-1-ABCD1234", "sk-dbfail-2-ABCD1234"}, credentialcrypto.Default())
	if err != nil {
		t.Fatalf("importCredentialRows: %v", err)
	}
	b, _ := json.Marshal(data)
	body := string(b)
	if gjson.Get(body, "imported").Int() != 0 || gjson.Get(body, "failed").Int() != 2 {
		t.Fatalf("计数不符 body=%s", body)
	}
	if got := gjson.Get(body, "rows.#(Index==1).Status").String(); got != importStatusFailed {
		t.Fatalf("row1.Status=%q want failed body=%s", got, body)
	}
	if got := gjson.Get(body, "rows.#(Index==1).Reason").String(); got != importReasonDBFailed {
		t.Fatalf("row1.Reason=%q want db_failed body=%s", got, body)
	}
}
