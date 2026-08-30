package channel

import (
	"strings"
	"testing"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"gorm.io/gorm"
)

// testCredHexKey 32 字节 hex 测试密钥（与 handler/pools 测试同一常量值语义）。
const testCredHexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

// setupCipherForChannel 注入测试 cipher 并在测试结束恢复 nil（与 pools 测试同姿势）。
func setupCipherForChannel(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	c, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("credentialcrypto.New: %v", err)
	}
	credentialcrypto.SetDefault(c)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
	return c
}

// TestBuildConfig_EndpointURLOverrides 锁 AC-2 URL 继承链 + AC-5 config 组装：
// 端点 URL 非空时覆盖 base_url，未知字段（version/auth_type）原样保留，
// api_key 写入明文（不写掩码/密文）。
func TestBuildConfig_EndpointURLOverrides(t *testing.T) {
	cfg, upstreamURL, err := BuildConfig(
		models.Provider{Config: `{"base_url":"https://default.example/v1","version":"2023-06-01","auth_type":"x-api-key"}`},
		models.Endpoint{URL: "https://override.example/claude/v1"},
		"sk-plain-123",
	)
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
	}
	if upstreamURL != "https://override.example/claude/v1" {
		t.Fatalf("upstreamURL = %q, want override URL", upstreamURL)
	}
	for _, want := range []string{
		`"base_url":"https://override.example/claude/v1"`,
		`"api_key":"sk-plain-123"`,
		`"version":"2023-06-01"`,
		`"auth_type":"x-api-key"`,
	} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config %s 缺少 %s", cfg, want)
		}
	}
}

// TestBuildConfig_InheritBaseURL 锁 AC-2 继承链第二级：端点 URL 空 → 继承
// Provider.Config.base_url，输出 base_url 不变。
func TestBuildConfig_InheritBaseURL(t *testing.T) {
	cfg, upstreamURL, err := BuildConfig(
		models.Provider{Config: `{"base_url":"https://default.example/v1"}`},
		models.Endpoint{URL: ""},
		"sk-plain-123",
	)
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
	}
	if upstreamURL != "https://default.example/v1" {
		t.Fatalf("upstreamURL = %q, want default base_url", upstreamURL)
	}
	if !strings.Contains(cfg, `"base_url":"https://default.example/v1"`) {
		t.Fatalf("config %s 应保留原 base_url", cfg)
	}
}

// TestBuildConfig_NoURL 锁「端点 URL 与 base_url 皆空 → 报错」：
// BuildReq 会拼出残缺 URL，宁在上游之前失败。
func TestBuildConfig_NoURL(t *testing.T) {
	_, _, err := BuildConfig(models.Provider{Config: `{}`}, models.Endpoint{URL: ""}, "sk")
	if err == nil {
		t.Fatal("BuildConfig with no URL succeeded, want error")
	}
	if !strings.Contains(err.Error(), "no upstream URL") {
		t.Fatalf("err = %v, want no upstream URL 语义", err)
	}
}

// TestBuildConfig_AcceptableByProvidersNew 锁 AC-5 产物可被 providers.New 反序列化：
// 三个协议形状的 config 都应是合法 New 输入（openai / anthropic）。
func TestBuildConfig_AcceptableByProvidersNew(t *testing.T) {
	for _, tt := range []struct {
		name     string
		typ      string
		baseURL  string
		endpoint string
	}{
		{name: "openai", typ: "openai", baseURL: `{"base_url":"https://api.example/v1"}`, endpoint: ""},
		{name: "anthropic", typ: "anthropic", baseURL: `{"base_url":"https://api.example/v1","version":"2023-06-01","auth_type":"x-api-key"}`, endpoint: "https://api.example/claude/v1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _, err := BuildConfig(models.Provider{Type: tt.typ, Config: tt.baseURL}, models.Endpoint{URL: tt.endpoint}, "sk-plain")
			if err != nil {
				t.Fatalf("BuildConfig error: %v", err)
			}
			p, err := providers.New(tt.typ, cfg, "")
			if err != nil {
				t.Fatalf("providers.New(%q) error: %v", tt.typ, err)
			}
			if p == nil {
				t.Fatal("providers.New returned nil")
			}
		})
	}
}

// TestPlainCredentialKey 锁「解密回明文 + cipher 未初始化报错」。
func TestPlainCredentialKey(t *testing.T) {
	c := setupCipherForChannel(t)
	enc, err := c.Encrypt("sk-secret-456")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := plainCredentialKey(models.Credential{Key: enc})
	if err != nil {
		t.Fatalf("plainCredentialKey error: %v", err)
	}
	if got != "sk-secret-456" {
		t.Fatalf("plainCredentialKey = %q, want %q", got, "sk-secret-456")
	}
}

// TestPlainCredentialKey_NoCipher 锁「cipher 未初始化（nil）→ 明确报错」：
// 明文落库路径禁止的同一原则——明文写盘/无密可用都应显式失败。
func TestPlainCredentialKey_NoCipher(t *testing.T) {
	credentialcrypto.SetDefault(nil)
	_, err := plainCredentialKey(models.Credential{Key: "anything"})
	if err == nil {
		t.Fatal("plainCredentialKey with nil cipher succeeded, want error")
	}
}

// TestPlainCredentialKey_BadCiphertext 锁 AC-5「解密失败报错」：
// 密文被篡改（GCM 认证失败）或非 hex 形态都不得产出明文或静默吞错，
// 必须返回带上下文（凭据 ID）的错误。
func TestPlainCredentialKey_BadCiphertext(t *testing.T) {
	c := setupCipherForChannel(t)
	enc, err := c.Encrypt("sk-secret-456")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	// 篡改末尾一个 hex 字符（保持合法 hex，走 GCM 认证失败而非 hex 解码失败）
	tampered := enc[:len(enc)-1] + "0"
	if tampered == enc {
		tampered = enc[:len(enc)-1] + "1"
	}
	_, err = plainCredentialKey(models.Credential{Model: gorm.Model{ID: 42}, Key: tampered})
	if err == nil {
		t.Fatal("plainCredentialKey with tampered ciphertext succeeded, want error")
	}
	if !strings.Contains(err.Error(), "42") {
		t.Fatalf("err = %v, want 含凭据 ID 上下文", err)
	}
	// 非 hex 形态：同样报错（hex.DecodeString 失败路径）
	if _, err = plainCredentialKey(models.Credential{Key: "not-hex!"}); err == nil {
		t.Fatal("plainCredentialKey with non-hex key succeeded, want error")
	}
}

// TestBuildConfig_EmptyConfig 锁「Provider.Config 为空串 + 端点 URL 非空」：
// config 基底从零构建（无 nil map 赋值风险），输出只含 base_url 与 api_key，
// 即「只有端点覆盖 URL、无任何原配置」的最小合法形态。
func TestBuildConfig_EmptyConfig(t *testing.T) {
	cfg, upstreamURL, err := BuildConfig(
		models.Provider{}, // Config 空串
		models.Endpoint{URL: "https://override.example/v1"},
		"sk-plain",
	)
	if err != nil {
		t.Fatalf("BuildConfig error: %v", err)
	}
	if upstreamURL != "https://override.example/v1" {
		t.Fatalf("upstreamURL = %q, want endpoint URL", upstreamURL)
	}
	for _, want := range []string{`"base_url":"https://override.example/v1"`, `"api_key":"sk-plain"`} {
		if !strings.Contains(cfg, want) {
			t.Fatalf("config %s 缺少 %s", cfg, want)
		}
	}
}
