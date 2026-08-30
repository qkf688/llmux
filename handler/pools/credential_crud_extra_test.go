package pools

import (
	"strconv"
	"strings"
	"testing"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/tidwall/gjson"
)

// 凭据端点补充边界覆盖：raw 解密失败/无 cipher 均 500、创建与重新 GET 字段一致性、批量幂等 RowsAffected 语义。

func TestGetCredentialRaw_DecryptFailure_500(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "raw损坏池")
	body, _ := createCredentialViaHandler(t, poolID, `{"key":"sk-tamper-ABCD1234"}`)
	credID := uint(gjson.Get(body, "data.ID").Uint())

	// 篡改库中密文：违反 GCM 认证即解密失败
	saved, err := repos().Credential.Get(t.Context(), credID)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := "aa" + saved.Key[2:]
	if _, err := repos().Credential.UpdateFields(t.Context(), credID, map[string]any{"key": corrupted}); err != nil {
		t.Fatal(err)
	}

	rawBody, rawCode := getCredentialRawViaHandler(t, poolID, credID)
	if gjson.Get(rawBody, "code").Int() != 500 {
		t.Fatalf("解密失败应 body-code=500 got %d body=%s", gjson.Get(rawBody, "code").Int(), rawBody)
	}
	if rawCode != 500 {
		t.Fatalf("解密失败 HTTP 应 500 got %d", rawCode)
	}
	if strings.Contains(rawBody, "sk-tamper") {
		t.Fatalf("解密失败响应不应泄漏明文 body=%s", rawBody)
	}
}

func TestGetCredentialRaw_NoCipher_500(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "raw无cipher池")
	body, _ := createCredentialViaHandler(t, poolID, `{"key":"sk-nocipher-ABCD1234"}`)
	credID := uint(gjson.Get(body, "data.ID").Uint())

	credentialcrypto.SetDefault(nil)
	defer setupCipherForCredTest(t)
	rawBody, rawCode := getCredentialRawViaHandler(t, poolID, credID)
	if gjson.Get(rawBody, "code").Int() != 500 {
		t.Fatalf("无cipher应 body-code=500 got %d body=%s", gjson.Get(rawBody, "code").Int(), rawBody)
	}
	if rawCode != 500 {
		t.Fatalf("无cipher HTTP 应 500 got %d", rawCode)
	}
}

// TestCreateThenGet_FieldIntegrity 创建响应与随后重新 GET 的响应字段应完全一致。
func TestCreateThenGet_FieldIntegrity(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "字段完整池")
	plain := "sk-integrity-ABCD1234"
	createBody, _ := createCredentialViaHandler(t, poolID, `{"key":"`+plain+`","note":"完整性"}`)
	credID := uint(gjson.Get(createBody, "data.ID").Uint())

	getBody, _ := getCredentialViaHandler(t, poolID, credID)

	keys := []string{
		"ID", "PoolID", "GroupID", "Status", "Note", "KeyMasked",
		"CooldownUntil", "CooldownReason", "FailCount", "LastUsedAt",
		"TotalRequests", "TotalErrors", "TotalTokens", "CreatedAt", "UpdatedAt",
	}
	for _, k := range keys {
		createV := gjson.Get(createBody, "data."+k).Raw
		getV := gjson.Get(getBody, "data."+k).Raw
		if createV != getV {
			t.Fatalf("字段 %s 创建响应 %s != 重新GET响应 %s\n createBody=%s\n getBody=%s", k, createV, getV, createBody, getBody)
		}
	}
	if gjson.Get(createBody, "data.KeyMasked").String() != maskKey(plain) {
		t.Fatalf("KeyMasked=%q want %q", gjson.Get(createBody, "data.KeyMasked").String(), maskKey(plain))
	}
}

// TestBatchUpdate_StatusIdempotentRowsAffected 记录 SQLite 下已同状态重复更新的 RowsAffected 语义。
func TestBatchUpdate_StatusIdempotentRowsAffected(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	poolID := createPoolAndGetID(t, "幂等语义池")
	ids := createN(t, poolID, 3, "idem")
	// 先把 ids[0] 单条置 disabled
	body, _ := updateCredentialViaHandler(t, poolID, ids[0], `{"status":"disabled"}`)
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("前置置disabled失败 body=%s", body)
	}
	// 批量三条全部置 disabled（idempotent：其中一条已同状态）
	batchBody, _ := batchUpdateViaHandler(t, poolID, `{"ids":[`+idsJoin(ids)+`],"status":"disabled"}`)
	if gjson.Get(batchBody, "code").Int() != 200 {
		t.Fatalf("批量应200 body=%s", batchBody)
	}
	t.Logf("已同状态重复批量置disabled：updated=%d（SQLite RowsAffected 语义）body=%s",
		gjson.Get(batchBody, "data.updated").Int(), batchBody)
	// 期望语义：池内命中行数全部计入（SQLite changes() 对匹配行全部计数）
	if got := gjson.Get(batchBody, "data.updated").Int(); got != 3 {
		t.Fatalf("SQLite 幂等更新应 updated=3（命中计数）got %d body=%s", got, batchBody)
	}
}

func idsJoin(ids []uint) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
	}
	return strings.Join(parts, ",")
}
