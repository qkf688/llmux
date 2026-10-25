package streaming

import (
	"strings"
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 本文件锁住 openai → responses 流式转换对上游工具调用 id/name 三种组合形状的行为。
//
// 背景（2026-10-25 生产故障链）：上游空名 tool_use → Claude Code 报
// "No such tool available:" 并把脏块写进对话历史 → 下一轮 anthropic 入站缺 id，
// llmux 补随机 id → 转换出 function.name:"" → 被出站校验拦成 400，会话卡死。
//
// 修复前 openai→responses 转换层的缺陷：
//   - 有 id 无 name：id 帧就发 output_item.added，name 写死一次为空 → 空名 tool_use；
//   - 有 name 无 id：整帧静默丢弃，工具调用从流里消失（客户端连错误都看不到）；
//   - 参数 delta 可能先于 output_item.added 发出（协议违规：delta 必须挂在已开的 item 上）。
//
// 修复口径：id+name 齐备才发 added；参数增量先进缓冲；流末兜底时有 name 无 id
// 补 call_ 前缀 id 发出、有 id 无 name 丢弃（上游没给工具名，发了就是空名脏数据）。

const (
	// fixtureToolCallNameAndID：id/name 首帧齐备——规范形状，回归基线。
	fixtureToolCallNameAndID = `data: {"id":"chatcmpl-ok","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-ok","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":\"Beijing\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-ok","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`

	// fixtureToolCallNameNoID：只有 name，全程无 id（wb2api 类上游的常见形态）。
	fixtureToolCallNameNoID = `data: {"id":"chatcmpl-noid","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-noid","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":\"Beijing\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-noid","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`

	// fixtureToolCallIDNoName：只有 id，全程无 name——脏数据形状（本次故障根源）。
	fixtureToolCallIDNoName = `data: {"id":"chatcmpl-noname","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_dirty","type":"function","function":{"arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-noname","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":\"Beijing\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-noname","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`

	// fixtureToolCallLateName：id 帧没给 name，第二帧才到；参数增量先于 name 到达。
	fixtureToolCallLateName = `data: {"id":"chatcmpl-late","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_late","type":"function","function":{"arguments":"{\"loc"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-late","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"get_weather","arguments":"ation\":\"Beijing\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-late","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`

	// fixtureToolCallNameAfterFinish：结束标记早于工具名到达——部分兼容上游先把
	// finish_reason 帧发出去，随后才补 tool_call 的 name。此时收尾已发出
	// response.completed，后续帧不得再开新 output item。
	fixtureToolCallNameAfterFinish = `data: {"id":"chatcmpl-after","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_after","type":"function","function":{"arguments":"{}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-after","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: {"id":"chatcmpl-after","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"get_weather","arguments":"{}"}}]},"finish_reason":null}]}

data: [DONE]

`

	// fixtureContentAfterFinish：message 项已开、正文已流出，finish 后又补一帧正文增量。
	fixtureContentAfterFinish = `data: {"id":"chatcmpl-contentlate","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"hi"},"finish_reason":null}]}

data: {"id":"chatcmpl-contentlate","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: {"id":"chatcmpl-contentlate","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"late"},"finish_reason":null}]}

data: [DONE]

`

	// fixtureToolStreamThenContent：纯 tool_call 流（message 项从未开过），
	// finish 后补一帧 content——终态后新开 message item（开了没关）的形状。
	fixtureToolStreamThenContent = `data: {"id":"chatcmpl-toolcontent","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_ok","type":"function","function":{"name":"get_weather","arguments":"{}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-toolcontent","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: {"id":"chatcmpl-toolcontent","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"late"},"finish_reason":null}]}

data: [DONE]

`

	// fixtureReasoningAfterFinish：reasoning 项已开，finish 后又补一帧 reasoning 增量。
	fixtureReasoningAfterFinish = `data: {"id":"chatcmpl-rsnlate","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"reasoning_content":"想"},"finish_reason":null}]}

data: {"id":"chatcmpl-rsnlate","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: {"id":"chatcmpl-rsnlate","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"reasoning_content":"续"},"finish_reason":null}]}

data: [DONE]

`
)

// functionCallAdded 返回第一个 function_call 类型的 output_item.added 事件。
// 找不到返回 output_index=-1 与 nil item。
func functionCallAdded(t *testing.T, events []sseEvent) (int, map[string]interface{}) {
	t.Helper()
	for _, ev := range events {
		if ev.typ() != "response.output_item.added" {
			continue
		}
		item, _ := ev.data["item"].(map[string]interface{})
		if item == nil {
			continue
		}
		if itemType, _ := item["type"].(string); itemType == "function_call" {
			return ev.outputIndex(t), item
		}
	}
	return -1, nil
}

// completedOutput 返回 response.completed 的 output 数组。
func completedOutput(t *testing.T, events []sseEvent) []interface{} {
	t.Helper()
	last := events[len(events)-1]
	if last.typ() != "response.completed" {
		t.Fatalf("最后一个事件是 %q，应为 response.completed", last.typ())
	}
	resp, _ := last.data["response"].(map[string]interface{})
	if resp == nil {
		t.Fatalf("response.completed 缺少 response 字段")
	}
	output, _ := resp["output"].([]interface{})
	return output
}

// outputContainsFunctionCall 断言 completed output 里含 function_call 项。
func outputContainsFunctionCall(t *testing.T, output []interface{}) {
	t.Helper()
	for _, it := range output {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		if itemType, _ := m["type"].(string); itemType == "function_call" {
			return
		}
	}
	t.Error("response.completed 的 output 未包含 function_call 项")
}

// TestOpenAIToolCall_NameAndID_Normal：id/name 首帧齐备时行为与修复前一致
// （回归基线：规范上游不能被本次改动波及）。
func TestOpenAIToolCall_NameAndID_Normal(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureToolCallNameAndID, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

	idx, item := functionCallAdded(t, events)
	if idx < 0 {
		t.Fatalf("未产出 function_call 的 output_item.added；事件序列: %v", eventTypes(events))
	}
	if name, _ := item["name"].(string); name != "get_weather" {
		t.Errorf("added 的 name 应为 get_weather，实际 %q", name)
	}
	if id, _ := item["id"].(string); id != "call_abc" {
		t.Errorf("added 的 id 应透传 call_abc，实际 %q", id)
	}

	argsDone := findEvent(t, events, "response.function_call_arguments.done")
	if args, _ := argsDone.data["arguments"].(string); args != `{"location":"Beijing"}` {
		t.Errorf("arguments.done 应为完整参数，实际 %q", args)
	}

	output := completedOutput(t, events)
	outputContainsFunctionCall(t, output)
}

// TestOpenAIToolCall_NameNoID_SynthesizedAtEnd：有 name 无 id 时，工具调用不得
// 整帧消失——流末兜底补 call_ 前缀 id 发出，参数完整。
func TestOpenAIToolCall_NameNoID_SynthesizedAtEnd(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureToolCallNameNoID, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

	idx, item := functionCallAdded(t, events)
	if idx < 0 {
		t.Fatalf("有 name 无 id 的工具调用被整帧丢弃（修复前行为）；事件序列: %v", eventTypes(events))
	}
	if name, _ := item["name"].(string); name != "get_weather" {
		t.Errorf("added 的 name 应为 get_weather，实际 %q", name)
	}
	id, _ := item["id"].(string)
	if !strings.HasPrefix(id, "call_") {
		t.Errorf("无 id 的工具调用应补 call_ 前缀的兜底 id，实际 %q", id)
	}

	argsDone := findEvent(t, events, "response.function_call_arguments.done")
	if args, _ := argsDone.data["arguments"].(string); args != `{"location":"Beijing"}` {
		t.Errorf("arguments.done 应为完整参数，实际 %q", args)
	}

	// 补发的工具调用也必须进 response.completed 的 output，否则下游判不出 finish_reason。
	outputContainsFunctionCall(t, completedOutput(t, events))
}

// TestOpenAIToolCall_IDNoName_Dropped：有 id 无 name 时不得产出空名 tool_use
// （本次故障根源）。整条工具调用丢弃，流仍正常收尾。
func TestOpenAIToolCall_IDNoName_Dropped(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureToolCallIDNoName, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

	if idx, item := functionCallAdded(t, events); idx >= 0 {
		t.Fatalf("有 id 无 name 的工具调用不应产出 added（空名 tool_use 是本次故障根源），实际 item: %v", item)
	}

	// 本 fixture 是纯 tool_call 流（无 role/content），丢弃后 output 应为空，
	// 但流必须正常以 response.completed 收尾。
	if output := completedOutput(t, events); len(output) != 0 {
		t.Errorf("空名工具调用被丢弃后 output 应为空，实际 %v", output)
	}
}

// TestOpenAIToolCall_LateName_Buffered：name 迟到时 added 延迟到 name 到达的帧，
// 取值是迟到帧的值而非首帧写死的空串；added 之前的参数增量必须补发，且
// 参数 delta 不得先于 output_item.added 发出（协议要求 delta 挂在已开的 item 上）。
func TestOpenAIToolCall_LateName_Buffered(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureToolCallLateName, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

	idx, item := functionCallAdded(t, events)
	if idx < 0 {
		t.Fatalf("name 迟到的工具调用未产出 added；事件序列: %v", eventTypes(events))
	}
	if name, _ := item["name"].(string); name != "get_weather" {
		t.Errorf("added 的 name 应取迟到帧的值 get_weather，实际 %q（修复前首帧写死为空）", name)
	}
	if id, _ := item["id"].(string); id != "call_late" {
		t.Errorf("added 的 id 应透传 call_late，实际 %q", id)
	}

	// 参数增量先于 name 到达：added 之前缓冲的参数必须补发，arguments.done 是全量。
	argsDone := findEvent(t, events, "response.function_call_arguments.done")
	if args, _ := argsDone.data["arguments"].(string); args != `{"location":"Beijing"}` {
		t.Errorf("arguments.done 应拼接缓冲参数与后续增量，实际 %q", args)
	}

	// added 必须先于该 item 的第一个参数 delta。
	seenAdded := false
	for _, ev := range events {
		switch ev.typ() {
		case "response.output_item.added":
			if ev.outputIndex(t) == idx {
				seenAdded = true
			}
		case "response.function_call_arguments.delta":
			if ev.outputIndex(t) == idx && !seenAdded {
				t.Errorf("参数 delta 先于 output_item.added 发出（output_index=%d），协议违规", idx)
			}
		}
	}
}

// assertNothingAfterCompleted 断言 response.completed 之后不再发出任何事件，
// 且已开的每个 output item 都恰有关闭（开—增—关三段式，开了没关与重复关同为违规）。
// 终态之后到达的任何帧（补发工具名 / 正文 / reasoning）都属同族协议违规，共用一份判据。
func assertNothingAfterCompleted(t *testing.T, events []sseEvent) {
	t.Helper()

	completedSeen := false
	addedIdx := map[int]bool{}
	doneIdx := map[int]bool{}
	for _, ev := range events {
		if completedSeen {
			t.Errorf("response.completed 之后仍发出 %s，终态不可撤回，协议违规", ev.typ())
			continue
		}
		switch ev.typ() {
		case "response.completed":
			completedSeen = true
		case "response.output_item.added":
			addedIdx[ev.outputIndex(t)] = true
		case "response.output_item.done":
			doneIdx[ev.outputIndex(t)] = true
		}
	}

	if !completedSeen {
		t.Fatalf("未产出 response.completed；事件序列: %v", eventTypes(events))
	}
	for idx := range addedIdx {
		if !doneIdx[idx] {
			t.Errorf("output_index=%d 的项开了却没关（缺 output_item.done），客户端判「未完成」", idx)
		}
	}
}

// TestOpenAIToolCall_NameAfterFinish_NoItemAfterCompleted 覆盖「结束标记早于工具名到达」：
// 流末兜底已丢弃该工具调用（并已发出终态）之后到达的帧不得再开新的 output item——
// 否则产出「开了却没关」的 item，严格 Responses 客户端判本轮运行失败
// （正是本次修复要消灭的故障类别）。
func TestOpenAIToolCall_NameAfterFinish_NoItemAfterCompleted(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureToolCallNameAfterFinish, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)
	assertNothingAfterCompleted(t, events)
}

// TestOpenAIResponses_NothingAfterCompleted 覆盖「终态之后补发任意增量帧」的三个形状：
// 正文增量、纯 tool_call 流后补正文（新开 message 项）、reasoning 增量。
// response.completed 一经发出，其后不得再出现任何事件——message / reasoning /
// tool_call 三路 output item 同受此约束，不能只封 tool_call 一路。
func TestOpenAIResponses_NothingAfterCompleted(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
	}{
		{name: "content_delta", in: fixtureContentAfterFinish},
		{name: "message_item_opened", in: fixtureToolStreamThenContent},
		{name: "reasoning_delta", in: fixtureReasoningAfterFinish},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)
			assertNothingAfterCompleted(t, events)
		})
	}
}
