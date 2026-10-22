package streaming

import (
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 本文件覆盖「Responses 流必须收尾」这条不变量在两类上游不合规形态下的表现：
//
//  1. 末包不带 delta 键（{"choices":[{"index":0,"finish_reason":"stop"}]}）——
//     兼容上游常见写法。修复前 finish 判定被 delta 断言挡在后面，流永不收尾。
//  2. 全程不发 finish_reason——兼容上游 / 被截断的流常见。修复前
//     state.finalize 只在 finish 分支里赋值，没 finish 就恒为 nil，EOF 兜底整段跳过。
//
// 两条的共同后果：客户端收到「有正文、无终态」的悬空流。严格 Responses 客户端
// 判「stream completed with an unfinished tool call」或干脆挂起等待终态。
//
// 收尾能力必须与「上游是否守规矩」解耦：转换层只要产出过内容，就要在流末
// 把开了的 output item 关掉并发出 response.completed。

const (
	// fixtureOpenAIFinishWithoutDeltaKey：末包省略 delta 键（不是 delta:{}，
	// 是整个键不存在）。断言链路上 finish_reason 仍能被读到。
	fixtureOpenAIFinishWithoutDeltaKey = `data: {"id":"chatcmpl-nodelta","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}

data: {"id":"chatcmpl-nodelta","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"finish_reason":"stop"}]}

data: [DONE]

`

	// fixtureOpenAINoFinishReason：上游全程不给 finish_reason，也无 [DONE]，
	// 直接 EOF。是最不守规矩但真实存在的形态。
	fixtureOpenAINoFinishReason = `data: {"id":"chatcmpl-nofinish","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"}}]}

data: {"id":"chatcmpl-nofinish","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}

`

	// fixtureOpenAINoFinishReasonWithToolCall：同上但带工具调用，额外断言
	// function_call 项也必须被关掉（arguments.done + output_item.done），
	// 否则客户端判「工具调用开了没关」。
	fixtureOpenAINoFinishReasonWithToolCall = `data: {"id":"chatcmpl-nofinish-tool","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-nofinish-tool","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":\"Beijing\"}"}}]},"finish_reason":null}]}

`
)

var openAIUnfinishedStreamFixtures = []struct {
	name string
	in   string
}{
	{name: "finish_without_delta_key", in: fixtureOpenAIFinishWithoutDeltaKey},
	{name: "no_finish_reason", in: fixtureOpenAINoFinishReason},
	{name: "no_finish_reason_with_tool_call", in: fixtureOpenAINoFinishReasonWithToolCall},
}

// TestOpenAIUnfinishedStream_ResponsesOutAlwaysCompletes 断言：无论上游给不给
// finish_reason、给不给 [DONE]，openai → openai-res 的流都必须以
// response.completed 收尾。
//
// 修复前两种形态都实测无 completed（探针见 .local/find-issues 2026-10-22 两条）。
func TestOpenAIUnfinishedStream_ResponsesOutAlwaysCompletes(t *testing.T) {
	t.Parallel()

	for _, c := range openAIUnfinishedStreamFixtures {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

			if n := countEvents(events, "response.completed"); n != 1 {
				t.Fatalf("response.completed 出现 %d 次，期望恰好 1 次；实际序列: %v", n, eventTypes(events))
			}

			last := events[len(events)-1]
			if last.typ() != "response.completed" {
				t.Errorf("最后一个事件是 %q，期望 response.completed；实际序列: %v", last.typ(), eventTypes(events))
			}
		})
	}
}

// TestOpenAIUnfinishedStream_OutputItemsAllClosed 断言兜底收尾也遵守
// 「开了的 output item 必须被关掉」这条不变量：每个 output_item.added
// 都有配对的 output_item.done，且每个 output_index 至多一个 done。
func TestOpenAIUnfinishedStream_OutputItemsAllClosed(t *testing.T) {
	t.Parallel()

	for _, c := range openAIUnfinishedStreamFixtures {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

			added := map[int]bool{}
			doneCount := map[int]int{}
			for _, ev := range events {
				switch ev.typ() {
				case "response.output_item.added":
					added[ev.outputIndex(t)] = true
				case "response.output_item.done":
					doneCount[ev.outputIndex(t)]++
				}
			}

			for idx := range added {
				if doneCount[idx] == 0 {
					t.Errorf("output_index %d 开了但没关；事件序列: %v", idx, eventTypes(events))
				}
				if doneCount[idx] > 1 {
					t.Errorf("output_index %d 被关了 %d 次，重复收尾属协议违规", idx, doneCount[idx])
				}
			}
		})
	}
}

// TestOpenAIUnfinishedStream_FunctionCallArgumentsDone 断言兜底收尾时
// function_call 项仍带 response.function_call_arguments.done——
// 客户端据此判断参数是否收全，只给 output_item.done 仍会被判「工具调用开了没关」。
func TestOpenAIUnfinishedStream_FunctionCallArgumentsDone(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAINoFinishReasonWithToolCall,
		consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

	argsDone := findEvent(t, events, "response.function_call_arguments.done")
	assertNumber(t, argsDone.data, "output_index", 1)

	// 参数必须是累积全量，而不是空串。
	arguments, ok := argsDone.data["arguments"].(string)
	if !ok {
		t.Fatalf("arguments.done 缺少 arguments 字段: %v", argsDone.data)
	}
	if arguments != `{"location":"Beijing"}` {
		t.Errorf("arguments.done 的参数应是累积全量，实际 %q", arguments)
	}
}

// TestOpenAIUnfinishedStream_CompletedCarriesUsage 断言兜底收尾不丢 usage：
// 上游在 choices:[] 尾包给的 usage 必须落到 response.completed 里。
func TestOpenAIUnfinishedStream_CompletedCarriesUsage(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAINoFinishReason,
		consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

	completed := findEvent(t, events, "response.completed")
	usage := nestedMap(t, completed.data, "response", "usage")
	assertNumber(t, usage, "input_tokens", 10)
	assertNumber(t, usage, "output_tokens", 2)
}

// TestOpenAIUnfinishedStream_LengthWithoutDeltaKeyReportsIncomplete 断言
// 末包省略 delta 键时，finish_reason=length 仍被读到并映射成 status=incomplete。
//
// 这条断言锁的是「finish 判定前置」这处改动本身：若 finish 判定被放回
// delta 断言之后，末包整帧被丢，status 会退化成兜底路径的 completed——
// 客户端看不出「输出被截断」，会拿半截答案当完整结果。
func TestOpenAIUnfinishedStream_LengthWithoutDeltaKeyReportsIncomplete(t *testing.T) {
	t.Parallel()

	const fixtureLengthWithoutDeltaKey = `data: {"id":"chatcmpl-len","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"半句"},"finish_reason":null}]}

data: {"id":"chatcmpl-len","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"finish_reason":"length"}]}

data: [DONE]

`

	events := runRealtimeTransform(t, fixtureLengthWithoutDeltaKey,
		consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

	completed := findEvent(t, events, "response.completed")
	response := nestedMap(t, completed.data, "response")
	status, _ := response["status"].(string)
	if status != "incomplete" {
		t.Errorf("status 应为 incomplete（上游 finish_reason=length），实际 %q；事件序列: %v",
			status, eventTypes(events))
	}
}

// TestOpenAIUnfinishedStream_NoDoneWithoutAdded 断言不存在「关了但没开过」的项。
//
// Responses 协议里每个 output item 都是「开—增—关」三段式：output_item.done
// 必须有配对的 output_item.added。只发 done 不发 added 同样是协议违规，
// 客户端会看到凭空冒出的 output_index。
//
// 触发场景：上游首帧就是 finish_reason（此前没有任何 delta），或整条流不带 role
// ——此时 message 项从未被 added，收尾却要发它的 done。
func TestOpenAIUnfinishedStream_NoDoneWithoutAdded(t *testing.T) {
	t.Parallel()

	// 首帧即 finish_reason：无 delta、无 role，message 项从未开过。
	const fixtureFinishOnlyFirstFrame = `data: {"id":"chatcmpl-onlyfinish","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"finish_reason":"stop"}]}

data: [DONE]

`

	// 整条流不带 role：message 项从未开过。
	const fixtureNoRoleAtAll = `data: {"id":"chatcmpl-norole","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"Hi"}}]}

`

	for _, c := range []struct {
		name string
		in   string
	}{
		{name: "finish_only_first_frame", in: fixtureFinishOnlyFirstFrame},
		{name: "no_role_at_all", in: fixtureNoRoleAtAll},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

			added := map[int]bool{}
			done := map[int]bool{}
			for _, ev := range events {
				switch ev.typ() {
				case "response.output_item.added":
					added[ev.outputIndex(t)] = true
				case "response.output_item.done":
					done[ev.outputIndex(t)] = true
				}
			}
			for idx := range done {
				if !added[idx] {
					t.Errorf("output_index %d 发了 done 却没有对应的 added；事件序列: %v",
						idx, eventTypes(events))
				}
			}
		})
	}
}
