package pools

import (
	"strings"
	"testing"

	"github.com/qkf688/llmux/consts"
)

// TestCredentialProbeBody 锁 #24-B 探针体经 providers 注册表（Metadata.ProbeBody）取且三协议形状正确：
// 「状态码通过判定（200/201）=passed」的前提是 body 对上游合法——anthropic 缺 max_tokens、responses 用错字段会被上游
// 判 400，把好 key 误判成坏。model 不在此体内（由 BuildReq sjson 注入指定 test_model）。
// 这是 anthropic/responses 探针体此前 e2e 零覆盖的回归网（e2e 假上游只走 openai chat/completions）。
func TestCredentialProbeBody(t *testing.T) {
	cases := []struct {
		name      string
		protocol  consts.Protocol
		wantKeys  []string
		notExpect string
	}{
		{"openai chat", consts.ProtocolOpenAI, []string{`"messages"`, `"content"`, `"max_tokens"`}, `"input"`},
		{"responses", consts.ProtocolResponses, []string{`"input"`, `"max_output_tokens"`}, `"messages"`},
		{"anthropic", consts.ProtocolAnthropic, []string{`"max_tokens"`, `"messages"`, `"content"`}, `"input"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, ok := credentialProbeBody(tc.protocol)
			if !ok {
				t.Fatalf("协议 %q 未取到探针体（provider 未注册 ProbeBody？）", tc.protocol)
			}
			s := string(body)
			for _, key := range tc.wantKeys {
				if !strings.Contains(s, key) {
					t.Fatalf("探针体 %q 应含 %q；实际=%s", tc.name, key, s)
				}
			}
			if tc.notExpect != "" && strings.Contains(s, tc.notExpect) {
				t.Fatalf("探针体 %q 不应含 %q；实际=%s", tc.name, tc.notExpect, s)
			}
			// 探针体不得内嵌 model（model 由 BuildReq 注入），否则会与指定 test_model 冲突。
			if strings.Contains(s, `"model"`) {
				t.Fatalf("探针体不应内嵌 model（由 BuildReq 注入）；实际=%s", s)
			}
		})
	}

	// 未知协议（provider type 无法从协议反推 / 未注册）→ ok=false，调用方判失败而不谎测。
	if _, ok := credentialProbeBody(consts.Protocol("bogus")); ok {
		t.Fatal("未知协议应回 ok=false（不拿空 body 打上游）")
	}
}
