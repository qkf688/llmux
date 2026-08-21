package transform

import (
	"errors"
	"reflect"
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 本文件测的是 style → 认领键集合的分派层（UnknownRequestKeys）。
//
// 与 unknown_request_keys_test.go 的分工：那份验证「键集合本身对不对」，直接拿
// 子包的 ClaimedRequestKeys 比对；这份验证「按 style 取键集合这一步对不对」，
// 重点是不支持的 style 必须显式报不支持，而不是悄悄拿别的协议的键集合去算。

func TestUnknownRequestKeys_DetectsUnclaimedKeysPerStyle(t *testing.T) {
	cases := []struct {
		style consts.Style
		raw   string
		want  []string
	}{
		{
			// top_k 是 OpenAI Chat Completions DTO 不认领的常见键。
			style: consts.StyleOpenAI,
			raw:   `{"model":"gpt-5","messages":[],"top_k":40}`,
			want:  []string{"top_k"},
		},
		{
			// 同一份 body 换到 Responses 协议：messages 在那边叫 input，所以
			// messages 也进未知集合。这正说明分派必须按入站 style 取键集合。
			style: consts.StyleOpenAIRes,
			raw:   `{"model":"gpt-5","messages":[],"top_k":40}`,
			want:  []string{"messages", "top_k"},
		},
		{
			style: consts.StyleOpenAI,
			raw:   `{"model":"gpt-5","messages":[],"temperature":0.7}`,
			want:  []string{},
		},
		{
			// top_k / container 是 Anthropic 真实 API 有、但本网关入站 DTO 未认领的键，
			// 报出来是正确行为（不要为了让报告干净而把它们塞进 DTO）。
			style: consts.StyleAnthropic,
			raw:   `{"model":"claude","messages":[],"system":"s","top_k":40,"container":"c"}`,
			want:  []string{"container", "top_k"},
		},
		{
			// anthropic 独有的顶层键必须被认领，否则检测一上线就整片误报。
			style: consts.StyleAnthropic,
			raw:   `{"model":"claude","messages":[],"system":"s","stop_sequences":["x"],"thinking":{"type":"enabled","budget_tokens":1024},"output_config":{"effort":"high"},"tool_choice":"auto","metadata":{"user_id":"u"}}`,
			want:  []string{},
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.style), func(t *testing.T) {
			got, err := UnknownRequestKeys(tc.style, []byte(tc.raw))
			if err != nil {
				t.Fatalf("UnknownRequestKeys: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("UnknownRequestKeys = %v, want %v", got, tc.want)
			}
		})
	}
}

// 未注册的 style 必须显式报不支持，**不许**回退到默认协议。
//
// getAdapterOrDefault 那种静默 fallback 到 openai 的形状在这里是有害的：拿
// openai 的键集合去查 anthropic 请求，system/stop_sequences 之类会被整片误报，
// 正是 TestUnknownTopLevelKeys_OutboundRenameIsNotUnknown 要防的病灶。
func TestUnknownRequestKeys_UnsupportedStyleReportsUnsupported(t *testing.T) {
	for _, style := range []consts.Style{"some-future-style", ""} {
		t.Run(string(style), func(t *testing.T) {
			got, err := UnknownRequestKeys(style, []byte(`{"model":"m","system":"s"}`))
			if !errors.Is(err, ErrClaimedKeysUnsupported) {
				t.Fatalf("err = %v, want ErrClaimedKeysUnsupported", err)
			}
			if got != nil {
				t.Fatalf("unsupported style must not return keys, got %v", got)
			}
		})
	}
}

func TestUnknownRequestKeys_PropagatesParseError(t *testing.T) {
	if _, err := UnknownRequestKeys(consts.StyleOpenAI, []byte(`{"model":`)); err == nil {
		t.Fatal("malformed JSON must return an error")
	} else if errors.Is(err, ErrClaimedKeysUnsupported) {
		t.Fatalf("parse failure must not be reported as unsupported style: %v", err)
	}

	// 合法 JSON 但不是对象：键的概念不成立，同样该报错而不是返回空集合。
	if _, err := UnknownRequestKeys(consts.StyleOpenAI, []byte(`null`)); err == nil {
		t.Fatal("non-object payload must return an error")
	}
}

func TestRegisterClaimedRequestKeys_DuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("registering the same style twice must panic")
		}
	}()
	registerClaimedRequestKeys(consts.StyleOpenAI, func() map[string]struct{} { return nil })
}

// MismatchedRequestKeys 与 UnknownRequestKeys 是互补的两类静默检测：这里验证「键被
// 认领了、但类型不匹配被宽容容器丢弃」这条路径的分派。
func TestMismatchedRequestKeys_DetectsTypeMismatchPerStyle(t *testing.T) {
	cases := []struct {
		style consts.Style
		raw   string
		want  []string
	}{
		{
			// temperature 传字符串：openai 的 OptionalNumber 容器静默吞掉，200 通过但参数不生效。
			style: consts.StyleOpenAI,
			raw:   `{"model":"gpt-5","messages":[],"temperature":"0.5"}`,
			want:  []string{"temperature"},
		},
		{
			// 类型全对时无命中。top_k 是未认领键，归 UnknownRequestKeys 管，不在这里报。
			style: consts.StyleOpenAI,
			raw:   `{"model":"gpt-5","messages":[],"temperature":0.7,"top_k":40}`,
			want:  []string{},
		},
		{
			// anthropic 的 max_tokens/thinking.budget_tokens 走 OptionalNumber，字符串被吞。
			style: consts.StyleAnthropic,
			raw:   `{"model":"claude","messages":[],"max_tokens":"lots"}`,
			want:  []string{"max_tokens"},
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.style), func(t *testing.T) {
			got, err := MismatchedRequestKeys(tc.style, []byte(tc.raw))
			if err != nil {
				t.Fatalf("MismatchedRequestKeys: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("MismatchedRequestKeys = %v, want %v", got, tc.want)
			}
		})
	}
}

// openai-res 刻意不注册类型不匹配检测：ResponsesRequest 用裸类型/指针字段，类型不对
// 时整条请求解析失败（客户端拿到显式错误），没有需要暴露的静默丢弃。未注册的 style
// 也一样，必须显式报 unsupported 而非静默返回空集合。
func TestMismatchedRequestKeys_UnsupportedStyleReportsUnsupported(t *testing.T) {
	for _, style := range []consts.Style{consts.StyleOpenAIRes, "some-future-style", ""} {
		t.Run(string(style), func(t *testing.T) {
			got, err := MismatchedRequestKeys(style, []byte(`{"model":"m","temperature":"x"}`))
			if !errors.Is(err, ErrMismatchedKeysUnsupported) {
				t.Fatalf("err = %v, want ErrMismatchedKeysUnsupported", err)
			}
			if got != nil {
				t.Fatalf("unsupported style must not return keys, got %v", got)
			}
		})
	}
}

func TestMismatchedRequestKeys_PropagatesParseError(t *testing.T) {
	if _, err := MismatchedRequestKeys(consts.StyleOpenAI, []byte(`null`)); err == nil {
		t.Fatal("non-object payload must return an error")
	} else if errors.Is(err, ErrMismatchedKeysUnsupported) {
		t.Fatalf("parse failure must not be reported as unsupported style: %v", err)
	}
}

func TestRegisterMismatchedRequestKeys_DuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("registering the same style twice must panic")
		}
	}()
	registerMismatchedRequestKeys(consts.StyleOpenAI, func([]byte) ([]string, error) { return nil, nil })
}
