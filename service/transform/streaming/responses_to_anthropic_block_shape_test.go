package streaming

import (
	"testing"

	"github.com/qkf688/llmux/consts"
)

// Anthropic 的 content_block 契约要求块对象自带其类型字段：
// thinking 块必须有 thinking 与 signature（哪怕是空串）、tool_use 块必须有 input。
// 只发 {"type":"thinking"} 会让按 SDK 类型取值的客户端拿到 undefined——
// Claude Code 能容忍，严格客户端（官方 SDK 序列化、第三方渲染）不一定。
func TestResponsesToAnthropic_ThinkingBlockCarriesEmptyFields(t *testing.T) {
	events := runRealtimeTransform(t, responsesSSEThinkingThenText, consts.FormatOpenAIResponses, consts.FormatAnthropic)

	start := findEvent(t, events, "content_block_start")
	block := nestedMap(t, start.data, "content_block")
	if block["type"] != "thinking" {
		t.Fatalf("首个 content_block 应为 thinking（推理先于正文到达），实际 %v", block)
	}
	if think, ok := block["thinking"]; !ok || think != "" {
		t.Fatalf("thinking 块必须带 thinking 字段（可为空串），实际 %v", block)
	}
	if sig, ok := block["signature"]; !ok || sig != "" {
		t.Fatalf("thinking 块必须带 signature 字段（可为空串），实际 %v", block)
	}
}

// Anthropic 要求 content_block 的 index 从 0 起、按「发出顺序」严格递增。
// 上游把 reasoning 排在正文之前（DeepSeek 一类推理模型的常规形态）时，
// 早先的实现给了 thinking=1、text=0 的倒序索引——客户端按 index 组装 content
// 数组会把正文排到推理前面。
func TestResponsesToAnthropic_ContentBlockIndexFollowsEmissionOrder(t *testing.T) {
	events := runRealtimeTransform(t, responsesSSEThinkingThenText, consts.FormatOpenAIResponses, consts.FormatAnthropic)

	var starts []sseEvent
	for _, ev := range events {
		if ev.typ() == "content_block_start" {
			starts = append(starts, ev)
		}
	}
	if len(starts) != 2 {
		t.Fatalf("期望 2 个 content_block_start，实际 %d: %v", len(starts), eventTypes(events))
	}
	if got := starts[0].index(t); got != 0 {
		t.Fatalf("首个 content_block_start 的 index = %d，期望 0（必须从 0 起）", got)
	}
	if got := starts[1].index(t); got != 1 {
		t.Fatalf("第二个 content_block_start 的 index = %d，期望 1（必须递增）", got)
	}
	if block := nestedMap(t, starts[1].data, "content_block"); block["type"] != "text" {
		t.Fatalf("第二个块应为 text，实际 %v", block)
	}
}

// tool_use 块同样必须自带 input（Anthropic 契约里它是对象，增量由 input_json_delta 补）。
func TestResponsesToAnthropic_ToolUseBlockCarriesEmptyInput(t *testing.T) {
	events := runRealtimeTransform(t, responsesSSEToolCallOnly, consts.FormatOpenAIResponses, consts.FormatAnthropic)

	var toolBlock map[string]interface{}
	for _, ev := range events {
		if ev.typ() != "content_block_start" {
			continue
		}
		block := nestedMap(t, ev.data, "content_block")
		if block["type"] == "tool_use" {
			toolBlock = block
		}
	}
	if toolBlock == nil {
		t.Fatalf("未产出 tool_use content_block: %v", eventTypes(events))
	}
	if input, ok := toolBlock["input"].(map[string]interface{}); !ok || len(input) != 0 {
		t.Fatalf("tool_use 块必须带空 input 对象，实际 %v", toolBlock)
	}
}

// responsesSSEThinkingThenText 复刻「message 项先 added（首帧只带 role）、
// reasoning 项随后 added 并先出 delta、正文最后到达」的真实事件顺序。
const responsesSSEThinkingThenText = `event: response.created
data: {"type":"response.created","response":{"id":"resp_order","model":"m"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"id":"msg_order","type":"message","role":"assistant"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":1,"item":{"id":"reasoning_order","type":"reasoning"}}

event: response.reasoning_summary_text.delta
data: {"type":"response.reasoning_summary_text.delta","output_index":1,"item_id":"reasoning_order","summary_index":0,"delta":"先想"}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":1,"item":{"id":"reasoning_order","type":"reasoning"}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","output_index":0,"delta":"再答"}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"id":"msg_order","type":"message"}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_order","model":"m","status":"completed","output":[],"usage":{"input_tokens":30,"output_tokens":5,"total_tokens":35}}}

data: [DONE]
`

const responsesSSEToolCallOnly = `event: response.created
data: {"type":"response.created","response":{"id":"resp_tool","model":"m"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"get_weather","arguments":""}}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"city\":\"Beijing\"}"}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"get_weather","arguments":"{\"city\":\"Beijing\"}"}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_tool","model":"m","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":4,"total_tokens":14}}}

data: [DONE]
`
