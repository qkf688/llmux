package providers

import "testing"

// TestDeriveConfigAuthType 钉认证方式展示真值的派生口径（Stage 20 派生化）：
// ① 键拆分与 adapter 反序列化同源——anthropic 只读 auth_type、openai 系只读
// openai_auth_type（拆键互不读取的另一面由 auth_headers_test.go 钉在出站头上）；
// ② 值原样返回不做值域过滤（垃圾值暴露配置错误优于静默回默认）；
// ③ 缺键/空值/坏 JSON 一律 nil（「未设置」与「显式设置」必须可区分）。
func TestDeriveConfigAuthType(t *testing.T) {
	cases := []struct {
		name         string
		providerType string
		configJSON   string
		want         *string
	}{
		{"anthropic 读 auth_type", TypeAnthropic, `{"auth_type":"x-api-key"}`, strPtr("x-api-key")},
		{"anthropic bearer", TypeAnthropic, `{"auth_type":"bearer"}`, strPtr("bearer")},
		{"anthropic 无视 openai_auth_type 键（拆键互不读取）", TypeAnthropic, `{"openai_auth_type":"bearer"}`, nil},
		{"openai 读 openai_auth_type", TypeOpenAI, `{"openai_auth_type":"x-api-key"}`, strPtr("x-api-key")},
		{"openai 无视 auth_type 键", TypeOpenAI, `{"auth_type":"x-api-key"}`, nil},
		{"openai-res 同 openai 方言键", TypeOpenAIRes, `{"openai_auth_type":"bearer"}`, strPtr("bearer")},
		{"非 anthropic 一律归 openai 方言键", "gemini", `{"openai_auth_type":"bearer"}`, strPtr("bearer")},
		{"垃圾值原样返回不回默认", TypeAnthropic, `{"auth_type":"garbage"}`, strPtr("garbage")},
		{"缺键 = nil", TypeAnthropic, `{"base_url":"https://x"}`, nil},
		{"空值 = nil", TypeAnthropic, `{"auth_type":""}`, nil},
		{"坏 JSON = nil", TypeAnthropic, `{`, nil},
		{"空 config = nil", TypeAnthropic, ``, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveConfigAuthType(tc.providerType, tc.configJSON)
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("DeriveConfigAuthType(%q,%s) = %q, want nil", tc.providerType, tc.configJSON, *got)
			case tc.want != nil && got == nil:
				t.Fatalf("DeriveConfigAuthType(%q,%s) = nil, want %q", tc.providerType, tc.configJSON, *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Fatalf("DeriveConfigAuthType(%q,%s) = %q, want %q", tc.providerType, tc.configJSON, *got, *tc.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
