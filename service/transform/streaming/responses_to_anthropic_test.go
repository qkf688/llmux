package streaming

import (
	"sort"
	"strings"
	"testing"
)

// fixtureResponsesParallelToolCalls 模拟 OpenAI Responses 流：两个 function_call 并行，
// args.delta 按 output_index 交错到达。这是 next-do 记录的 bug 复现输入：
// 旧代码在第二个 output_item.added 时关掉 tool#0 的 block，导致 tool#0 后续 args.delta
// 静默丢失。本 fixture 带 output_item.done，验证 done 驱动的顺序化切换路径。
const fixtureResponsesParallelToolCalls = `event: response.created
data: {"type":"response.created","response":{"id":"resp_par","status":"in_progress","model":"gpt-4"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"call_0","call_id":"call_0","name":"get_weather","arguments":""}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"get_time","arguments":""}}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"location\""}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"timezone\""}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":":\"Beijing\"}"}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":1,"delta":":\"UTC\"}"}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","id":"call_0","call_id":"call_0","name":"get_weather","arguments":"{\"location\":\"Beijing\"}","status":"completed"}}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"get_time","arguments":"{\"timezone\":\"UTC\"}","status":"completed"}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_par","status":"completed","model":"gpt-4","output":[{"type":"function_call","id":"call_0","name":"get_weather","arguments":"{\"location\":\"Beijing\"}"},{"type":"function_call","id":"call_1","name":"get_time","arguments":"{\"timezone\":\"UTC\"}"}],"usage":{"input_tokens":10,"output_tokens":8,"total_tokens":18}}}

data: [DONE]

`

// TestStreamConversion_ResponsesToAnthropic_ParallelToolCalls 覆盖 next-do 记录的 bug：
// 多 tool_call 并行时 args.delta 按 output_index 交错到达，旧代码在第二个 item.added
// 关掉 tool#0 block 导致 tool#0 后续 args.delta 静默丢失。
//
// 不变量：
//  1. 每个 content_block_delta 落在「已 start、未 stop」的 block 上（block 生命周期合法）
//  2. 两个 tool_call 的 args 完整到达（拼接 partial_json 等于期望值，无丢失）
//  3. tool_use block 的 id/name 正确
//  4. block 顺序：tool#0 block 先开先关，tool#1 block 后开后关（顺序化，不交错）
func TestStreamConversion_ResponsesToAnthropic_ParallelToolCalls(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureResponsesParallelToolCalls, "openai-res", "anthropic")

	openBlocks := map[int]string{}
	closedBlocks := map[int]bool{}
	var blockOpenOrder []int
	toolUseBlocks := map[int]string{} // blockIndex -> item id

	// 每个 tool_call 的 args 拼接
	argsByBlock := map[int]*strings.Builder{}
	blockToItemID := map[int]string{}

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
				blockToItemID[idx] = id
				argsByBlock[idx] = &strings.Builder{}
			}

		case "content_block_delta":
			idx := ev.index(t)
			bt, open := openBlocks[idx]
			if !open {
				t.Errorf("事件 #%d content_block_delta 落在未开的 block index %d 上", i, idx)
				continue
			}
			if closedBlocks[idx] {
				t.Errorf("事件 #%d content_block_delta 落在已 stop 的 block index %d 上", i, idx)
			}
			delta, _ := ev.data["delta"].(map[string]interface{})
			deltaType, _ := delta["type"].(string)
			if bt == "tool_use" && deltaType != "input_json_delta" {
				t.Errorf("事件 #%d tool_use block 上收到非 input_json_delta（%q）", i, deltaType)
			}
			if deltaType == "input_json_delta" {
				if pj, _ := delta["partial_json"].(string); pj != "" {
					if b, ok := argsByBlock[idx]; ok {
						b.WriteString(pj)
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

	// 不变量 3：tool_use block 数量应为 2
	if len(toolUseBlocks) != 2 {
		t.Errorf("应有 2 个 tool_use block，实际 %d: %v", len(toolUseBlocks), toolUseBlocks)
	}

	// 不变量 4：block 开块顺序——tool#0(call_0) 先开，tool#1(call_1) 后开
	if len(blockOpenOrder) != 2 {
		t.Errorf("应有 2 次 content_block_start，实际 %d 次: %v", len(blockOpenOrder), blockOpenOrder)
	} else {
		firstID := blockToItemID[blockOpenOrder[0]]
		secondID := blockToItemID[blockOpenOrder[1]]
		if firstID != "call_0" {
			t.Errorf("第一个开的 block 应是 call_0，实际 %q", firstID)
		}
		if secondID != "call_1" {
			t.Errorf("第二个开的 block 应是 call_1，实际 %q", secondID)
		}
	}

	// 不变量 2：两个 tool_call 的 args 完整到达
	wantArgsByItem := map[string]string{
		"call_0": `{"location":"Beijing"}`,
		"call_1": `{"timezone":"UTC"}`,
	}
	for idx, id := range blockToItemID {
		got := argsByBlock[idx].String()
		want := wantArgsByItem[id]
		if got != want {
			t.Errorf("tool_call %s args 不完整：got %q, want %q", id, got, want)
		}
	}
}

// fixtureResponsesParallelToolCallsNoDone 与 fixtureResponsesParallelToolCalls 相同的输入，
// 但省略 output_item.done，验证 completed 兜底 flush 路径（上游不发 done 时参数仍不丢）。
const fixtureResponsesParallelToolCallsNoDone = `event: response.created
data: {"type":"response.created","response":{"id":"resp_par2","status":"in_progress","model":"gpt-4"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"call_0","call_id":"call_0","name":"get_weather","arguments":""}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"get_time","arguments":""}}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"location\""}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"timezone\""}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":":\"Beijing\"}"}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":1,"delta":":\"UTC\"}"}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_par2","status":"completed","model":"gpt-4","output":[{"type":"function_call","id":"call_0","name":"get_weather","arguments":"{\"location\":\"Beijing\"}"},{"type":"function_call","id":"call_1","name":"get_time","arguments":"{\"timezone\":\"UTC\"}"}],"usage":{"input_tokens":10,"output_tokens":8,"total_tokens":18}}}

data: [DONE]

`

// TestStreamConversion_ResponsesToAnthropic_ParallelToolCallsNoDone 验证上游不发
// output_item.done 时，completed 兜底 flush 仍能保证两个 tool_call 的 args 完整到达。
func TestStreamConversion_ResponsesToAnthropic_ParallelToolCallsNoDone(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureResponsesParallelToolCallsNoDone, "openai-res", "anthropic")

	openBlocks := map[int]string{}
	closedBlocks := map[int]bool{}
	argsByBlock := map[int]*strings.Builder{}
	blockToItemID := map[int]string{}

	for i, ev := range events {
		switch ev.typ() {
		case "content_block_start":
			block, _ := ev.data["content_block"].(map[string]interface{})
			blockType, _ := block["type"].(string)
			idx := ev.index(t)
			if _, dup := openBlocks[idx]; dup {
				t.Errorf("事件 #%d content_block index %d 重复 start", i, idx)
			}
			openBlocks[idx] = blockType
			if blockType == "tool_use" {
				id, _ := block["id"].(string)
				blockToItemID[idx] = id
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
			delta, _ := ev.data["delta"].(map[string]interface{})
			if dt, _ := delta["type"].(string); dt == "input_json_delta" {
				if pj, _ := delta["partial_json"].(string); pj != "" {
					if b, ok := argsByBlock[idx]; ok {
						b.WriteString(pj)
					}
				}
			}

		case "content_block_stop":
			idx := ev.index(t)
			delete(openBlocks, idx)
			closedBlocks[idx] = true
		}
	}

	if len(openBlocks) != 0 {
		t.Errorf("流结束时仍有未关闭的 content_block: %v", openBlocks)
	}

	// 两个 tool_call 的 args 完整到达
	wantArgsByItem := map[string]string{
		"call_0": `{"location":"Beijing"}`,
		"call_1": `{"timezone":"UTC"}`,
	}
	if len(blockToItemID) != 2 {
		t.Errorf("应有 2 个 tool_use block，实际 %d: %v", len(blockToItemID), blockToItemID)
	}
	for idx, id := range blockToItemID {
		got := argsByBlock[idx].String()
		want := wantArgsByItem[id]
		if got != want {
			t.Errorf("tool_call %s args 不完整（兜底 flush）：got %q, want %q", id, got, want)
		}
	}
}

// fixtureResponsesReasoningContentInterleaved 模拟 reasoning 与 content 交错的 Responses 流：
// reasoning delta 与 output_text.delta 按 output_index 交错到达（content 先出后再插 reasoning）。
// 验证 thinking delta 与 text delta 都完整到达，block 生命周期合法。
const fixtureResponsesReasoningContentInterleaved = `event: response.created
data: {"type":"response.created","response":{"id":"resp_rc","status":"in_progress","model":"o1"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"type":"reasoning","id":"reasoning_1","status":"in_progress","summary":[]}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":1,"item":{"type":"message","id":"msg_1","status":"in_progress","role":"assistant","content":[]}}

event: response.reasoning_summary_text.delta
data: {"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"思考"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","output_index":1,"delta":"答案"}

event: response.reasoning_summary_text.delta
data: {"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"过程"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","output_index":1,"delta":"是"}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_rc","status":"completed","model":"o1","output":[{"type":"reasoning","id":"reasoning_1","summary":[{"type":"summary_text","text":"思考过程"}]},{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"答案是"}]}],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}

data: [DONE]

`

// TestStreamConversion_ResponsesToAnthropic_ReasoningContentInterleaved 覆盖场景 2：
// reasoning 与 content 交错到达（content 先出后再插 reasoning，上游协议异常）时，
// block 生命周期合法（不产出非法 SSE），text 完整到达。
// reasoning 被 text 切走后迟到的 delta 丢弃（thinking block 不重开，避免分裂），
// 这是上游异常的合理处理——优先保 text（最终输出）完整。
func TestStreamConversion_ResponsesToAnthropic_ReasoningContentInterleaved(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureResponsesReasoningContentInterleaved, "openai-res", "anthropic")

	openBlocks := map[int]string{}
	closedBlocks := map[int]bool{}

	var thinkingAll, textAll strings.Builder

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
				t.Errorf("事件 #%d content_block index %d 重复 start", i, idx)
			}
			openBlocks[idx] = blockType

		case "content_block_delta":
			idx := ev.index(t)
			if _, open := openBlocks[idx]; !open {
				t.Errorf("事件 #%d content_block_delta 落在未开的 block index %d 上", i, idx)
				continue
			}
			if closedBlocks[idx] {
				t.Errorf("事件 #%d content_block_delta 落在已 stop 的 block index %d 上", i, idx)
			}
			delta, _ := ev.data["delta"].(map[string]interface{})
			switch delta["type"] {
			case "text_delta":
				if s, _ := delta["text"].(string); s != "" {
					textAll.WriteString(s)
				}
			case "thinking_delta":
				if s, _ := delta["thinking"].(string); s != "" {
					thinkingAll.WriteString(s)
				}
			}

		case "content_block_stop":
			idx := ev.index(t)
			delete(openBlocks, idx)
			closedBlocks[idx] = true
		}
	}

	if len(openBlocks) != 0 {
		t.Errorf("流结束时仍有未关闭的 content_block: %v", openBlocks)
	}

	// text 完整到达（最终输出优先保）
	if got, want := textAll.String(), "答案是"; got != want {
		t.Errorf("text delta 不完整：got %q, want %q", got, want)
	}
	// thinking 只收到 text 切走前的部分（"思考"），切走后迟到的 "过程" 丢弃
	if got, want := thinkingAll.String(), "思考"; got != want {
		t.Errorf("thinking delta 应为切走前的部分 %q，got %q", want, got)
	}
}

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
	toolUseBlocks := map[int]string{}  // blockIndex -> item id
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

// TestStreamConversion_ResponsesToAnthropic_ParallelToolCalls_BlockOrderStrict 断言
// 并行 tool_call 的 block 不交错：tool#0 block 的 start..stop 区间与 tool#1 block 的
// start..stop 区间不重叠（tool#0 全部 stop 后 tool#1 才 start）。
func TestStreamConversion_ResponsesToAnthropic_ParallelToolCalls_BlockOrderStrict(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureResponsesParallelToolCalls, "openai-res", "anthropic")

	// 记录每个 block 的 [startPos, stopPos] 在事件序列中的位置
	type span struct {
		startPos, stopPos int
	}
	spans := map[int]*span{}
	for i, ev := range events {
		switch ev.typ() {
		case "content_block_start":
			idx := ev.index(t)
			if _, dup := spans[idx]; dup {
				t.Errorf("block %d 重复 start", idx)
			}
			spans[idx] = &span{startPos: i, stopPos: -1}
		case "content_block_stop":
			idx := ev.index(t)
			if s, ok := spans[idx]; ok {
				s.stopPos = i
			}
		}
	}

	// 收集所有 span，按 startPos 排序
	type blockSpan struct {
		idx  int
		span span
	}
	var ordered []blockSpan
	for idx, s := range spans {
		if s.stopPos < 0 {
			t.Errorf("block %d 未 stop", idx)
			continue
		}
		ordered = append(ordered, blockSpan{idx, *s})
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].span.startPos < ordered[j].span.startPos
	})

	// 相邻 block 的区间不重叠：前一个的 stopPos < 后一个的 startPos
	for k := 1; k < len(ordered); k++ {
		if ordered[k-1].span.stopPos >= ordered[k].span.startPos {
			t.Errorf("block %d 的 stop（pos %d）未早于 block %d 的 start（pos %d），block 交错",
				ordered[k-1].idx, ordered[k-1].span.stopPos, ordered[k].idx, ordered[k].span.startPos)
		}
	}
}

// fixtureResponsesThreeParallelToolCalls 模拟 3 个并行 function_call，output_index 0/1/2，
// args.delta 按 output_index 交错到达，带 output_item.done。
// 用于覆盖审查 S-3：3+ 并行时 openNextPendingAnthropicBlock 的「遍历找最小未 started
// 且有缓冲的 output_index」逻辑能否正确排序选择。
const fixtureResponsesThreeParallelToolCalls = `event: response.created
data: {"type":"response.created","response":{"id":"resp_3","status":"in_progress","model":"gpt-4"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"call_0","call_id":"call_0","name":"get_weather","arguments":""}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"get_time","arguments":""}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","id":"call_2","call_id":"call_2","name":"get_status","arguments":""}}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"loc\""}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"tz\""}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":2,"delta":"{\"svc\""}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":":\"A\"}"}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":1,"delta":":\"B\"}"}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":2,"delta":":\"C\"}"}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","id":"call_0","call_id":"call_0","name":"get_weather","arguments":"{\"loc\":\"A\"}","status":"completed"}}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"get_time","arguments":"{\"tz\":\"B\"}","status":"completed"}}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":2,"item":{"type":"function_call","id":"call_2","call_id":"call_2","name":"get_status","arguments":"{\"svc\":\"C\"}","status":"completed"}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_3","status":"completed","model":"gpt-4","output":[{"type":"function_call","id":"call_0","name":"get_weather","arguments":"{\"loc\":\"A\"}"},{"type":"function_call","id":"call_1","name":"get_time","arguments":"{\"tz\":\"B\"}"},{"type":"function_call","id":"call_2","name":"get_status","arguments":"{\"svc\":\"C\"}"}],"usage":{"input_tokens":15,"output_tokens":12,"total_tokens":27}}}

data: [DONE]

`

// TestStreamConversion_ResponsesToAnthropic_ThreeParallelToolCalls 覆盖审查 S-3：
// 3+ 并行 tool_call 时 openNextPendingAnthropicBlock 的排序/选择逻辑。
//
// 不变量：
//  1. 3 个 tool_call 的 args 都完整到达
//  2. block 开块顺序按 output_index 升序（0→1→2）
//  3. block 区间不交错（参考 _BlockOrderStrict 的 span 断言模式）
func TestStreamConversion_ResponsesToAnthropic_ThreeParallelToolCalls(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureResponsesThreeParallelToolCalls, "openai-res", "anthropic")

	openBlocks := map[int]string{}
	closedBlocks := map[int]bool{}
	var blockOpenOrder []int
	toolUseBlocks := map[int]string{}
	argsByBlock := map[int]*strings.Builder{}

	// span 追踪用于不交错断言
	type span struct {
		startPos, stopPos int
	}
	spans := map[int]*span{}

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
				t.Errorf("事件 #%d content_block index %d 重复 start", i, idx)
			}
			if _, dup := spans[idx]; dup {
				t.Errorf("block %d 重复 start", idx)
			}
			openBlocks[idx] = blockType
			blockOpenOrder = append(blockOpenOrder, idx)
			spans[idx] = &span{startPos: i, stopPos: -1}
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
			if s, ok := spans[idx]; ok {
				s.stopPos = i
			}
		}
	}

	if len(openBlocks) != 0 {
		t.Errorf("流结束时仍有未关闭的 content_block: %v", openBlocks)
	}

	// 不变量 1：3 个 tool_use block
	if len(toolUseBlocks) != 3 {
		t.Errorf("应有 3 个 tool_use block，实际 %d: %v", len(toolUseBlocks), toolUseBlocks)
	}

	// 不变量 2：block 开块顺序 0→1→2（按 output_index 升序）
	wantOrder := []string{"call_0", "call_1", "call_2"}
	if len(blockOpenOrder) != 3 {
		t.Errorf("应有 3 次 content_block_start，实际 %d 次: %v", len(blockOpenOrder), blockOpenOrder)
	} else {
		for k, wantID := range wantOrder {
			gotID := toolUseBlocks[blockOpenOrder[k]]
			if gotID != wantID {
				t.Errorf("开块顺序第 %d 个应是 %q，实际 %q", k, wantID, gotID)
			}
		}
	}

	// 不变量 3：block 区间不交错
	type blockSpan struct {
		idx  int
		span span
	}
	var ordered []blockSpan
	for idx, s := range spans {
		if s.stopPos < 0 {
			t.Errorf("block %d 未 stop", idx)
			continue
		}
		ordered = append(ordered, blockSpan{idx, *s})
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].span.startPos < ordered[j].span.startPos
	})
	for k := 1; k < len(ordered); k++ {
		if ordered[k-1].span.stopPos >= ordered[k].span.startPos {
			t.Errorf("block %d 的 stop（pos %d）未早于 block %d 的 start（pos %d），block 交错",
				ordered[k-1].idx, ordered[k-1].span.stopPos, ordered[k].idx, ordered[k].span.startPos)
		}
	}

	// 不变量 1 续：3 个 tool_call 的 args 完整
	wantArgsByItem := map[string]string{
		"call_0": `{"loc":"A"}`,
		"call_1": `{"tz":"B"}`,
		"call_2": `{"svc":"C"}`,
	}
	for idx, id := range toolUseBlocks {
		got := argsByBlock[idx].String()
		want := wantArgsByItem[id]
		if got != want {
			t.Errorf("tool_call %s args 不完整：got %q, want %q", id, got, want)
		}
	}
}

// fixtureResponsesImageInterleavedWithToolCalls 模拟两个并行 function_call，
// 中途插入一个 response.content_part.added（image_url），image 之后继续有 tool_call 的 args.delta。
// 用于覆盖审查 S-4 / 遗漏场景-1：验证 W-2 修复后 image 不再破坏 active 状态。
const fixtureResponsesImageInterleavedWithToolCalls = `event: response.created
data: {"type":"response.created","response":{"id":"resp_img","status":"in_progress","model":"gpt-4"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"call_0","call_id":"call_0","name":"get_weather","arguments":""}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"get_time","arguments":""}}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"loc\""}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"tz\""}

event: response.content_part.added
data: {"type":"response.content_part.added","output_index":0,"part":{"type":"image_url","image_url":"data:image/png;base64,iVBORw0KGgo="}}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":0,"delta":":\"Beijing\"}"}

event: response.function_call_arguments.delta
data: {"type":"response.function_call_arguments.delta","output_index":1,"delta":":\"UTC\"}"}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","id":"call_0","call_id":"call_0","name":"get_weather","arguments":"{\"loc\":\"Beijing\"}","status":"completed"}}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","id":"call_1","call_id":"call_1","name":"get_time","arguments":"{\"tz\":\"UTC\"}","status":"completed"}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_img","status":"completed","model":"gpt-4","output":[{"type":"function_call","id":"call_0","name":"get_weather","arguments":"{\"loc\":\"Beijing\"}"},{"type":"function_call","id":"call_1","name":"get_time","arguments":"{\"tz\":\"UTC\"}"}],"usage":{"input_tokens":10,"output_tokens":8,"total_tokens":18}}}

data: [DONE]

`

// TestStreamConversion_ResponsesToAnthropic_ImageInterleavedWithToolCalls 覆盖审查 S-4：
// image 穿插并行 tool_call 时，W-2 修复（image handler 关 block 后补 activeOutputIndex=-1）
// 是否保证后续 delta 不落在已 stop 的 block 上。
//
// 不变量：
//  1. image 前的 tool_call args 完整
//  2. image 后的 tool_call args 完整
//  3. image block 合法 start/stop
//  4. 全程无 delta 落在已 stop 的 block 上
func TestStreamConversion_ResponsesToAnthropic_ImageInterleavedWithToolCalls(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureResponsesImageInterleavedWithToolCalls, "openai-res", "anthropic")

	openBlocks := map[int]string{}
	closedBlocks := map[int]bool{}
	toolUseBlocks := map[int]string{}   // blockIndex -> item id
	argsByBlock := map[int]*strings.Builder{}
	imageBlockIndex := -1
	var blockOpenOrder []int

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
				toolUseBlocks[idx] = id
				argsByBlock[idx] = &strings.Builder{}
			}
			if blockType == "image" && imageBlockIndex < 0 {
				imageBlockIndex = idx
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
		}
	}

	if len(openBlocks) != 0 {
		t.Errorf("流结束时仍有未关闭的 content_block: %v", openBlocks)
	}

	// 不变量 3：image block 合法 start/stop
	if imageBlockIndex < 0 {
		t.Errorf("未找到 image block")
	} else if !closedBlocks[imageBlockIndex] {
		t.Errorf("image block index %d 未 stop", imageBlockIndex)
	}

	// 不变量 4：全程无 delta 落在已 stop 的 block 上
	// （上面的 content_block_delta 分支已逐事件断言，此处不重复）

	// 不变量 1 & 2：两个 tool_call 的 args 完整（image 打断后 args 可能跨多个 block，
	// 按 tool_call id + block 开块顺序聚合）。image 穿插并行 tool_call 是上游协议异常，
	// args 分裂到两个 block 不可避免（image 前的 args 已发出），但不应丢失。
	type blockArgs struct {
		blockIdx int
		itemID   string
		args     string
	}
	var blockArgsInOrder []blockArgs
	for _, idx := range blockOpenOrder {
		if id, ok := toolUseBlocks[idx]; ok {
			blockArgsInOrder = append(blockArgsInOrder, blockArgs{idx, id, argsByBlock[idx].String()})
		}
	}
	argsByItemID := map[string]*strings.Builder{}
	itemIDs := map[string]bool{}
	for _, ba := range blockArgsInOrder {
		if _, ok := argsByItemID[ba.itemID]; !ok {
			argsByItemID[ba.itemID] = &strings.Builder{}
		}
		argsByItemID[ba.itemID].WriteString(ba.args)
		itemIDs[ba.itemID] = true
	}

	wantArgsByItem := map[string]string{
		"call_0": `{"loc":"Beijing"}`,
		"call_1": `{"tz":"UTC"}`,
	}
	if len(itemIDs) != 2 {
		t.Errorf("应有 2 个 tool_call（call_0/call_1），实际 %d: %v", len(itemIDs), itemIDs)
	}
	for id, want := range wantArgsByItem {
		b, ok := argsByItemID[id]
		if !ok {
			t.Errorf("tool_call %s 未出现", id)
			continue
		}
		if got := b.String(); got != want {
			t.Errorf("tool_call %s args 不完整（跨 block 聚合）：got %q, want %q", id, got, want)
		}
	}
}
