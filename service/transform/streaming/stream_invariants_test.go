package streaming

import (
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 本文件是协议不变量断言，与 golden 快照相互独立：
// golden 只证明「输出没变」，这里证明「输出是对的」。
// 待办 22–25 的修复以本文件转绿为准，golden 重生成只是事后快照。

const (
	// OpenAI Chat 流：一个 tool_call，finish_reason=tool_calls。
	fixtureOpenAIToolCallStream = `data: {"id":"chatcmpl-inv","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":null,"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-inv","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":\"Beijing\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-inv","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`

	// Anthropic 流：文本块 + tool_use 块，stop_reason=tool_use。
	fixtureAnthropicToolUseStream = `event: message_start
data: {"type":"message_start","message":{"id":"msg_inv","type":"message","role":"assistant","content":[],"model":"test-model","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Let me check."}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_abc","name":"get_weather","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"location\":\"Beijing\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":10,"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`
)

// 待办 24：Anthropic content_block 只允许这些类型，"message" 不是合法值。
var legalAnthropicBlockTypes = map[string]bool{
	"text":                   true,
	"thinking":               true,
	"redacted_thinking":      true,
	"tool_use":               true,
	"server_tool_use":        true,
	"web_search_tool_result": true,
	"image":                  true,
}

// TestInvariant_AnthropicOut_BlockTypesAndIndices 覆盖待办 24（非法块类型）与 25（index 撞车）。
func TestInvariant_AnthropicOut_BlockTypesAndIndices(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		from consts.WireFormat
		in   string
	}{
		{name: "from_openai", from: consts.FormatOpenAIChat, in: fixtureOpenAIToolCallStream},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, c.from, consts.FormatAnthropic)
			openBlocks := map[int]string{}

			for _, ev := range events {
				switch ev.typ() {
				case "content_block_start":
					block, ok := ev.data["content_block"].(map[string]interface{})
					if !ok {
						t.Fatalf("content_block_start 缺少 content_block 字段: %v", ev.data)
					}
					blockType, _ := block["type"].(string)

					// 待办 24
					if !legalAnthropicBlockTypes[blockType] {
						t.Errorf("非法的 content_block 类型 %q（合法值见 legalAnthropicBlockTypes）", blockType)
					}
					// 待办 22：tool_use 必须带 id 与 name
					if blockType == "tool_use" {
						if id, _ := block["id"].(string); id == "" {
							t.Errorf("tool_use content_block 丢失 id: %v", block)
						}
						if name, _ := block["name"].(string); name == "" {
							t.Errorf("tool_use content_block 丢失 name: %v", block)
						}
					}

					// 待办 25：同一 index 不得在未 stop 时重复 start
					idx := ev.index(t)
					if prev, dup := openBlocks[idx]; dup {
						t.Errorf("content_block index %d 重复 start（前一个是 %q，本次是 %q），start/stop 不配对",
							idx, prev, blockType)
					}
					openBlocks[idx] = blockType

				case "content_block_stop":
					idx := ev.index(t)
					if _, open := openBlocks[idx]; !open {
						t.Errorf("content_block index %d 收到 stop 但没有对应的 start", idx)
					}
					delete(openBlocks, idx)
				}
			}

			if len(openBlocks) != 0 {
				t.Errorf("流结束时仍有未关闭的 content_block: %v", openBlocks)
			}
		})
	}
}

// TestInvariant_AnthropicOut_StopReasonFollowsUpstream 覆盖待办 23 的 openai 侧同源缺陷：
// 上游产出了 tool_call，发给 Anthropic 客户端的 stop_reason 必须是 tool_use 而非 end_turn。
func TestInvariant_AnthropicOut_StopReasonFollowsUpstream(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAIToolCallStream, "openai", "anthropic")

	found := false
	for _, ev := range events {
		if ev.typ() != "message_delta" {
			continue
		}
		found = true
		delta, ok := ev.data["delta"].(map[string]interface{})
		if !ok {
			t.Fatalf("message_delta 缺少 delta 字段: %v", ev.data)
		}
		if got, _ := delta["stop_reason"].(string); got != "tool_use" {
			t.Errorf("上游 finish_reason=tool_calls，stop_reason 应为 tool_use，实际 %q", got)
		}
	}
	if !found {
		t.Fatal("未收到 message_delta 事件")
	}
}

// TestInvariant_ResponsesOut_OutputIndexAndCompleted 覆盖待办 25（output_index 撞车）
// 与待办 23（response.completed 的 output 恒空）。
func TestInvariant_ResponsesOut_OutputIndexAndCompleted(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		from             consts.WireFormat
		in               string
		wantFunctionCall bool
	}{
		{name: "from_openai", from: consts.FormatOpenAIChat, in: fixtureOpenAIToolCallStream, wantFunctionCall: true},
		{name: "from_anthropic", from: consts.FormatAnthropic, in: fixtureAnthropicToolUseStream, wantFunctionCall: true},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, c.from, consts.FormatOpenAIResponses)

			// 待办 25：不同 item 不得共用同一个 output_index
			indexToItemID := map[int]string{}
			sawFunctionCallItem := false
			for _, ev := range events {
				if ev.typ() != "response.output_item.added" {
					continue
				}
				item, ok := ev.data["item"].(map[string]interface{})
				if !ok {
					t.Fatalf("output_item.added 缺少 item 字段: %v", ev.data)
				}
				itemID, _ := item["id"].(string)
				itemType, _ := item["type"].(string)

				idx := ev.outputIndex(t)
				if prev, dup := indexToItemID[idx]; dup && prev != itemID {
					t.Errorf("output_index %d 被两个不同 item 共用（%q 与 %q）", idx, prev, itemID)
				}
				indexToItemID[idx] = itemID

				// 待办 22：function_call 必须带 id 与 name
				if itemType == "function_call" {
					sawFunctionCallItem = true
					if itemID == "" {
						t.Errorf("function_call item 丢失 id: %v", item)
					}
					if name, _ := item["name"].(string); name == "" {
						t.Errorf("function_call item 丢失 name: %v", item)
					}
				}
			}

			if c.wantFunctionCall && !sawFunctionCallItem {
				t.Error("上游有工具调用，但未产出 function_call 的 output_item.added")
			}

			// 待办 23：response.completed 的 output 不得恒为空
			last := events[len(events)-1]
			if last.typ() != "response.completed" {
				t.Fatalf("最后一个事件应为 response.completed，实际 %q", last.typ())
			}
			resp, ok := last.data["response"].(map[string]interface{})
			if !ok {
				t.Fatalf("response.completed 缺少 response 字段: %v", last.data)
			}
			output, _ := resp["output"].([]interface{})
			if len(output) == 0 {
				t.Error("response.completed 的 output 为空，下游无从判定 finish_reason")
			}
			if c.wantFunctionCall {
				hasFunctionCall := false
				for _, item := range output {
					m, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					if t2, _ := m["type"].(string); t2 == "function_call" {
						hasFunctionCall = true
					}
				}
				if !hasFunctionCall {
					t.Error("response.completed 的 output 未包含 function_call 项")
				}
			}
		})
	}
}

// TestInvariant_OpenAIOut_ToolCallSurvivesAnthropicUpstream 覆盖待办 22/23 的端到端后果：
// Anthropic 上游 + OpenAI 客户端时，SDK 必须能重建 tool_call 并看到 finish_reason=tool_calls。
func TestInvariant_OpenAIOut_ToolCallSurvivesAnthropicUpstream(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureAnthropicToolUseStream, "anthropic", "openai")

	var gotID, gotName, gotArgs, gotFinish string
	for _, ev := range events {
		choices, ok := ev.data["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}
		choice, ok := choices[0].(map[string]interface{})
		if !ok {
			continue
		}
		if fr, _ := choice["finish_reason"].(string); fr != "" {
			gotFinish = fr
		}
		delta, ok := choice["delta"].(map[string]interface{})
		if !ok {
			continue
		}
		toolCalls, ok := delta["tool_calls"].([]interface{})
		if !ok {
			continue
		}
		for _, tc := range toolCalls {
			m, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}
			if id, _ := m["id"].(string); id != "" {
				gotID = id
			}
			fn, ok := m["function"].(map[string]interface{})
			if !ok {
				continue
			}
			if name, _ := fn["name"].(string); name != "" {
				gotName = name
			}
			if args, _ := fn["arguments"].(string); args != "" {
				gotArgs += args
			}
		}
	}

	if gotID != "toolu_abc" {
		t.Errorf("tool_call id 应透传为 toolu_abc，实际 %q", gotID)
	}
	if gotName != "get_weather" {
		t.Errorf("tool_call name 应透传为 get_weather，实际 %q", gotName)
	}
	if gotArgs != `{"location":"Beijing"}` {
		t.Errorf("tool_call arguments 应完整拼回，实际 %q", gotArgs)
	}
	if gotFinish != "tool_calls" {
		t.Errorf("finish_reason 应为 tool_calls，实际 %q", gotFinish)
	}
}

// fixtureAnthropicToolUseOnlyStream 是「纯 tool_use」的 anthropic 上游（无任何文本块），
// 用于覆盖待办 45：anthropic→responses 不应在没有文本块时发 message 类型的 done 事件。
const fixtureAnthropicToolUseOnlyStream = `event: message_start
data: {"type":"message_start","message":{"id":"msg_toolonly","type":"message","role":"assistant","content":[],"model":"test-model","stop_reason":null,"usage":{"input_tokens":5,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_x","name":"fn","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":5,"output_tokens":3}}

event: message_stop
data: {"type":"message_stop"}

`

// TestInvariant_ResponsesOut_PureToolUseNoMessageDone 覆盖待办 45：
// anthropic→responses 纯 tool_use 流不应发 message 类型的 done 事件
// （output_text.done / content_part.done / output_item.done[message]）。
// 这些事件在纯 tool_use 流下：(a) 事件流冗余，(b) 会让 messageOutputIndex=0
// 与 blockIndex=0 的 function_call 撞车。reasoning 路径独立不受影响。
func TestInvariant_ResponsesOut_PureToolUseNoMessageDone(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureAnthropicToolUseOnlyStream, "anthropic", "openai-res")

	forbidden := map[string]bool{
		"response.output_text.done":  true,
		"response.content_part.done": true,
	}
	gotFunctionCallAdded := false
	for _, ev := range events {
		typ := ev.typ()
		if forbidden[typ] {
			t.Errorf("纯 tool_use 流不应发 %s，实际事件: %v", typ, ev.data)
		}
		if typ == "response.output_item.added" {
			if item, ok := ev.data["item"].(map[string]interface{}); ok {
				if t2, _ := item["type"].(string); t2 == "function_call" {
					gotFunctionCallAdded = true
				}
				if t2, _ := item["type"].(string); t2 == "message" {
					t.Errorf("纯 tool_use 流不应出现 message 类型的 output_item.added: %v", ev.data)
				}
			}
		}
		if typ == "response.output_item.done" {
			if item, ok := ev.data["item"].(map[string]interface{}); ok {
				if t2, _ := item["type"].(string); t2 == "message" {
					t.Errorf("纯 tool_use 流不应出现 message 类型的 output_item.done: %v", ev.data)
				}
			}
		}
	}
	if !gotFunctionCallAdded {
		t.Error("应至少发一次 function_call 类型的 output_item.added")
	}
}

// TestInvariant_ResponsesOut_SequenceNumberMonotonic 覆盖待办 43：
// openai→responses 流式 tool 事件必须带 sequence_number，且全程单调递增（严格 Responses 客户端要求）。
// 修复前 `emitOpenAIToResponsesToolCalls` 用 writeRealtimeEventJSONData 直发，不走 nextRealtimeSequence。
func TestInvariant_ResponsesOut_SequenceNumberMonotonic(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAIToolCallStream, "openai", "openai-res")

	var lastSeq = -1
	for i, ev := range events {
		seqRaw, ok := ev.data["sequence_number"]
		if !ok {
			t.Errorf("事件 #%d (%s) 缺 sequence_number 字段: %v", i, ev.typ(), ev.data)
			continue
		}
		seq, ok := seqRaw.(float64)
		if !ok {
			t.Errorf("事件 #%d (%s) sequence_number 不是数字: %v", i, ev.typ(), seqRaw)
			continue
		}
		if int(seq) < lastSeq {
			t.Errorf("事件 #%d (%s) sequence_number=%d 早于前一个 %d，不单调", i, ev.typ(), int(seq), lastSeq)
		}
		lastSeq = int(seq)
	}
}
