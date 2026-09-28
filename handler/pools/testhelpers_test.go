package pools

import (
	"testing"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// 本文件承载号池测试的共享夹具（cipher 注入 + 建池）：号池域只剩池 CRUD、批量导入、
// 按凭据测试三类测试消费它们，凭据自身的 CRUD/启停用例已随接口迁到 handler/credentials。

const testCredHexKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// setupCipherForCredTest 注入测试用 cipher 并在用例结束复位默认值。
// 掩码/解密类断言必须打在真实密文上——cipher 未配置时 MaskStored 会退化为 `****`，
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

// seedCredentialInPool 直接写库往号池里放一条 active 凭据（明文进、密文落），返回凭据 ID。
// 供「按凭据测试」与「批量导入去重」用例预置池内 key——凭据的新增入口已迁到
// handler/credentials，号池域的测试不再经 HTTP 造凭据。
func seedCredentialInPool(t *testing.T, poolID uint, plain string) uint {
	t.Helper()
	if credentialcrypto.Default() == nil {
		setupCipherForCredTest(t)
	}
	cipher := credentialcrypto.Default()
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt seed key: %v", err)
	}
	cred := &models.Credential{
		Key:     enc,
		KeyHash: cipher.Hash(plain),
		PoolID:  &poolID,
		Status:  models.CredentialStatusActive,
	}
	if err := repository.Default().Credential.Create(t.Context(), cred); err != nil {
		t.Fatalf("seed credential in pool: %v", err)
	}
	return cred.ID
}

// createPoolAndGetID 走 CreatePool handler 建池并返回 ID。
func createPoolAndGetID(t *testing.T, name string) uint {
	t.Helper()
	body := createPoolViaHandler(t, `{"name":"`+name+`"}`)
	id := gjson.Get(body, "data.ID").Uint()
	if id == 0 {
		t.Fatalf("create pool缺ID body=%s", body)
	}
	return uint(id)
}
