package streaming

import (
	"sort"
	"strings"
	"testing"
)

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
