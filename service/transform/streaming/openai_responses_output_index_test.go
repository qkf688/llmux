package streaming

import (
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 本文件覆盖「Responses 流里 output_index 与 item 的归属关系」这条不变量。
//
// 背景（2026-10-22 探针实测）：上游 OpenAI Chat 流可能先给 reasoning_content
// 或 tool_calls、再给 role。此时：
//   - message 项的 output_index 是惰性分配的（reasoning 占了 0、message 拿 1），
//     但 response.output_text.delta 的 output_index 写死 0 → 正文被挂到 reasoning 项上，
//     message 项正文为空；
//   - response.created 挂在 role 上 → reasoning 项的 output_item.added 先于
//     response.created 出现，违反「response.created 是流首事件」。
//
// 判据：每个事件的 output_index 必须指向它真正所属的那个 item，
// 且 response.created 必须是流里的第一个事件。

const (
	// fixtureOpenAIReasoningBeforeRole：reasoning_content 先于 role 到达。
	fixtureOpenAIReasoningBeforeRole = `data: {"id":"chatcmpl-rbr","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"reasoning_content":"先想一下"},"finish_reason":null}]}

data: {"id":"chatcmpl-rbr","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"最终答案"},"finish_reason":null}]}

data: {"id":"chatcmpl-rbr","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

data: [DONE]

`

	// fixtureOpenAIToolCallBeforeRole：tool_calls 先于 role 到达。
	fixtureOpenAIToolCallBeforeRole = `data: {"id":"chatcmpl-tbr","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"run_code","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-tbr","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"code\":\"print(1)\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-tbr","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"跑完了"},"finish_reason":null}]}

data: {"id":"chatcmpl-tbr","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

data: [DONE]

`

	// fixtureOpenAINoRoleAtAll：整条流不带 role（部分兼容网关的形态）。
	fixtureOpenAINoRoleAtAll = `data: {"id":"chatcmpl-norole","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"没有 role 的正文"},"finish_reason":null}]}

data: {"id":"chatcmpl-norole","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

data: [DONE]

`
)

// messageItemIndex 返回 message 类型 output item 的 output_index，找不到返回 -1。
func messageItemIndex(t *testing.T, events []sseEvent) int {
	t.Helper()
	for _, ev := range events {
		if ev.typ() != "response.output_item.added" {
			continue
		}
		item, _ := ev.data["item"].(map[string]interface{})
		if item == nil {
			continue
		}
		if itemType, _ := item["type"].(string); itemType == "message" {
			return ev.outputIndex(t)
		}
	}
	return -1
}

// itemIndexOfType 返回指定类型 output item 的 output_index，找不到返回 -1。
func itemIndexOfType(t *testing.T, events []sseEvent, itemType string) int {
	t.Helper()
	for _, ev := range events {
		if ev.typ() != "response.output_item.added" {
			continue
		}
		item, _ := ev.data["item"].(map[string]interface{})
		if item == nil {
			continue
		}
		if got, _ := item["type"].(string); got == itemType {
			return ev.outputIndex(t)
		}
	}
	return -1
}

// TestOpenAIResponses_OutputIndexBelongsToItsItem 断言每个 output_text / content_part
// 事件的 output_index 都指向 message 项自己，而不是别人（reasoning / function_call）。
//
// 修复前：正文 delta 的 output_index 写死 0，reasoning 先到时正文被挂到 reasoning 项上。
func TestOpenAIResponses_OutputIndexBelongsToItsItem(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		in   string
	}{
		{name: "reasoning_before_role", in: fixtureOpenAIReasoningBeforeRole},
		{name: "tool_call_before_role", in: fixtureOpenAIToolCallBeforeRole},
		{name: "no_role_at_all", in: fixtureOpenAINoRoleAtAll},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

			msgIdx := messageItemIndex(t, events)
			if msgIdx < 0 {
				t.Fatalf("未产出 message 类型的 output item；事件序列: %v", eventTypes(events))
			}

			for _, ev := range events {
				switch ev.typ() {
				case "response.output_text.delta",
					"response.output_text.done",
					"response.content_part.added",
					"response.content_part.done":
					if got := ev.outputIndex(t); got != msgIdx {
						t.Errorf("事件 %s 的 output_index=%d，应为 message 项的 %d；事件序列: %v",
							ev.typ(), got, msgIdx, eventTypes(events))
					}
				}
			}
		})
	}
}

// TestOpenAIResponses_TextDeltaNotOnForeignItem 断言正文增量不落在 reasoning /
// function_call 项上——即使那些项先于 message 项开出来。
func TestOpenAIResponses_TextDeltaNotOnForeignItem(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name      string
		in        string
		otherType string
	}{
		{name: "reasoning_before_role", in: fixtureOpenAIReasoningBeforeRole, otherType: "reasoning"},
		{name: "tool_call_before_role", in: fixtureOpenAIToolCallBeforeRole, otherType: "function_call"},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

			otherIdx := itemIndexOfType(t, events, c.otherType)
			if otherIdx < 0 {
				t.Fatalf("未产出 %s 类型的 output item；事件序列: %v", c.otherType, eventTypes(events))
			}

			msgIdx := messageItemIndex(t, events)
			if otherIdx == msgIdx {
				t.Errorf("%s 项与 message 项共用了 output_index=%d，两者必须互不重叠",
					c.otherType, otherIdx)
			}

			for _, ev := range events {
				if ev.typ() != "response.output_text.delta" {
					continue
				}
				if got := ev.outputIndex(t); got == otherIdx {
					t.Errorf("正文增量落在 %s 项的 output_index=%d 上；事件序列: %v",
						c.otherType, got, eventTypes(events))
				}
			}
		})
	}
}

// TestOpenAIResponses_CreatedIsFirstEvent 断言 response.created 是流里的第一个事件。
//
// 上游先给 reasoning_content / tool_calls 再给 role 时，若 response.created 挂在
// role 上，reasoning 项的 output_item.added 会先于它出现——违反 Responses 协议
// 「response.created 是流首事件」。
func TestOpenAIResponses_CreatedIsFirstEvent(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		in   string
	}{
		{name: "reasoning_before_role", in: fixtureOpenAIReasoningBeforeRole},
		{name: "tool_call_before_role", in: fixtureOpenAIToolCallBeforeRole},
		{name: "no_role_at_all", in: fixtureOpenAINoRoleAtAll},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

			if events[0].typ() != "response.created" {
				t.Errorf("流首事件是 %q，应为 response.created；事件序列: %v",
					events[0].typ(), eventTypes(events))
			}
		})
	}
}

// TestOpenAIResponses_MessageItemOpensWithoutRole 断言上游整条流不带 role 时，
// 仍会开出 message 项——否则客户端拿到「有正文、无 message 项」的裸 delta 流。
func TestOpenAIResponses_MessageItemOpensWithoutRole(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAINoRoleAtAll,
		consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

	added := findEvent(t, events, "response.output_item.added")
	item, _ := added.data["item"].(map[string]interface{})
	if item == nil {
		t.Fatalf("output_item.added 缺少 item 字段: %v", added.data)
	}
	if itemType, _ := item["type"].(string); itemType != "message" {
		t.Fatalf("首个 output item 的类型是 %q，应为 message", itemType)
	}
	if role, _ := item["role"].(string); role != "assistant" {
		t.Errorf("message 项的 role 应为 assistant，实际 %q", role)
	}
}
