package credentialcrypto

import (
	"encoding/hex"
	"strings"
	"testing"
)

const testHexKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func newTestCipher(t *testing.T) *Cipher {
	t.Helper()
	c, err := New(testHexKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// TestNew_KeyValidation 锁定密钥长度/格式校验：非 32 字节或非 hex 必须报错，
// 防止弱密钥或误配的密钥静默降级。
func TestNew_KeyValidation(t *testing.T) {
	tests := []struct {
		name    string
		hexKey  string
		wantErr bool
	}{
		{name: "32 字节 hex 合法", hexKey: testHexKey, wantErr: false},
		{name: "16 字节（128 位）拒绝", hexKey: "00112233445566778899aabbccddeeff", wantErr: true},
		{name: "非 hex 字符拒绝", hexKey: "zz" + testHexKey[2:], wantErr: true},
		{name: "空串拒绝", hexKey: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.hexKey)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New(%q) err = %v, wantErr = %v", tt.hexKey, err, tt.wantErr)
			}
		})
	}
}

// TestCipher_EncryptDecrypt_RoundTrip 锁定加解密闭环与密文格式。
func TestCipher_EncryptDecrypt_RoundTrip(t *testing.T) {
	c := newTestCipher(t)
	plain := "sk-test-abcdef123456"

	enc, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == plain {
		t.Fatal("密文与明文相同，未加密")
	}
	if strings.Contains(enc, plain) {
		t.Fatal("密文包含明文字节序列")
	}
	// 格式：hex(nonce ‖ ciphertext)，AES-GCM nonce 12B，整体至少 2*12 hex 字符
	raw, err := hex.DecodeString(enc)
	if err != nil {
		t.Fatalf("密文不是合法 hex: %v", err)
	}
	if len(raw) < c.gcm.NonceSize()+1 {
		t.Fatalf("密文长度 %d，应 >= nonce(%d)+1", len(raw), c.gcm.NonceSize())
	}

	got, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != plain {
		t.Fatalf("解密结果 = %q, want %q", got, plain)
	}
}

// TestCipher_Encrypt_Nondeterministic 锁定每次加密产生不同密文（随机 nonce），
// 同明文两次加密不可比对（这也是 KeyHash 单独存在的理由）。
func TestCipher_Encrypt_Nondeterministic(t *testing.T) {
	c := newTestCipher(t)
	a, err := c.Encrypt("sk-x")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	b, err := c.Encrypt("sk-x")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if a == b {
		t.Fatal("同明文两次加密密文相同，nonce 未随机化")
	}
}

// TestCipher_Decrypt_WrongKey 锁定换密钥后旧密文不可解（轮换即失效的语义）。
func TestCipher_Decrypt_WrongKey(t *testing.T) {
	c := newTestCipher(t)
	enc, err := c.Encrypt("sk-sensitive")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	other, err := New(strings.Repeat("ff", 32))
	if err != nil {
		t.Fatalf("New other key: %v", err)
	}
	if _, err := other.Decrypt(enc); err == nil {
		t.Fatal("错误密钥解密应失败")
	}
}

// TestCipher_Decrypt_Tampered 锁定 GCM 完整性校验：篡改密文任何字节都解密失败。
func TestCipher_Decrypt_Tampered(t *testing.T) {
	c := newTestCipher(t)
	enc, err := c.Encrypt("sk-tamper-me")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	raw, _ := hex.DecodeString(enc)
	raw[len(raw)-1] ^= 0x01 // 翻转最后一个字节
	tampered := hex.EncodeToString(raw)
	if _, err := c.Decrypt(tampered); err == nil {
		t.Fatal("篡改密文解密应失败")
	}
}

// TestCipher_Hash 锁定 KeyHash 语义：同明文等值、异明文不等、空串为空。
func TestCipher_Hash(t *testing.T) {
	c := newTestCipher(t)
	h1 := c.Hash("sk-abc")
	h2 := c.Hash("sk-abc")
	h3 := c.Hash("sk-abd")
	if h1 != h2 {
		t.Fatal("同明文哈希应相等")
	}
	if h1 == h3 {
		t.Fatal("异明文哈希应不同")
	}
	if len(h1) != 64 {
		t.Fatalf("SHA-256 hex 长度 = %d, want 64", len(h1))
	}
	if c.Hash("") != "" {
		t.Fatal("空明文哈希应为空串")
	}
}
