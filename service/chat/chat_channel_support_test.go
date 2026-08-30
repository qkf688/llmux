package chat

import (
	"testing"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

// setupChatChannelCipher 注入凭据加密 cipher（选路需要解密明文 key）。
// 与 service/channel 测试同姿势：SetDefault + t.Cleanup 复位，测试间不残留。
// ⚠️ 本包测试基于 credentialcrypto 包级全局注入，**禁止为 chat 包测试加 t.Parallel()**
// （多个并行用例会交叉污染同一全局 cipher/repository 默认实例）。
func setupChatChannelCipher(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	cipher, err := credentialcrypto.New(testChatCredHexKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	credentialcrypto.SetDefault(cipher)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
	return cipher
}

// mustEncrypt 加密测试凭据 key（helper，避免每个用例重复 error 样板）。
// 接口参数保持最小面：只需 Encrypt，解密/哈希由调用方直接用 *credentialcrypto.Cipher。
func mustEncrypt(t *testing.T, cipher interface {
	Encrypt(plain string) (string, error)
}, plain string) string {
	t.Helper()
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt %q: %v", plain, err)
	}
	return enc
}

// addLegacyChannelRows 给 provider 补「S1 迁移后老 Provider 等价形态」的选路数据：
// 1 端点（协议=ProtocolOfType(type)、URL 空继承 Provider.Config.base_url）
// + 1 分组 + 1 内联凭据（明文 key 经 cipher 加密落库）。
// S3-2 起凭据只存在加密的 credentials 表，直连 Config.api_key 的旧测试数据必须补行，
// 否则选路在端点/凭据层直接失败。
func addLegacyChannelRows(t *testing.T, cipher *credentialcrypto.Cipher, provider models.Provider) {
	t.Helper()
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOfType(provider.Type)), Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint for provider %q: %v", provider.Name, err)
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group for provider %q: %v", provider.Name, err)
	}
	plain := "test-key" // 任意占位明文：KeyHash 必须与之配对（Hash 加密散发），
	// 导入不同明文时同时改两处，否则选路解密出的 key 与 KeyHash 断言不一致。
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt credential for provider %q: %v", provider.Name, err)
	}
	cred := models.Credential{GroupID: &grp.ID, Key: enc, KeyHash: cipher.Hash(plain)}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential for provider %q: %v", provider.Name, err)
	}
}
