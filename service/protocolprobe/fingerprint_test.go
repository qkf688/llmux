package protocolprobe

import (
	"net/http"
	"testing"

	"github.com/qkf688/llmux/consts"
)

func TestClassifyError_Families(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		body      string
		wantFam   Family
		wantErrTp string
	}{
		{
			name:      "anthropic_native_not_found",
			body:      `{"type":"error","error":{"type":"not_found_error","message":"model: x"},"request_id":"req_1"}`,
			wantFam:   FamilyAnthropic,
			wantErrTp: "not_found_error",
		},
		{
			name:      "anthropic_invalid_request",
			body:      `{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens too large"}}`,
			wantFam:   FamilyAnthropic,
			wantErrTp: "invalid_request_error",
		},
		{
			name:      "openai_model_not_found",
			body:      `{"error":{"message":"The model does not exist","type":"invalid_request_error","param":"model","code":"model_not_found"}}`,
			wantFam:   FamilyOpenAI,
			wantErrTp: "model_not_found",
		},
		{
			// 网关方言：仍是 OpenAI 形状但 type 是自家命名——同族≠原生，故本包只归家族、
			// 不据此断言直通（是否直通由「是否与自身协议同族」在 Recommend 里比）。
			name:      "gateway_openai_dialect",
			body:      `{"error":{"message":"渠道不存在","type":"new_api_error","code":500}}`,
			wantFam:   FamilyOpenAI,
			wantErrTp: "new_api_error",
		},
		{
			name:      "google_status_shape",
			body:      `{"error":{"code":404,"message":"models/x is not found","status":"NOT_FOUND"}}`,
			wantFam:   FamilyGoogle,
			wantErrTp: "NOT_FOUND",
		},
		{
			// 数字 code 不是 Google 签名（大量兼容网关也塞数字），只认大写 status 串。
			name:      "numeric_code_only_is_not_google",
			body:      `{"error":{"code":400,"message":"bad request"}}`,
			wantFam:   FamilyOpenAI,
			wantErrTp: "400",
		},
		{
			name:    "html_error_page",
			body:    `<html><body>404 Not Found</body></html>`,
			wantFam: FamilyUnknown,
		},
		{
			name:    "valid_json_unrecognized",
			body:    `{"detail":"not found"}`,
			wantFam: FamilyUnknown,
		},
		{
			name:    "empty_body",
			body:    ``,
			wantFam: FamilyUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fam, errType, _ := ClassifyError([]byte(tc.body))
			if fam != tc.wantFam {
				t.Fatalf("family = %q, want %q", fam, tc.wantFam)
			}
			if errType != tc.wantErrTp {
				t.Fatalf("errorType = %q, want %q", errType, tc.wantErrTp)
			}
		})
	}
}

// 家族映射回归：responses 与 openai 同族（响应都按 OpenAI 方言），否则 Responses 路径
// 回 OpenAI 错误信封会被判成「不同族 → 靠转译」而误排除。注意同族只是**弱证据**：
// 正面证明直通靠 fieldSpec.strong 与 IDPrefixConflict，见 verdict.go。
func TestFamilyOfProtocol_ResponsesSharesOpenAIFamily(t *testing.T) {
	t.Parallel()
	if got := FamilyOfProtocol(consts.ProtocolResponses); got != FamilyOpenAI {
		t.Fatalf("responses family = %q, want openai", got)
	}
	if got := FamilyOfProtocol(consts.ProtocolAnthropic); got != FamilyAnthropic {
		t.Fatalf("anthropic family = %q, want anthropic", got)
	}
}

func TestNativeHeaders(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		header http.Header
		family Family
		want   []string
	}{
		{
			name: "anthropic_native_headers",
			header: http.Header{
				"Anthropic-Ratelimit-Requests-Limit": []string{"60"},
				"Request-Id":                         []string{"req_1"},
				"Cf-Ray":                             []string{"abc"},
			},
			family: FamilyAnthropic,
			want:   []string{"anthropic-ratelimit-requests-limit", "request-id"},
		},
		{
			name: "openai_native_headers",
			header: http.Header{
				"Openai-Organization": []string{"org-1"},
				"X-Request-Id":        []string{"req_2"},
				"Server":              []string{"cloudflare"},
			},
			family: FamilyOpenAI,
			want:   []string{"openai-organization", "x-request-id"},
		},
		{
			// 网关把上游头剥干净是最常见形态——此时只能算「没有弱证据」，不是「证明被转译」。
			name:   "gateway_strips_headers",
			header: http.Header{"Server": []string{"new-api"}, "Cf-Ray": []string{"abc"}},
			family: FamilyAnthropic,
			want:   nil,
		},
		{
			name:   "nil_header",
			header: nil,
			family: FamilyOpenAI,
			want:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NativeHeaders(tc.header, tc.family)
			if len(got) != len(tc.want) {
				t.Fatalf("headers = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("headers = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestInspectFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		format        consts.WireFormat
		body          string
		wantMiss      []string
		wantStrongHit []string
	}{
		{
			name:   "anthropic_full",
			format: consts.FormatAnthropic,
			body: `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"thinking","thinking":"x","signature":"sig"}],
				"stop_reason":"end_turn","stop_sequence":null,
				"usage":{"input_tokens":5,"output_tokens":2,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`,
			wantMiss:      nil,
			wantStrongHit: []string{"content[].signature"},
		},
		{
			// 转译层最典型的形状：Anthropic 特有的 cache_* 与 stop_sequence 被抹掉。
			name:   "anthropic_translated_shape",
			format: consts.FormatAnthropic,
			body: `{"id":"msg_1","type":"message","content":[{"type":"text","text":"hi"}],
				"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":2}}`,
			// signature 属机会性字段（模型不返 thinking 时本就不出现），不进 Miss。
			wantMiss: []string{"stop_sequence", "usage.cache_creation_input_tokens", "usage.cache_read_input_tokens"},
		},
		{
			// 强证据：签名串是不透明 blob，翻译层造不出来 → 进 StrongHit。
			name:   "anthropic_signature_is_strong",
			format: consts.FormatAnthropic,
			body: `{"id":"msg_1","type":"message","content":[{"type":"thinking","thinking":"x","signature":"abc"}],
				"stop_reason":"end_turn","stop_sequence":null,
				"usage":{"input_tokens":5,"output_tokens":2,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`,
			wantMiss:      nil,
			wantStrongHit: []string{"content[].signature"},
		},
		{
			name:   "openai_chat_full",
			format: consts.FormatOpenAIChat,
			body: `{"id":"chatcmpl-1","object":"chat.completion","system_fingerprint":"fp_1","service_tier":"default",
				"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
				"usage":{"prompt_tokens":5,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":0}}}`,
			wantMiss:      nil,
			wantStrongHit: []string{"system_fingerprint"},
		},
		{
			name:   "openai_chat_minimal",
			format: consts.FormatOpenAIChat,
			body: `{"id":"chatcmpl-1","object":"chat.completion",
				"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
				"usage":{"prompt_tokens":5,"completion_tokens":2}}`,
			// system_fingerprint 同上：机会性字段，缺席不算缺。
			wantMiss: []string{"usage.prompt_tokens_details", "usage.completion_tokens_details", "service_tier"},
		},
		{
			name:   "responses_full",
			format: consts.FormatOpenAIResponses,
			body: `{"id":"resp_1","object":"response","status":"completed","error":null,
				"output":[{"id":"msg_1","type":"message"}],
				"usage":{"input_tokens":5,"output_tokens":2,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}}`,
			// encrypted_content 已整条移除（探针体不带 include，结构上永不出现）。
			wantMiss: nil,
		},
		{
			// 非法 JSON 下不报「缺字段」——那会把「压根不是 JSON」说成「字段不全」。
			name:     "invalid_json",
			format:   consts.FormatAnthropic,
			body:     `not json`,
			wantMiss: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report := InspectFields(tc.format, []byte(tc.body))
			if len(report.Miss) != len(tc.wantMiss) {
				t.Fatalf("miss = %v, want %v (hit=%v)", report.Miss, tc.wantMiss, report.Hit)
			}
			for i := range report.Miss {
				if report.Miss[i] != tc.wantMiss[i] {
					t.Fatalf("miss = %v, want %v (hit=%v)", report.Miss, tc.wantMiss, report.Hit)
				}
			}
			if len(report.StrongHit) != len(tc.wantStrongHit) {
				t.Fatalf("strong_hit = %v, want %v", report.StrongHit, tc.wantStrongHit)
			}
			for i := range report.StrongHit {
				if report.StrongHit[i] != tc.wantStrongHit[i] {
					t.Fatalf("strong_hit = %v, want %v", report.StrongHit, tc.wantStrongHit)
				}
			}
			// 非 JSON 输入下 hit/miss 都为空：意思是「没检到字段信息」，不是「字段全缺」，
			// 两者的差别由这条断言守住（Total 恒 > 0，故不能拿总数相等来判）。
			if tc.name == "invalid_json" && (len(report.Hit) != 0 || len(report.Miss) != 0) {
				t.Fatalf("非法 JSON 下 hit=%v miss=%v, want 都为空", report.Hit, report.Miss)
			}
			if tc.name != "invalid_json" {
				if total := Total(tc.format); len(report.Hit)+len(report.Miss) != total {
					t.Fatalf("hit+miss = %d, want Total = %d", len(report.Hit)+len(report.Miss), total)
				}
			}
		})
	}
}

// 字段「有」才算证据：值为 0 / null 的键出现即算命中（cache 计数为 0 是合法响应，
// 但它出现本身就说明这一层没被转译抹掉）。
func TestInspectFields_ZeroValuedKeyCountsAsHit(t *testing.T) {
	t.Parallel()
	body := `{"type":"message","content":[{"type":"text"}],"stop_reason":"end_turn","stop_sequence":null,
		"usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`
	report := InspectFields(consts.FormatAnthropic, []byte(body))
	for _, key := range []string{"stop_sequence", "usage.cache_creation_input_tokens", "usage.cache_read_input_tokens"} {
		found := false
		for _, hit := range report.Hit {
			if hit == key {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("key %q 应算命中（键存在），实际 hit=%v miss=%v", key, report.Hit, report.Miss)
		}
	}
}

func TestIDPrefix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "anthropic", body: `{"id":"msg_01abc"}`, want: "msg"},
		{name: "openai", body: `{"id":"chatcmpl-123"}`, want: "chatcmpl"},
		{name: "responses", body: `{"id":"resp_123"}`, want: "resp"},
		{name: "no_id", body: `{"object":"message"}`, want: ""},
		{name: "no_separator", body: `{"id":"abc"}`, want: "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IDPrefix([]byte(tc.body)); got != tc.want {
				t.Fatalf("IDPrefix = %q, want %q", got, tc.want)
			}
		})
	}
}

// id 前缀矛盾是「翻译层拼出来的」最硬的证据：按目标协议写了骨架、却漏了源协议的 id 形状。
// 前缀不认识时必须放行——第三方实现的前缀并不统一，拿未知当转译会误杀。
func TestIDPrefixConflict(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		protocol consts.Protocol
		prefix   string
		wantTo   consts.Protocol
		want     bool
	}{
		{name: "chat_with_msg_prefix", protocol: consts.ProtocolOpenAI, prefix: "msg", wantTo: consts.ProtocolAnthropic, want: true},
		{name: "responses_with_chatcmpl_prefix", protocol: consts.ProtocolResponses, prefix: "chatcmpl", wantTo: consts.ProtocolOpenAI, want: true},
		{name: "anthropic_with_resp_prefix", protocol: consts.ProtocolAnthropic, prefix: "resp", wantTo: consts.ProtocolResponses, want: true},
		{name: "chat_native_prefix", protocol: consts.ProtocolOpenAI, prefix: "chatcmpl"},
		{name: "anthropic_native_prefix", protocol: consts.ProtocolAnthropic, prefix: "msg"},
		{name: "unknown_prefix_passes", protocol: consts.ProtocolOpenAI, prefix: "custom-abc"},
		{name: "empty_prefix_passes", protocol: consts.ProtocolOpenAI, prefix: ""},
		// 身份词恰好是已知身份词的超串：不是那个协议的实现，必须放行。
		// （判据用等值而非前缀包含，这三条是回归锁。）
		{name: "msgpack_is_not_anthropic", protocol: consts.ProtocolOpenAI, prefix: "msgpack"},
		{name: "response_is_not_responses", protocol: consts.ProtocolOpenAI, prefix: "response"},
		{name: "message_is_not_anthropic", protocol: consts.ProtocolOpenAI, prefix: "message"},
		{name: "chatcmplx_is_not_openai", protocol: consts.ProtocolAnthropic, prefix: "chatcmplx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotTo, got := IDPrefixConflict(tc.protocol, tc.prefix)
			if got != tc.want || gotTo != tc.wantTo {
				t.Fatalf("IDPrefixConflict(%q,%q) = (%q,%v), want (%q,%v)",
					tc.protocol, tc.prefix, gotTo, got, tc.wantTo, tc.want)
			}
		})
	}
}
