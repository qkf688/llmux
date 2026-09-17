package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Stage 4b 契约：auth_type 是凭据头唯一权威——无论入站透传/key 级/关联级配了什么，
// 上游只会收到与所选认证方式匹配的那一个凭据头，对侧凭据头在构造点被删除，
// 「两个凭据头并发送达上游」的场景清零（双头并送时生效的是哪个由上游实现定义，
// 官方与各中转口径不一，网关只承诺确定行为）。
// openai 式默认 bearer（空值=现状，存量配置零迁移）；anthropic 式默认 x-api-key。

func TestBuildReq_AuthHeaderSingleSource(t *testing.T) {
	cases := []struct {
		name         string
		providerType string
		config       string
		// preHeaders 模拟 chatcore.BuildHeaders 合并后的入站头（key 级/关联级自定义头的注入面）。
		preHeaders map[string]string
		wantAuth   string // 期望 Authorization 值；"" = 必须不存在
		wantXKey   string // 期望 x-api-key 值；"" = 必须不存在
	}{
		{
			name:         "anthropic 默认 x-api-key：配置的 Authorization 被删",
			providerType: TypeAnthropic,
			config:       `{"base_url":"https://x","api_key":"k","version":"2023-06-01"}`,
			preHeaders:   map[string]string{"Authorization": "Bearer smuggled"},
			wantAuth:     "", wantXKey: "k",
		},
		{
			name:         "anthropic 显式 x-api-key：与默认同",
			providerType: TypeAnthropic,
			config:       `{"base_url":"https://x","api_key":"k","version":"2023-06-01","auth_type":"x-api-key"}`,
			preHeaders:   map[string]string{"Authorization": "Bearer smuggled"},
			wantAuth:     "", wantXKey: "k",
		},
		{
			name:         "anthropic bearer：配置的 x-api-key 被删",
			providerType: TypeAnthropic,
			config:       `{"base_url":"https://x","api_key":"k","version":"2023-06-01","auth_type":"bearer"}`,
			preHeaders:   map[string]string{"x-api-key": "smuggled"},
			wantAuth:     "Bearer k", wantXKey: "",
		},
		{
			name:         "anthropic 未知值回退默认 x-api-key（保留旧口径）",
			providerType: TypeAnthropic,
			config:       `{"base_url":"https://x","api_key":"k","version":"2023-06-01","auth_type":"garbage"}`,
			preHeaders:   nil,
			wantAuth:     "", wantXKey: "k",
		},
		{
			name:         "openai 默认 bearer：配置的 x-api-key 被删（逃生舱废除）",
			providerType: TypeOpenAI,
			config:       `{"base_url":"https://x","api_key":"k"}`,
			preHeaders:   map[string]string{"x-api-key": "smuggled"},
			wantAuth:     "Bearer k", wantXKey: "",
		},
		{
			name:         "openai x-api-key：配置的 Authorization 被删",
			providerType: TypeOpenAI,
			config:       `{"base_url":"https://x","api_key":"k","auth_type":"x-api-key"}`,
			preHeaders:   map[string]string{"Authorization": "Bearer smuggled"},
			wantAuth:     "", wantXKey: "k",
		},
		{
			name:         "openai 未知值回退 bearer",
			providerType: TypeOpenAI,
			config:       `{"base_url":"https://x","api_key":"k","auth_type":"garbage"}`,
			preHeaders:   nil,
			wantAuth:     "Bearer k", wantXKey: "",
		},
		{
			name:         "openai-res 同基类：x-api-key 方式",
			providerType: TypeOpenAIRes,
			config:       `{"base_url":"https://x","api_key":"k","auth_type":"x-api-key"}`,
			preHeaders:   map[string]string{"Authorization": "Bearer smuggled"},
			wantAuth:     "", wantXKey: "k",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New(tc.providerType, tc.config, "")
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			h := http.Header{}
			for k, v := range tc.preHeaders {
				h.Set(k, v)
			}
			req, err := p.BuildReq(context.Background(), h, "some-model", []byte(`{"model":"m"}`))
			if err != nil {
				t.Fatalf("BuildReq: %v", err)
			}
			if got := req.Header.Get("Authorization"); got != tc.wantAuth {
				t.Errorf("Authorization = %q, want %q", got, tc.wantAuth)
			}
			if got := req.Header.Get("x-api-key"); got != tc.wantXKey {
				t.Errorf("x-api-key = %q, want %q", got, tc.wantXKey)
			}
		})
	}
}

// TestModels_AuthHeaderSingleSource 覆盖旁路位点：模型列表 GET 不走 BuildReq，
// 认证方式同样必须由 auth_type 单一决定（否则「对话用 x-api-key、拉模型用 bearer」的分裂）。
func TestModels_AuthHeaderSingleSource(t *testing.T) {
	cases := []struct {
		name         string
		providerType string
		config       string
		wantAuth     string
		wantXKey     string
	}{
		{"openai 默认 bearer", TypeOpenAI, `{"base_url":%q,"api_key":"k"}`, "Bearer k", ""},
		{"openai x-api-key", TypeOpenAI, `{"base_url":%q,"api_key":"k","auth_type":"x-api-key"}`, "", "k"},
		{"anthropic 默认 x-api-key", TypeAnthropic, `{"base_url":%q,"api_key":"k","version":"2023-06-01"}`, "", "k"},
		{"anthropic bearer", TypeAnthropic, `{"base_url":%q,"api_key":"k","version":"2023-06-01","auth_type":"bearer"}`, "Bearer k", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got http.Header
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Clone()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":[]}`))
			}))
			defer srv.Close()

			p, err := New(tc.providerType, fmt.Sprintf(tc.config, srv.URL), "")
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if _, err := p.Models(t.Context()); err != nil {
				t.Fatalf("Models: %v", err)
			}
			if auth := got.Get("Authorization"); auth != tc.wantAuth {
				t.Errorf("upstream saw Authorization = %q, want %q", auth, tc.wantAuth)
			}
			if xk := got.Get("x-api-key"); xk != tc.wantXKey {
				t.Errorf("upstream saw x-api-key = %q, want %q", xk, tc.wantXKey)
			}
		})
	}
}
