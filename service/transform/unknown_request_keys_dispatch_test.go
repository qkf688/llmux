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
		style string
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
	}

	for _, tc := range cases {
		t.Run(tc.style, func(t *testing.T) {
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
	// anthropic 是真实存在的 style，但入站没有请求 DTO 可反射（见
	// service/transform/anthropic/adapter.go 的说明），属于已知限制。
	for _, style := range []string{consts.StyleAnthropic, "some-future-style", ""} {
		t.Run(style, func(t *testing.T) {
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
