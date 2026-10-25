package streaming

import (
	"testing"
)

// 本文件覆盖 anthropic → responses 转换对「空名 tool_use」的行为。
//
// Anthropic 协议要求 content_block_start 单帧携带 tool_use 的完整 id/name；
// 正常上游不会缺 name，但不守协议的上游（与 openai→responses 线同类故障的
// anthropic 端点）会产出 name:"" 的 tool_use。空名 function_call 转发给客户端
// 会报 No such tool 并写脏历史，与 openai 线修复前同构，故整条丢弃并 warn。
//
// 与 openai 线的差异：anthropic 的 id/name 不分帧到达，无需缓冲，
// 在 content_block_start 源头丢弃即可；后续该 block 的 input_json_delta
// 也必须拦掉（不产出孤立参数 delta），收尾事件不涉及该 block。

// fixtureAnthropicEmptyNameToolUse：空名 tool_use + 参数增量 + stop_reason=tool_use。
const fixtureAnthropicEmptyNameToolUse = `event: message_start
data: {"type":"message_start","message":{"id":"msg_emptytool","type":"message","role":"assistant","content":[],"model":"test-model","stop_reason":null,"usage":{"input_tokens":5,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_dirty","name":"","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"location\":\"Beijing\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":5,"output_tokens":3}}

event: message_stop
data: {"type":"message_stop"}

`

// fixtureAnthropicNamedToolUse：正常命名 tool_use，回归基线——行为不得被本次改动波及。
const fixtureAnthropicNamedToolUse = `event: message_start
data: {"type":"message_start","message":{"id":"msg_namedtool","type":"message","role":"assistant","content":[],"model":"test-model","stop_reason":null,"usage":{"input_tokens":5,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_ok","name":"get_weather","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"location\":\"Beijing\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":5,"output_tokens":3}}

event: message_stop
data: {"type":"message_stop"}

`

// TestAnthropicToolUse_EmptyNameDropped：空名 tool_use 不产出任何 function_call
// 事件，也不给 completed 的 output 留下空名项。
func TestAnthropicToolUse_EmptyNameDropped(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureAnthropicEmptyNameToolUse, "anthropic", "openai-res")

	for _, ev := range events {
		switch ev.typ() {
		case "response.output_item.added", "response.output_item.done",
			"response.function_call_arguments.delta", "response.function_call_arguments.done":
			if item, ok := ev.data["item"].(map[string]interface{}); ok {
				if itemType, _ := item["type"].(string); itemType == "function_call" {
					t.Errorf("空名 tool_use 不应产出 function_call 的 %s: %v", ev.typ(), ev.data)
				}
			}
			if name, ok := ev.data["item_id"]; ok && name == "toolu_dirty" {
				t.Errorf("空名 tool_use 的收尾事件不应出现（item_id=toolu_dirty）: %v", ev.data)
			}
		}
	}

	// completed 仍要正常发出（流收尾不受丢弃影响），且 output 里无 function_call。
	last := events[len(events)-1]
	if last.typ() != "response.completed" {
		t.Fatalf("最后一个事件是 %q，应为 response.completed；事件序列: %v", last.typ(), eventTypes(events))
	}
	resp, _ := last.data["response"].(map[string]interface{})
	output, _ := resp["output"].([]interface{})
	for _, it := range output {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		if itemType, _ := m["type"].(string); itemType == "function_call" {
			t.Errorf("response.completed 的 output 不应包含空名 function_call: %v", m)
		}
	}
}

// TestAnthropicToolUse_NamedSurvives：正常命名 tool_use 的 added/参数/收尾/出参
// 全链路保留（回归基线）。
func TestAnthropicToolUse_NamedSurvives(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureAnthropicNamedToolUse, "anthropic", "openai-res")

	_, item := functionCallAdded(t, events)
	if item == nil {
		t.Fatalf("正常 tool_use 应产出 function_call 的 added；事件序列: %v", eventTypes(events))
	}
	if name, _ := item["name"].(string); name != "get_weather" {
		t.Errorf("added 的 name 应为 get_weather，实际 %q", name)
	}
	if id, _ := item["id"].(string); id != "toolu_ok" {
		t.Errorf("added 的 id 应透传 toolu_ok，实际 %q", id)
	}

	argsDone := findEvent(t, events, "response.function_call_arguments.done")
	if args, _ := argsDone.data["arguments"].(string); args != `{"location":"Beijing"}` {
		t.Errorf("arguments.done 应为完整参数，实际 %q", args)
	}

	last := events[len(events)-1]
	if last.typ() != "response.completed" {
		t.Fatalf("最后一个事件是 %q，应为 response.completed", last.typ())
	}
	resp, _ := last.data["response"].(map[string]interface{})
	output, _ := resp["output"].([]interface{})
	found := false
	for _, it := range output {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		if itemType, _ := m["type"].(string); itemType == "function_call" {
			found = true
		}
	}
	if !found {
		t.Error("response.completed 的 output 应包含 function_call 项")
	}
}
