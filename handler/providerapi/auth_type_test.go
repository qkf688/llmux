package providerapi

import (
	"testing"

	"github.com/qkf688/llmux/providers"
)

// resolveAuthType 钉顶层 auth_type 列的入库口径（Stage 4b 审查回批收紧）：
// ① 列是 anthropic 专属展示副本——openai 系方言真值在 config.openai_auth_type，
// 不经本列（防止「列显 bearer、实际发 x-api-key」的双写漂移）；
// ② 值必须 ∈ {bearer, x-api-key}，垃圾值不再落列（旧代码任意非空即收）。
func TestResolveAuthType(t *testing.T) {
	cases := []struct {
		name         string
		providerType string
		authType     string
		want         *string
	}{
		{"anthropic bearer 收录", providers.TypeAnthropic, "bearer", strPtr("bearer")},
		{"anthropic x-api-key 收录", providers.TypeAnthropic, "x-api-key", strPtr("x-api-key")},
		{"anthropic 空值 = 回退默认，不落列", providers.TypeAnthropic, "", nil},
		{"anthropic 未知值拒绝（旧口径任意非空即收已收紧）", providers.TypeAnthropic, "garbage", nil},
		{"openai 方言不落本列（config 单源，审查 P0）", providers.TypeOpenAI, "x-api-key", nil},
		{"openai-res 同上", providers.TypeOpenAIRes, "bearer", nil},
		{"其它类型不收", "gemini", "bearer", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveAuthType(tc.providerType, tc.authType)
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("resolveAuthType(%q,%q) = %q, want nil", tc.providerType, tc.authType, *got)
			case tc.want != nil && got == nil:
				t.Fatalf("resolveAuthType(%q,%q) = nil, want %q", tc.providerType, tc.authType, *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Fatalf("resolveAuthType(%q,%q) = %q, want %q", tc.providerType, tc.authType, *got, *tc.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
