package streaming

import (
	"strings"
	"testing"
)

// fixtureResponsesParallelToolCallsEmptyArgs 模拟两个并行 function_call，
// tool#0 有 args.delta，tool#1 空 args（无任何 function_call_arguments.delta）。
// 用于评估 next-do 记录的遗漏场景-3：并行场景下空 args 的 tool_call 是否丢失
// content_block_start（item.added 排队不开 block，done/completed 兜底因 buffer 为空跳过）。
const fixtureResponsesParallelToolCallsEmptyArgs = `event: response.created
data: {"type":"response.created","response":{"id":"resp_empty","status":"in_progress","model":"gpt-4"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"call_0","call_id":"call_0","name":"get_weather","arguments":""}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"noop","arguments":""}}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"location\":\"Beijing\"}"}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","id":"call_0","call_id":"call_0","name":"get_weather","arguments":"{\"location\":\"Beijing\"}","status":"completed"}}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"noop","arguments":"","status":"completed"}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_empty","status":"completed","model":"gpt-4","output":[{"type":"function_call","id":"call_0","name":"get_weather","arguments":"{\"location\":\"Beijing\"}"},{"type":"function_call","id":"call_1","name":"noop","arguments":""}],"usage":{"input_tokens":10,"output_tokens":8,"total_tokens":18}}}

data: [DONE]

`

// TestStreamConversion_ResponsesToAnthropic_ParallelToolCallsEmptyArgs 覆盖遗漏场景-3：
// 并行场景下空 args 的 tool_call（tool#1 无任何 function_call_arguments.delta）仍能产出
// 合法的 content_block_start/stop，不丢失该 tool_call。
//
// 不变量：
//  1. 两个 tool_use block 都合法 start/stop（含空 args 的 tool#1）
//  2. tool#0 的 args 完整到达
//  3. tool#1 的 block 无 delta（空 args）
//  4. block 顺序化不交错（tool#0 全部 stop 后 tool#1 才 start）
//  5. tool#1 的 content_block_start 带 id/name（noop）
func TestStreamConversion_ResponsesToAnthropic_ParallelToolCallsEmptyArgs(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureResponsesParallelToolCallsEmptyArgs, "openai-res", "anthropic")

	openBlocks := map[int]string{}
	closedBlocks := map[int]bool{}
	var blockOpenOrder []int
	toolUseBlocks := map[int]string{} // blockIndex -> item id
	toolUseNames := map[int]string{}  // blockIndex -> item name
	argsByBlock := map[int]*strings.Builder{}
	deltaCountByBlock := map[int]int{}

	for i, ev := range events {
		switch ev.typ() {
		case "content_block_start":
			block, ok := ev.data["content_block"].(map[string]interface{})
			if !ok {
				t.Fatalf("事件 #%d content_block_start 缺少 content_block 字段: %v", i, ev.data)
			}
			blockType, _ := block["type"].(string)
			if !legalAnthropicBlockTypes[blockType] {
				t.Errorf("事件 #%d 非法 content_block 类型 %q", i, blockType)
			}
			idx := ev.index(t)
			if _, dup := openBlocks[idx]; dup {
				t.Errorf("事件 #%d content_block index %d 重复 start（未 stop 就再开）", i, idx)
			}
			openBlocks[idx] = blockType
			blockOpenOrder = append(blockOpenOrder, idx)
			if blockType == "tool_use" {
				id, _ := block["id"].(string)
				name, _ := block["name"].(string)
				if id == "" {
					t.Errorf("事件 #%d tool_use block 丢失 id", i)
				}
				if name == "" {
					t.Errorf("事件 #%d tool_use block 丢失 name", i)
				}
				toolUseBlocks[idx] = id
				toolUseNames[idx] = name
				argsByBlock[idx] = &strings.Builder{}
			}

		case "content_block_delta":
			idx := ev.index(t)
			if _, open := openBlocks[idx]; !open {
				t.Errorf("事件 #%d content_block_delta 落在未开的 block index %d 上", i, idx)
				continue
			}
			if closedBlocks[idx] {
				t.Errorf("事件 #%d content_block_delta 落在已 stop 的 block index %d 上", i, idx)
			}
			deltaCountByBlock[idx]++
			if delta, _ := ev.data["delta"].(map[string]interface{}); delta != nil {
				if dt, _ := delta["type"].(string); dt == "input_json_delta" {
					if pj, _ := delta["partial_json"].(string); pj != "" {
						if b, ok := argsByBlock[idx]; ok {
							b.WriteString(pj)
						}
					}
				}
			}

		case "content_block_stop":
			idx := ev.index(t)
			if _, open := openBlocks[idx]; !open {
				t.Errorf("事件 #%d content_block_stop index %d 没有对应的 start", i, idx)
			}
			delete(openBlocks, idx)
			closedBlocks[idx] = true
		}
	}

	if len(openBlocks) != 0 {
		t.Errorf("流结束时仍有未关闭的 content_block: %v", openBlocks)
	}

	// 不变量 1：两个 tool_use block
	if len(toolUseBlocks) != 2 {
		t.Errorf("应有 2 个 tool_use block（含空 args 的 tool#1），实际 %d: %v", len(toolUseBlocks), toolUseBlocks)
	}

	// 不变量 4：block 顺序——tool#0(call_0) 先开，tool#1(call_1) 后开
	if len(blockOpenOrder) != 2 {
		t.Errorf("应有 2 次 content_block_start，实际 %d 次: %v", len(blockOpenOrder), blockOpenOrder)
	} else {
		firstID := toolUseBlocks[blockOpenOrder[0]]
		secondID := toolUseBlocks[blockOpenOrder[1]]
		if firstID != "call_0" {
			t.Errorf("第一个开的 block 应是 call_0，实际 %q", firstID)
		}
		if secondID != "call_1" {
			t.Errorf("第二个开的 block 应是 call_1，实际 %q", secondID)
		}
	}

	// 不变量 2：tool#0 args 完整
	for idx, id := range toolUseBlocks {
		if id == "call_0" {
			if got, want := argsByBlock[idx].String(), `{"location":"Beijing"}`; got != want {
				t.Errorf("tool#0 args 不完整：got %q, want %q", got, want)
			}
		}
	}

	// 不变量 3：tool#1 空 args（无 delta）
	for idx, id := range toolUseBlocks {
		if id == "call_1" {
			if deltaCountByBlock[idx] != 0 {
				t.Errorf("tool#1（空 args）不应有 delta，实际 %d 条", deltaCountByBlock[idx])
			}
			if got := argsByBlock[idx].String(); got != "" {
				t.Errorf("tool#1（空 args）args 应为空，实际 %q", got)
			}
		}
	}

	// 不变量 5：tool#1 的 block name 是 noop
	for idx, id := range toolUseBlocks {
		if id == "call_1" {
			if got, want := toolUseNames[idx], "noop"; got != want {
				t.Errorf("tool#1 block name 应为 %q，实际 %q", want, got)
			}
		}
	}
}

// fixtureResponsesSingleToolCall 单个 function_call 经 responses→anthropic 路由：
// item.added → args.delta*N → output_item.done → completed。
// 用于回归覆盖遗漏场景-2：修复后单 tool_call 的代码路径从旧的「item.added 时关前一个 block」
// 变为新的「item.added 有 active 时排队不关，done 驱动切换」，需确认单 tool_call 仍正常。
const fixtureResponsesSingleToolCall = `event: response.created
data: {"type":"response.created","response":{"id":"resp_single","status":"in_progress","model":"gpt-4"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"call_s","call_id":"call_s","name":"get_weather","arguments":""}}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"location\""}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":":\"Shanghai\"}"}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","id":"call_s","call_id":"call_s","name":"get_weather","arguments":"{\"location\":\"Shanghai\"}","status":"completed"}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_single","status":"completed","model":"gpt-4","output":[{"type":"function_call","id":"call_s","name":"get_weather","arguments":"{\"location\":\"Shanghai\"}"}],"usage":{"input_tokens":5,"output_tokens":4,"total_tokens":9}}}

data: [DONE]

`

// TestStreamConversion_ResponsesToAnthropic_SingleToolCall 回归覆盖遗漏场景-2：
// 修复后单 tool_call 的代码路径变更（item.added 不再关前一个 block，done 驱动切换），
// 确认单 tool_call 仍正常——block 合法 start/delta/stop，args 完整，block 在 done 时 stop。
//
// 不变量：
//  1. 1 个 tool_use block，合法 start/delta/stop
//  2. args 完整到达（拼接 partial_json 等于期望值）
//  3. block 在 output_item.done 时 stop（非 completed 兜底 stop）——即 completed 之前已 stop
//  4. stop_reason=tool_use
func TestStreamConversion_ResponsesToAnthropic_SingleToolCall(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureResponsesSingleToolCall, "openai-res", "anthropic")

	openBlocks := map[int]string{}
	closedBlocks := map[int]bool{}
	argsByBlock := map[int]*strings.Builder{}
	toolUseBlocks := map[int]string{}
	var blockStopPos, completedPos int = -1, -1

	for i, ev := range events {
		switch ev.typ() {
		case "content_block_start":
			block, _ := ev.data["content_block"].(map[string]interface{})
			blockType, _ := block["type"].(string)
			if !legalAnthropicBlockTypes[blockType] {
				t.Errorf("事件 #%d 非法 content_block 类型 %q", i, blockType)
			}
			idx := ev.index(t)
			if _, dup := openBlocks[idx]; dup {
				t.Errorf("事件 #%d content_block index %d 重复 start", i, idx)
			}
			openBlocks[idx] = blockType
			if blockType == "tool_use" {
				id, _ := block["id"].(string)
				toolUseBlocks[idx] = id
				argsByBlock[idx] = &strings.Builder{}
			}
		case "content_block_delta":
			idx := ev.index(t)
			if _, open := openBlocks[idx]; !open {
				t.Errorf("事件 #%d content_block_delta 落在未开的 block index %d 上", i, idx)
				continue
			}
			if closedBlocks[idx] {
				t.Errorf("事件 #%d content_block_delta 落在已 stop 的 block index %d 上", i, idx)
			}
			if delta, _ := ev.data["delta"].(map[string]interface{}); delta != nil {
				if dt, _ := delta["type"].(string); dt == "input_json_delta" {
					if pj, _ := delta["partial_json"].(string); pj != "" {
						if b, ok := argsByBlock[idx]; ok {
							b.WriteString(pj)
						}
					}
				}
			}
		case "content_block_stop":
			idx := ev.index(t)
			if _, open := openBlocks[idx]; !open {
				t.Errorf("事件 #%d content_block_stop index %d 没有对应的 start", i, idx)
			}
			delete(openBlocks, idx)
			closedBlocks[idx] = true
			blockStopPos = i
		case "message_delta":
			// message_delta 在 completed 之后发出，用 message_delta 标记 completed 已处理
			if completedPos < 0 {
				completedPos = i
			}
			// 不变量 4：stop_reason=tool_use
			if delta, ok := ev.data["delta"].(map[string]interface{}); ok {
				if sr, _ := delta["stop_reason"].(string); sr != "tool_use" {
					t.Errorf("stop_reason 应为 tool_use，实际 %q", sr)
				}
			}
		}
	}

	if len(openBlocks) != 0 {
		t.Errorf("流结束时仍有未关闭的 content_block: %v", openBlocks)
	}

	// 不变量 1：1 个 tool_use block
	if len(toolUseBlocks) != 1 {
		t.Errorf("应有 1 个 tool_use block，实际 %d: %v", len(toolUseBlocks), toolUseBlocks)
	}

	// 不变量 2：args 完整
	for idx, id := range toolUseBlocks {
		if id != "call_s" {
			t.Errorf("tool_use block id 应为 call_s，实际 %q", id)
		}
		if got, want := argsByBlock[idx].String(), `{"location":"Shanghai"}`; got != want {
			t.Errorf("tool_call args 不完整：got %q, want %q", got, want)
		}
	}

	// 不变量 3：block 在 done 时 stop（早于 completed 阶段的 message_delta）
	if blockStopPos < 0 {
		t.Errorf("tool_use block 未 stop")
	} else if completedPos >= 0 && blockStopPos >= completedPos {
		t.Errorf("block stop（pos %d）应在 completed 阶段（pos %d）之前，实际在之后或同级", blockStopPos, completedPos)
	}
}
