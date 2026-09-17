package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
)

// Stage 4b 契约：auth_type 是凭据头唯一权威——无论入站透传/key 级/关联级配了什么，
// 上游只会收到与所选认证方式匹配的那一个凭据头，对侧凭据头在构造点被删除，
// 「两个凭据头并发送达上游」的场景清零（双头并送时生效的是哪个由上游实现定义，
// 官方与各中转口径不一，网关只承诺确定行为）。
// openai 式默认 bearer（空值=现状，存量配置零迁移）；anthropic 式默认 x-api-key。

// assertAuthHeader 断言凭据头的「存在且唯一值」或「键完全不存在」。
// 不用 Get()==""：键缺失与「键存在且值为空串」在 Get 上不可区分，而空值头是另一种
// wire 形态（setAnthropicBeta 注释警告过的同类坑——把 Del 误写成 Set("") 必须能红）。
func assertAuthHeader(t *testing.T, h http.Header, name, want string) {
	t.Helper()
	vals, ok := h[textproto.CanonicalMIMEHeaderKey(name)]
	if want == "" {
		if ok {
			t.Fatalf("header %q present with %q, want key absent", name, vals)
		}
		return
	}
	if !ok || len(vals) != 1 || vals[0] != want {
		t.Fatalf("header %q = %q, want exactly [%q]", name, vals, want)
	}
}

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
			name:         "openai x-api-key：配置的 Authorization 被删（方言键 openai_auth_type，与 anthropic 的 auth_type 分键防混合协议串方言）",
			providerType: TypeOpenAI,
			config:       `{"base_url":"https://x","api_key":"k","openai_auth_type":"x-api-key"}`,
			preHeaders:   map[string]string{"Authorization": "Bearer smuggled"},
			wantAuth:     "", wantXKey: "k",
		},
		{
			name:         "openai 未知值回退 bearer",
			providerType: TypeOpenAI,
			config:       `{"base_url":"https://x","api_key":"k","openai_auth_type":"garbage"}`,
			preHeaders:   nil,
			wantAuth:     "Bearer k", wantXKey: "",
		},
		{
			// P0 回归钉（审查裁决）：混合协议供应商的 config 单份喂两家 adapter，
			// anthropic 方言键 auth_type 绝不能被 openai 读到——存量
			// 「anthropic 主类型 + openai 协议」行的 auth_type=x-api-key 不得翻转 openai 端点。
			name:         "openai 无视 anthropic 的 auth_type 键（按协议拆方言）",
			providerType: TypeOpenAI,
			config:       `{"base_url":"https://x","api_key":"k","auth_type":"x-api-key"}`,
			preHeaders:   nil,
			wantAuth:     "Bearer k", wantXKey: "",
		},
		{
			name:         "anthropic 无视 openai 的 openai_auth_type 键",
			providerType: TypeAnthropic,
			config:       `{"base_url":"https://x","api_key":"k","version":"2023-06-01","openai_auth_type":"bearer"}`,
			preHeaders:   nil,
			wantAuth:     "", wantXKey: "k",
		},
		{
			name:         "openai-res 同基类：x-api-key 方式",
			providerType: TypeOpenAIRes,
			config:       `{"base_url":"https://x","api_key":"k","openai_auth_type":"x-api-key"}`,
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
			assertAuthHeader(t, req.Header, "Authorization", tc.wantAuth)
			assertAuthHeader(t, req.Header, "x-api-key", tc.wantXKey)
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
		{"openai x-api-key", TypeOpenAI, `{"base_url":%q,"api_key":"k","openai_auth_type":"x-api-key"}`, "", "k"},
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
			assertAuthHeader(t, got, "Authorization", tc.wantAuth)
			assertAuthHeader(t, got, "x-api-key", tc.wantXKey)
		})
	}
}
