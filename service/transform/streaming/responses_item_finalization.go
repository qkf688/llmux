package streaming

// emitResponsesOutputItemFinalization 为本流中所有「开了还没关」的 output item
// 发收尾事件：
//
//	function_call 项 → response.function_call_arguments.done（带完整 arguments）
//	                   + response.output_item.done（item 终态，status=completed）
//	其他项（reasoning 等）→ response.output_item.done
//
// **为什么必须有**：Responses 协议里每个 output item 都是「开—增—关」三段式，
// 客户端按「开了没关」判未完成。只发 output_item.added + 参数 delta 而不发收尾，
// 严格 Responses 客户端（deepseek-harness 实测）会报
// "stream completed with an unfinished tool call: <name>" 并让本轮运行失败——
// 而上游 Chat/Anthropic 流本身是完整的（finish_reason=tool_calls、id/name/arguments
// 齐全），锅在转换层少发了收尾。
//
// **两条协议线共用这一份**：openai→responses 与 anthropic→responses 都把 output item
// 按 output_index 存进 state.responsesOutput，形状一致，差别的只是「项从哪来」；
// 收尾口径不该分叉（分叉过一次：OpenAI 线漏发工具调用收尾、Anthropic 线漏发
// OpenAI 线也漏发的 reasoning 项收尾）。
//
// **只关没关过的**：已由各自路径发过 response.output_item.done 的项
// （message、anthropic 线的 reasoning）由 store 的 closed 标记排除，
// 不会在这里被关第二次。
func emitResponsesOutputItemFinalization(state *realtimeStreamState) error {
	for _, ref := range state.responsesOutput.pendingItems() {
		itemType, _ := ref.item["type"].(string)

		// function_call 额外需要 arguments.done：客户端据此判断参数是否收全，
		// 只给 output_item.done 仍会被判「工具调用开了没关」。
		if itemType == "function_call" {
			itemID, _ := ref.item["id"].(string)
			arguments, _ := ref.item["arguments"].(string)

			argsDone := map[string]interface{}{
				"type":            "response.function_call_arguments.done",
				"sequence_number": nextRealtimeSequence(state),
				"output_index":    ref.index,
				"item_id":         itemID,
				"arguments":       arguments,
			}
			if err := writeRealtimeOrderedData(state, argsDone); err != nil {
				return err
			}
		}

		// 终态 item 带 status=completed（added 时无 status，done 时才有）。
		// **不改存储项**：done 事件用一份带 status 的副本，response.completed 的
		// output 快照仍是非流式路径的同款形状（ResponsesItem 的 function_call
		// 本就没有 status 字段），两条路径对客户端是同一份契约。
		doneItem := map[string]interface{}{}
		for k, v := range ref.item {
			doneItem[k] = v
		}
		doneItem["status"] = "completed"
		itemDone := map[string]interface{}{
			"type":            "response.output_item.done",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    ref.index,
			"item":            doneItem,
		}
		if err := writeRealtimeOrderedData(state, itemDone); err != nil {
			return err
		}
		state.responsesOutput.markClosed(ref.index)
	}
	return nil
}
