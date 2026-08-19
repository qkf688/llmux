package transform

import (
	"reflect"
	"testing"

	"github.com/qkf688/llmux/service/transform/openai"
	"github.com/qkf688/llmux/service/transform/responses"
	"github.com/qkf688/llmux/service/transform/shared"
)

// 本文件是「入站未知字段检测」的跨协议契约测试。放顶层包是因为只有这里能同时
// import openai 与 responses 两个子包，对照两份键集合。
//
// 要证明的核心性质：判定只看「入站 DTO 认领了哪些键」，与出站会不会改名完全
// 无关。这正是旧方案（比对入站 body 与出站 body 的顶层键）做不到的——那种比法
// 把改名当成丢失，靠手工白名单补也只是维护转换逻辑的第二份副本。

func TestClaimedRequestKeys_ProtocolsClaimTheirOwnKeys(t *testing.T) {
	cases := []struct {
		style      string
		claimed    map[string]struct{}
		mustHave   []string
		mustNotHav []string
	}{
		{
			style:      "openai",
			claimed:    openai.ClaimedRequestKeys(),
			mustHave:   []string{"model", "messages", "max_tokens", "stop", "reasoning_effort", "service_tier", "safety_identifier", "prompt_cache_key"},
			mustNotHav: []string{"input", "instructions", "max_output_tokens", "reasoning"},
		},
		{
			style:      "openai-res",
			claimed:    responses.ClaimedRequestKeys(),
			mustHave:   []string{"model", "input", "instructions", "max_output_tokens", "reasoning", "service_tier", "safety_identifier", "prompt_cache_key"},
			mustNotHav: []string{"messages", "max_tokens", "stop", "reasoning_effort"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.style, func(t *testing.T) {
			for _, key := range tc.mustHave {
				if _, ok := tc.claimed[key]; !ok {
					t.Errorf("%s must claim %q", tc.style, key)
				}
			}
			for _, key := range tc.mustNotHav {
				if _, ok := tc.claimed[key]; ok {
					t.Errorf("%s must not claim %q (it belongs to the other protocol)", tc.style, key)
				}
			}
		})
	}
}

// 一份贴近真实客户端的 OpenAI 请求：所有键都该被认领，一个都不许报。
const realisticOpenAIRequest = `{
  "model": "gpt-5",
  "messages": [{"role": "user", "content": "hi"}],
  "stream": true,
  "stream_options": {"include_usage": true},
  "max_completion_tokens": 1024,
  "temperature": 0.7,
  "top_p": 0.9,
  "stop": ["\n\n"],
  "tools": [],
  "tool_choice": "auto",
  "parallel_tool_calls": true,
  "reasoning_effort": "high",
  "response_format": {"type": "json_object"},
  "metadata": {"trace": "abc"},
  "user": "u-1",
  "service_tier": "flex",
  "safety_identifier": "s-1",
  "prompt_cache_key": "ck-1"
}`

func TestUnknownTopLevelKeys_RealisticOpenAIRequestHasNone(t *testing.T) {
	unknown, err := shared.UnknownTopLevelKeys([]byte(realisticOpenAIRequest), openai.ClaimedRequestKeys())
	if err != nil {
		t.Fatalf("UnknownTopLevelKeys: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("realistic request must not report unknown keys, got %v", unknown)
	}
}

func TestUnknownTopLevelKeys_DetectsUnclaimedKeys(t *testing.T) {
	raw := []byte(`{"model":"gpt-5","messages":[],"vendor_private_knob":1,"top_k":40}`)

	unknown, err := shared.UnknownTopLevelKeys(raw, openai.ClaimedRequestKeys())
	if err != nil {
		t.Fatalf("UnknownTopLevelKeys: %v", err)
	}
	// top_k 是真实存在的情况：部分客户端会带它，但 OpenAI Chat Completions DTO
	// 不认领，转换后就悄悄没了。这类键正是本检测要暴露的对象。
	if want := []string{"top_k", "vendor_private_knob"}; !reflect.DeepEqual(unknown, want) {
		t.Fatalf("unknown = %v, want %v", unknown, want)
	}
}

// 出站改名不构成未知字段——这是本方案相对「入站/出站键求差」的关键差异。
//
// max_tokens 走 openai → openai-res 转换时会被改名成 max_output_tokens，旧方案
// 会把它判成丢失；这里用入站 style 的键集合判定，它是已认领键，不报。
//
// 同时给出反向对照：拿出站协议的键集合去查同一份入站 body，max_tokens 反而会被
// 报成未知。所以消费方必须用「入站 style」的键集合，用错就退化成旧方案的误报。
func TestUnknownTopLevelKeys_OutboundRenameIsNotUnknown(t *testing.T) {
	inbound := []byte(`{"model":"gpt-5","messages":[],"max_tokens":256,"stop":["x"]}`)

	unknown, err := shared.UnknownTopLevelKeys(inbound, openai.ClaimedRequestKeys())
	if err != nil {
		t.Fatalf("UnknownTopLevelKeys with inbound style: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("renamed-on-outbound keys must not be unknown, got %v", unknown)
	}

	mismatched, err := shared.UnknownTopLevelKeys(inbound, responses.ClaimedRequestKeys())
	if err != nil {
		t.Fatalf("UnknownTopLevelKeys with wrong style: %v", err)
	}
	// messages 也在内：Responses 协议把它叫 input。改名字段全被误报，正是旧方案的病灶。
	if want := []string{"max_tokens", "messages", "stop"}; !reflect.DeepEqual(mismatched, want) {
		t.Fatalf("using the outbound style must misreport renamed keys (guards against regressing to the old diff approach): got %v, want %v", mismatched, want)
	}
}
