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
