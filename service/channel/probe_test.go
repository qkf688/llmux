package channel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// TestDropCustomModels 锁剥离口径：只删 custom_models，其余键（含 anthropic 的
// version/auth_type 与遗留死键）原样保留。
func TestDropCustomModels(t *testing.T) {
	t.Run("命中键则剥离并保留其余", func(t *testing.T) {
		got, err := DropCustomModels(`{"api_key":"k","custom_models":["c1"],"upstream_models":["u1"],"base_url":"https://example.com"}`)
		if err != nil {
			t.Fatalf("DropCustomModels: %v", err)
		}
		if strings.Contains(got, "custom_models") {
			t.Fatalf("custom_models 未剥离: %s", got)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(got), &parsed); err != nil {
			t.Fatalf("结果非合法 JSON: %v", err)
		}
		if parsed["api_key"] != "k" || parsed["base_url"] != "https://example.com" {
			t.Fatalf("其余键被改写: %s", got)
		}
		if _, ok := parsed["upstream_models"]; !ok {
			t.Fatalf("遗留死键应原样保留（由 models 启动迁移统一清理）: %s", got)
		}
	})

	// 无该键 / 空 config 必须**原样返回**：不重跑 marshal 就不会打乱键序，
	// 让「剥与不剥」在字节层面可预期（探活与同步的错误文案、日志据此稳定）。
	t.Run("无该键原样返回", func(t *testing.T) {
		const in = `{"base_url":"https://example.com","api_key":"k"}`
		got, err := DropCustomModels(in)
		if err != nil {
			t.Fatalf("DropCustomModels: %v", err)
		}
		if got != in {
			t.Fatalf("got %s, want 原样 %s", got, in)
		}
	})

	t.Run("空 config 原样返回", func(t *testing.T) {
		got, err := DropCustomModels("")
		if err != nil {
			t.Fatalf("空 config 不应报错: %v", err)
		}
		if got != "" {
			t.Fatalf("got %q, want 空串", got)
		}
	})

	t.Run("非法 JSON 报错", func(t *testing.T) {
		if _, err := DropCustomModels("{not-json"); err == nil {
			t.Fatal("want error for malformed config")
		}
	})
}

// modelsHandlerByKey 按 auth 头返回模型列表；未登记的 key → 401，模拟上游判定。
// captured 回传实际收到的请求头与路径，供断言「用的是哪条链路」。
func modelsHandlerByKey(listsByAuth map[string]string, hits *atomic.Int64, captured *atomic.Value) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		if captured != nil {
			captured.Store(r.Header.Clone())
		}
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		body, ok := listsByAuth[r.Header.Get("Authorization")+r.Header.Get("x-api-key")]
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"You didn't provide an API key."}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func modelListBody(ids ...string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, fmt.Sprintf(`{"id":%q,"object":"model","created":0,"owned_by":"o"}`, id))
	}
	return `{"object":"list","data":[` + strings.Join(parts, ",") + `]}`
}

// TestNewClientFromSelection_OpenAI 锁共享装配链的四个契约（一次断言全含）：
// ① 解密后的凭据明文进 Authorization；② base_url 取端点 URL（Provider.Config 里的
// 错误主机不得被使用）；③ Provider.Config 的 custom_models 被剥（否则 Models() 本地
// 短路，压根不打上游）；④ 按端点协议派生注册类型。
func TestNewClientFromSelection_OpenAI(t *testing.T) {
	cipher := setupCipherForChannel(t)
	var hits atomic.Int64
	srv := httptest.NewServer(modelsHandlerByKey(
		map[string]string{"Bearer sk-pool-1": modelListBody("m1", "m2")}, &hits, nil))
	t.Cleanup(srv.Close)

	provider := models.Provider{
		Model:  gorm.Model{ID: 7},
		Type:   "openai",
		Config: `{"base_url":"https://should-not-be-used.invalid/v1","custom_models":["local-only"]}`,
	}
	client, err := NewClientFromSelection(provider,
		models.Endpoint{Model: gorm.Model{ID: 11}, Protocol: string(consts.ProtocolOpenAI), URL: srv.URL, Enabled: true},
		credWithKey(t, cipher, "sk-pool-1"))
	if err != nil {
		t.Fatalf("NewClientFromSelection: %v", err)
	}

	got, err := client.Models(t.Context())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(got) != 2 || got[0].ID != "m1" || got[1].ID != "m2" {
		t.Fatalf("models = %+v, want [m1 m2]（custom_models 不得短路）", got)
	}
	if hits.Load() != 1 {
		t.Fatalf("上游 hits = %d, want 1", hits.Load())
	}
}

// TestNewClientFromSelection_AnthropicUsesAPIKeyHeader 锁「按端点协议派生类型」：
// anthropic 端点必须走 x-api-key 而非 Authorization——注册类型选错就会把可用 key
// 以错误方言发出，表现为恒 401。
func TestNewClientFromSelection_AnthropicUsesAPIKeyHeader(t *testing.T) {
	cipher := setupCipherForChannel(t)
	var captured atomic.Value
	srv := httptest.NewServer(modelsHandlerByKey(
		map[string]string{"sk-anthropic": modelListBody()}, nil, &captured))
	t.Cleanup(srv.Close)

	client, err := NewClientFromSelection(
		models.Provider{Model: gorm.Model{ID: 8}, Type: "anthropic", Config: `{"base_url":"https://unused.invalid"}`},
		models.Endpoint{Model: gorm.Model{ID: 12}, Protocol: string(consts.ProtocolAnthropic), URL: srv.URL + "/v1", Enabled: true},
		credWithKey(t, cipher, "sk-anthropic"))
	if err != nil {
		t.Fatalf("NewClientFromSelection: %v", err)
	}
	// 端点 URL 带 /v1 后缀 → 请求路径为 /v1/models，故用 404 之外的方式取头：
	// 只要请求发出并带上正确凭据头即达成断言。
	_, _ = client.Models(t.Context())

	h, ok := captured.Load().(http.Header)
	if !ok {
		t.Fatal("上游未收到任何请求（客户端装配或 URL 继承链有误）")
	}
	if h.Get("x-api-key") != "sk-anthropic" {
		t.Fatalf("x-api-key = %q, want sk-anthropic", h.Get("x-api-key"))
	}
	if h.Get("Authorization") != "" {
		t.Fatalf("anthropic 端点不应带 Authorization: %q", h.Get("Authorization"))
	}
}

// TestNewClientFromSelection_UnknownProtocolFails 锁未知协议显式失败：
// 静默回落 openai 会把漏配端点按错误方言出站，宁在发请求之前失败。
func TestNewClientFromSelection_UnknownProtocolFails(t *testing.T) {
	cipher := setupCipherForChannel(t)
	_, err := NewClientFromSelection(
		models.Provider{Type: "openai", Config: `{"base_url":"https://x.invalid"}`},
		models.Endpoint{Model: gorm.Model{ID: 99}, Protocol: "grpc", URL: "https://x.invalid", Enabled: true},
		credWithKey(t, cipher, "sk-1"))
	if err == nil || !strings.Contains(err.Error(), "unknown endpoint protocol") {
		t.Fatalf("err = %v, want unknown endpoint protocol 语义", err)
	}
}

// TestNewClientFromSelection_CipherMissingFails 锁「无解密能力即显式失败」：
// 不做明文兜底降级（与 DecryptCredentialKey 同一原则——密文只有这一条解密路径）。
func TestNewClientFromSelection_CipherMissingFails(t *testing.T) {
	_, err := NewClientFromSelection(
		models.Provider{Type: "openai", Config: `{"base_url":"https://x.invalid"}`},
		models.Endpoint{Model: gorm.Model{ID: 1}, Protocol: string(consts.ProtocolOpenAI), URL: "https://x.invalid", Enabled: true},
		models.Credential{Key: "not-even-ciphertext"})
	if err == nil {
		t.Fatal("cipher 未初始化时必须失败，不得静默用密文当 api_key 出站")
	}
	if !strings.Contains(err.Error(), "cipher not initialized") {
		t.Fatalf("err = %v, want cipher not initialized 语义", err)
	}
}

// credWithKey 建一条已加密的 active 凭据（探活/同步的候选形态）。
func credWithKey(t *testing.T, cipher *credentialcrypto.Cipher, plain string) models.Credential {
	t.Helper()
	return models.Credential{
		Key:     mustEncrypt(t, plain),
		KeyHash: cipher.Hash(plain),
		Status:  models.CredentialStatusActive,
	}
}
