package streaming

import "testing"

// 本文件钉死 Responses 流式的「每个 output item 必须被关掉」这条协议不变量。
//
// 背景（2026-10-22 生产实测，chat_logs#32826 / deepseek-harness 报
// 「stream completed with an unfinished tool call: run_code」）：openai(Chat) →
// openai-res 的流式转换只给 message 项发了 response.output_item.done，
// function_call 项只有 output_item.added + 参数 delta，缺
// response.function_call_arguments.done 与该项的 output_item.done。严格 Responses
// 客户端按「开了没关」判未完成，直接让本轮运行失败——上游 Chat 流本身是完整的
// （finish_reason=tool_calls、id/name/arguments 齐全），锅在转换层。
//
// 判据：
//   - 每个 function_call 类型的 output_item.added，必须存在同 output_index 的
//     response.output_item.done（item.status=completed）与配套的
//     response.function_call_arguments.done（arguments 等于 delta 拼回的完整串）；
//   - reasoning 等其它类型的项同样必须有 output_item.done（OpenAI 线此前漏发，
//     Anthropic 线有——同一条不变量，两条线不能分叉）；
//   - 一条流只开一次 message 项（上游末包重复带 role 时不得重开）。

// assertFunctionCallItemsClosed 断言流里每个 function_call 项都有配对的收尾事件。
func assertFunctionCallItemsClosed(t *testing.T, events []sseEvent, wantArgs string) {
	t.Helper()

	added := map[int]bool{}
	done := map[int]map[string]interface{}{}
	argsDone := map[int]string{}
	for _, ev := range events {
		switch ev.typ() {
		case "response.output_item.added":
			item, _ := ev.data["item"].(map[string]interface{})
			if item == nil {
				continue
			}
			if itemType, _ := item["type"].(string); itemType == "function_call" {
				added[ev.outputIndex(t)] = true
			}
		case "response.output_item.done":
			item, _ := ev.data["item"].(map[string]interface{})
			if item == nil {
				continue
			}
			if itemType, _ := item["type"].(string); itemType == "function_call" {
				done[ev.outputIndex(t)] = item
			}
		case "response.function_call_arguments.done":
			args, _ := ev.data["arguments"].(string)
			argsDone[ev.outputIndex(t)] = args
		}
	}

	if len(added) == 0 {
		t.Fatalf("流里没有 function_call 类型的 output_item.added: %v", eventTypes(events))
	}
	for idx := range added {
		item, ok := done[idx]
		if !ok {
			t.Errorf("output_index=%d 的 function_call 缺 response.output_item.done（客户端据此判「工具调用未完成」）", idx)
			continue
		}
		if status, _ := item["status"].(string); status != "completed" {
			t.Errorf("output_index=%d 的 function_call 终态 status=%q，应为 completed", idx, status)
		}
		gotArgs, ok := argsDone[idx]
		if !ok {
			t.Errorf("output_index=%d 缺 response.function_call_arguments.done", idx)
			continue
		}
		if gotArgs != wantArgs {
			t.Errorf("output_index=%d 的 arguments.done=%q，应为 %q", idx, gotArgs, wantArgs)
		}
	}
}

// assertItemClosed 断言指定类型的 output item 都被关掉了（存在同 index 的
// response.output_item.done，且终态 status=completed）。
func assertItemClosed(t *testing.T, events []sseEvent, wantType string) {
	t.Helper()

	added := map[int]bool{}
	done := map[int]map[string]interface{}{}
	for _, ev := range events {
		switch ev.typ() {
		case "response.output_item.added":
			item, _ := ev.data["item"].(map[string]interface{})
			if item == nil {
				continue
			}
			if itemType, _ := item["type"].(string); itemType == wantType {
				added[ev.outputIndex(t)] = true
			}
		case "response.output_item.done":
			item, _ := ev.data["item"].(map[string]interface{})
			if item == nil {
				continue
			}
			if itemType, _ := item["type"].(string); itemType == wantType {
				done[ev.outputIndex(t)] = item
			}
		}
	}

	if len(added) == 0 {
		t.Fatalf("流里没有 %s 类型的 output_item.added: %v", wantType, eventTypes(events))
	}
	for idx := range added {
		item, ok := done[idx]
		if !ok {
			t.Errorf("output_index=%d 的 %s 缺 response.output_item.done（客户端据此判「项未完成」）", idx, wantType)
			continue
		}
		if status, _ := item["status"].(string); status != "completed" {
			t.Errorf("output_index=%d 的 %s 终态 status=%q，应为 completed", idx, wantType, status)
		}
	}
}

// TestResponsesOut_OpenAIToolCallItemClosed 覆盖 openai(Chat) → openai-res：
// Chat 的 tool_calls 转成 Responses 的 function_call 项后必须被收尾。
func TestResponsesOut_OpenAIToolCallItemClosed(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAIToolCallStream, "openai", "openai-res")
	assertFunctionCallItemsClosed(t, events, `{"location":"Beijing"}`)
}

// TestResponsesOut_AnthropicToolUseItemClosed 覆盖 anthropic → openai-res：
// tool_use 块转成 function_call 项后同样必须被收尾（与 OpenAI 线同一条不变量）。
func TestResponsesOut_AnthropicToolUseItemClosed(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureAnthropicToolUseStream, "anthropic", "openai-res")
	assertFunctionCallItemsClosed(t, events, `{"location":"Beijing"}`)
}

// TestResponsesOut_OpenAIReasoningItemClosed 覆盖 openai(Chat) → openai-res：
// reasoning_content 转成 reasoning 项后也必须被收尾。此前 OpenAI 线只给 message 项
// 发过 output_item.done，reasoning 项「开了没关」——与 Anthropic 线不对称。
func TestResponsesOut_OpenAIReasoningItemClosed(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAIReasoningToolCallStream, "openai", "openai-res")
	assertItemClosed(t, events, "reasoning")
}

// TestResponsesOut_OpenAIRepeatedRoleOpensMessageOnce 覆盖上游末包重复带 role 的情形：
// 生产流（chat_logs#32826）的最后一帧同时带 role=assistant 与 finish_reason=tool_calls，
// 若不拦就会把 response.created / response.in_progress / output_item.added(message)
// 全套再发一遍——客户端看到同一 response id 与同一 item id 各出现两次。
func TestResponsesOut_OpenAIRepeatedRoleOpensMessageOnce(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAIReasoningToolCallStream, "openai", "openai-res")

	created := 0
	messageAdded := 0
	for _, ev := range events {
		switch ev.typ() {
		case "response.created":
			created++
		case "response.output_item.added":
			item, _ := ev.data["item"].(map[string]interface{})
			if item == nil {
				continue
			}
			if itemType, _ := item["type"].(string); itemType == "message" {
				messageAdded++
			}
		}
	}

	if created != 1 {
		t.Errorf("一条流只应发一次 response.created，实际 %d 次（上游末包重复带 role 时被重复开流）", created)
	}
	if messageAdded != 1 {
		t.Errorf("一条流只应开一次 message 项，实际开了 %d 次", messageAdded)
	}
}
