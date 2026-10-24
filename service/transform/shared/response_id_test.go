package shared

import (
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 单一来源回归：协议形状 → id 前缀。三个值同时被 transform 出站重铸与
// protocolprobe 的「前缀矛盾」判据消费，任一处改错都会让两侧口径漂移。
func TestResponseIDPrefix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		format consts.WireFormat
		want   string
		token  string
	}{
		{format: consts.FormatOpenAIChat, want: "chatcmpl-", token: "chatcmpl"},
		{format: consts.FormatOpenAIResponses, want: "resp_", token: "resp"},
		{format: consts.FormatAnthropic, want: "msg_", token: "msg"},
		{format: consts.WireFormat("unknown"), want: "", token: ""},
	}
	for _, tc := range cases {
		t.Run(string(tc.format), func(t *testing.T) {
			t.Parallel()
			if got := ResponseIDPrefix(tc.format); got != tc.want {
				t.Fatalf("ResponseIDPrefix = %q, want %q", got, tc.want)
			}
			if got := ResponseIDPrefixToken(tc.format); got != tc.token {
				t.Fatalf("ResponseIDPrefixToken = %q, want %q", got, tc.token)
			}
		})
	}
}

// 剥前缀必须逐层剥到无可剥：多跳转换会叠出 `chatcmpl-msg_xxx` 这种双前缀串，
// 只剥一层会留下另一半。
func TestBareResponseID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "anthropic", in: "msg_123", want: "123"},
		{name: "openai_dash", in: "chatcmpl-123", want: "123"},
		{name: "openai_underscore", in: "chatcmpl_123", want: "123"},
		{name: "responses", in: "resp_123", want: "123"},
		{name: "responses_dash", in: "resp-123", want: "123"},
		{name: "double_prefix", in: "chatcmpl-msg_123", want: "123"},
		{name: "triple_prefix", in: "msg_resp_chatcmpl-123", want: "123"},
		{name: "no_prefix", in: "abc", want: "abc"},
		{name: "empty", in: "", want: ""},
		// 只剥到剩前缀本身时停下：`msg_` 剥完是空串，那不是可用的裸 id。
		{name: "prefix_only", in: "msg_", want: "msg_"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := BareResponseID(tc.in); got != tc.want {
				t.Fatalf("BareResponseID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// 幂等：同一 id 反复重铸结果不变——中间格式二次转换（Anthropic→Responses→OpenAI）
// 会连续调用两次，不幂等就会一层层叠前缀。
func TestResponseIDFor_Idempotent(t *testing.T) {
	t.Parallel()
	for _, format := range []consts.WireFormat{consts.FormatOpenAIChat, consts.FormatOpenAIResponses, consts.FormatAnthropic} {
		format := format
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()
			once := ResponseIDFor(format, "msg_abc123")
			twice := ResponseIDFor(format, once)
			if once != twice {
				t.Fatalf("重铸不幂等: once=%q twice=%q", once, twice)
			}
		})
	}
}

// 重铸只换前缀，不换裸 id：调用方（客户端续话、日志排查）靠裸 id 关联请求。
func TestResponseIDFor_PreservesBareID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		format consts.WireFormat
		in     string
		want   string
	}{
		// 上游 Anthropic 方言 → OpenAI Chat：不能原样透出 msg_ 前缀（自相矛盾形状）。
		{format: consts.FormatOpenAIChat, in: "msg_2026abc", want: "chatcmpl-2026abc"},
		// 上游 Chat → Responses：剥 chatcmpl- 盖 resp_。
		{format: consts.FormatOpenAIResponses, in: "chatcmpl-2026abc", want: "resp_2026abc"},
		// 上游 Responses → Anthropic：剥 resp_ 盖 msg_。
		{format: consts.FormatAnthropic, in: "resp_2026abc", want: "msg_2026abc"},
		// 已经是目标形状：幂等（裸化后重盖，结果不变）。
		{format: consts.FormatAnthropic, in: "msg_2026abc", want: "msg_2026abc"},
	}
	for _, tc := range cases {
		t.Run(string(tc.format)+"/"+tc.in, func(t *testing.T) {
			t.Parallel()
			if got := ResponseIDFor(tc.format, tc.in); got != tc.want {
				t.Fatalf("ResponseIDFor(%s,%q) = %q, want %q", tc.format, tc.in, got, tc.want)
			}
		})
	}
}

// 空 id 原样返回：上游没给 id 时不该凭空造一个只有前缀的串——客户端会把它当成
// 一个「拿到了 id」的事实，而真实语义是「上游没给」。
func TestResponseIDFor_EmptyStaysEmpty(t *testing.T) {
	t.Parallel()
	for _, format := range []consts.WireFormat{consts.FormatOpenAIChat, consts.FormatOpenAIResponses, consts.FormatAnthropic} {
		if got := ResponseIDFor(format, ""); got != "" {
			t.Fatalf("ResponseIDFor(%s,\"\") = %q, want 空串", format, got)
		}
	}
	if got := ResponseIDFor(consts.WireFormat("unknown"), "msg_1"); got != "msg_1" {
		t.Fatalf("未注册形状应原样返回，实际 %q", got)
	}
}

// Responses 的 item 级 id 前缀固定 msg_，与响应级 resp_ 不是同一个东西：
// 用错会让客户端把 output 回灌成下一轮 input 时按 msg_ 找不到 item。
func TestResponseItemIDFor(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{in: "msg_123", want: "msg_123"},
		{in: "resp_abc", want: "msg_abc"},
		{in: "chatcmpl-abc", want: "msg_abc"},
		{in: "", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := ResponseItemIDFor(tc.in); got != tc.want {
				t.Fatalf("ResponseItemIDFor(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
