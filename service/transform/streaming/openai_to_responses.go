package streaming

import (
	"encoding/json"

	"github.com/qkf688/llmux/common/maputil"
)

func init() {
	RegisterRealtimeRoute("openai", "openai-res", handleRealtimeOpenAIToResponses)
}

func handleRealtimeOpenAIToResponses(state *realtimeStreamState, data string) error {
	// OpenAI 的 [DONE] 不需要转换，但它是「上游已结束」的信号：
	// 此时把延后的 response.completed（携带尾包 usage）flush 出去。
	// 正常路径下 usage 尾包到达时已交付，这里退化为幂等兜底。
	if data == "[DONE]" {
		return flushPendingOpenAIToResponsesCompleted(state)
	}

	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		logRealtimeChunkParseError(state, data, err)
		return nil
	}

	// usage 可能出现在任意 chunk（含 finish 之后的 choices:[] 尾包）。
	// 先无条件抓取，供延后的 response.completed 使用（待办 #18）。
	captureOpenAIUsage(state, chunk)

	// usage 一到即可交付终态，不必再等 [DONE]。
	// 这一步是活性保障而非优化：若上游发完 usage 后既不发 [DONE] 也不关连接
	// （keep-alive 挂住、代理层半开），而终态只在 [DONE]/EOF 交付，客户端要等到
	// 读超时；且第二跳 responses_to_anthropic 的收尾只由 response.completed 驱动、
	// 无 EOF 兜底，届时 anthropic 客户端会拿到未关闭的 content_block。
	// 对「usage 与 finish 同包」的上游不触发：那时 pendingCompleted 尚为 nil。
	if state.pendingCompleted != nil && len(state.pendingUsage) > 0 {
		if err := flushPendingOpenAIToResponsesCompleted(state); err != nil {
			return err
		}
	}

	choices, ok := chunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		// choices:[] 的尾包（stream_options.include_usage 的 usage 载体）到此为止：
		// usage 已被 captureOpenAIUsage 收走，无 delta/finish 可处理。
		return nil
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil
	}

	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return nil
	}

	if role := maputil.String(delta, "role"); role != "" {
		if err := startOpenAIToResponsesStream(state, chunk, role); err != nil {
			return err
		}
	}

	if reasoningContent := maputil.String(delta, "reasoning_content"); reasoningContent != "" {
		if err := emitOpenAIToResponsesReasoningDelta(state, reasoningContent); err != nil {
			return err
		}
	}

	if content := maputil.String(delta, "content"); content != "" {
		state.accumulatedText += content
		textDelta := map[string]interface{}{
			"type":            "response.output_text.delta",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    0,
			"item_id":         state.itemID,
			"content_index":   0,
			"delta":           content,
		}
		if err := writeRealtimeOrderedData(state, textDelta); err != nil {
			return err
		}
	}

	if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
		if err := emitOpenAIToResponsesToolCalls(state, toolCalls); err != nil {
			return err
		}
	}

	if finishReason := maputil.String(choice, "finish_reason"); finishReason != "" {
		return finishOpenAIToResponsesStream(state, chunk, finishReason)
	}

	return nil
}

// ensureMessageOutputIndex 惰性给 message 项分配 output_index。
// 分配而非写死 0，是为了让 reasoning / function_call 能拿到互不重叠的 index。
func ensureMessageOutputIndex(state *realtimeStreamState) int {
	if !state.hasMessageOutputIndex {
		state.messageOutputIndex = state.responsesOutput.alloc()
		state.hasMessageOutputIndex = true
	}
	return state.messageOutputIndex
}

func startOpenAIToResponsesStream(state *realtimeStreamState, chunk map[string]interface{}, role string) error {
	state.responseID = maputil.String(chunk, "id")
	state.itemID = "msg_" + state.responseID
	msgIndex := ensureMessageOutputIndex(state)

	responseCreated := map[string]interface{}{
		"type":            "response.created",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"id":         state.responseID,
			"object":     "response",
			"model":      maputil.String(chunk, "model"),
			"created_at": int(maputil.Float64(chunk, "created")),
			"status":     "in_progress",
			"output":     []interface{}{},
		},
	}
	if err := writeRealtimeOrderedData(state, responseCreated); err != nil {
		return err
	}

	responseInProgress := map[string]interface{}{
		"type":            "response.in_progress",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"id":         state.responseID,
			"object":     "response",
			"model":      maputil.String(chunk, "model"),
			"created_at": int(maputil.Float64(chunk, "created")),
			"status":     "in_progress",
			"output":     []interface{}{},
		},
	}
	if err := writeRealtimeOrderedData(state, responseInProgress); err != nil {
		return err
	}

	itemAdded := map[string]interface{}{
		"type":            "response.output_item.added",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item": map[string]interface{}{
			"id":      state.itemID,
			"type":    "message",
			"role":    role,
			"content": []interface{}{},
			"status":  "in_progress",
		},
	}
	if err := writeRealtimeOrderedData(state, itemAdded); err != nil {
		return err
	}

	contentPartAdded := map[string]interface{}{
		"type":            "response.content_part.added",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item_id":         state.itemID,
		"content_index":   0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": "",
		},
	}
	return writeRealtimeOrderedData(state, contentPartAdded)
}

func emitOpenAIToResponsesReasoningDelta(state *realtimeStreamState, reasoningContent string) error {
	// 如果还没有创建 reasoning item，先创建
	if !state.hasReasoningItem {
		state.hasReasoningItem = true
		state.reasoningItemID = "reasoning_" + state.responseID
		// 独立分配：写死 0 会与 message 项的 output_index 撞车。
		state.reasoningOutputIndex = state.responsesOutput.alloc()

		reasoningItemAdded := map[string]interface{}{
			"type":            "response.output_item.added",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item": map[string]interface{}{
				"id":      state.reasoningItemID,
				"type":    "reasoning",
				"status":  "in_progress",
				"summary": []interface{}{},
			},
		}
		if err := writeRealtimeOrderedData(state, reasoningItemAdded); err != nil {
			return err
		}

		summaryPartAdded := map[string]interface{}{
			"type":            "response.reasoning_summary_part.added",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item_id":         state.reasoningItemID,
			"summary_index":   0,
			"part": map[string]interface{}{
				"type": "summary_text",
			},
		}
		if err := writeRealtimeOrderedData(state, summaryPartAdded); err != nil {
			return err
		}
	}

	state.accumulatedReasoning += reasoningContent
	reasoningDelta := map[string]interface{}{
		"type":            "response.reasoning_summary_text.delta",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    state.reasoningOutputIndex,
		"item_id":         state.reasoningItemID,
		"summary_index":   0,
		"delta":           reasoningContent,
	}
	return writeRealtimeOrderedData(state, reasoningDelta)
}

func emitOpenAIToResponsesToolCalls(state *realtimeStreamState, toolCalls []interface{}) error {
	for _, tc := range toolCalls {
		toolCall, ok := tc.(map[string]interface{})
		if !ok {
			continue
		}
		function, ok := toolCall["function"].(map[string]interface{})
		if !ok {
			continue
		}

		toolCallIndex := int(maputil.Float64(toolCall, "index"))
		outputIndex := getOrAllocResponsesOutputIndexForToolCall(state, toolCallIndex)

		// 首包含 id/name 时，发送 output_item.added 事件
		if id := maputil.String(toolCall, "id"); id != "" {
			item := map[string]interface{}{
				"type":      "function_call",
				"id":        id,
				"call_id":   id,
				"name":      maputil.String(function, "name"),
				"arguments": "",
			}
			state.responsesOutput.set(outputIndex, item)

			itemAdded := map[string]interface{}{
				"type":            "response.output_item.added",
				"sequence_number": nextRealtimeSequence(state),
				"output_index":    outputIndex,
				"item":            item,
			}
			if err := writeRealtimeOrderedData(state, itemAdded); err != nil {
				return err
			}
		}

		// 参数增量
		if args := maputil.String(function, "arguments"); args != "" {
			state.responsesOutput.appendArguments(outputIndex, args)

			argsDelta := map[string]interface{}{
				"type":            "response.function_call_arguments.delta",
				"sequence_number": nextRealtimeSequence(state),
				"output_index":    outputIndex,
				"delta":           args,
			}
			if err := writeRealtimeOrderedData(state, argsDelta); err != nil {
				return err
			}
		}
	}

	return nil
}

// getOrAllocResponsesOutputIndexForToolCall 把 OpenAI 的 tool_call index（从 0 起）
// 映射到一个独立的 Responses output_index，避开 message / reasoning 项已占用的 index。
func getOrAllocResponsesOutputIndexForToolCall(state *realtimeStreamState, toolCallIndex int) int {
	if state.openAIToolCallIndexToResponsesOutputIndex == nil {
		state.openAIToolCallIndexToResponsesOutputIndex = map[int]int{}
	}
	if idx, ok := state.openAIToolCallIndexToResponsesOutputIndex[toolCallIndex]; ok {
		return idx
	}
	idx := state.responsesOutput.alloc()
	state.openAIToolCallIndexToResponsesOutputIndex[toolCallIndex] = idx
	return idx
}

func finishOpenAIToResponsesStream(state *realtimeStreamState, chunk map[string]interface{}, finishReason string) error {
	msgIndex := ensureMessageOutputIndex(state)

	// 如果有 reasoning 内容，发送 reasoning 完成事件
	if state.hasReasoningItem && state.accumulatedReasoning != "" {
		reasoningTextDone := map[string]interface{}{
			"type":            "response.reasoning_summary_text.done",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item_id":         state.reasoningItemID,
			"summary_index":   0,
			"text":            state.accumulatedReasoning,
		}
		if err := writeRealtimeOrderedData(state, reasoningTextDone); err != nil {
			return err
		}

		reasoningPartDone := map[string]interface{}{
			"type":            "response.reasoning_summary_part.done",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item_id":         state.reasoningItemID,
			"summary_index":   0,
			"part": map[string]interface{}{
				"type": "summary_text",
				"text": state.accumulatedReasoning,
			},
		}
		if err := writeRealtimeOrderedData(state, reasoningPartDone); err != nil {
			return err
		}

		state.responsesOutput.set(state.reasoningOutputIndex, map[string]interface{}{
			"id":   state.reasoningItemID,
			"type": "reasoning",
			"summary": []map[string]interface{}{
				{
					"type": "summary_text",
					"text": state.accumulatedReasoning,
				},
			},
			"status": "completed",
		})
	}

	outputTextDone := map[string]interface{}{
		"type":            "response.output_text.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item_id":         state.itemID,
		"content_index":   0,
		"text":            state.accumulatedText,
	}
	if err := writeRealtimeOrderedData(state, outputTextDone); err != nil {
		return err
	}

	contentPartDone := map[string]interface{}{
		"type":            "response.content_part.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item_id":         state.itemID,
		"content_index":   0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": state.accumulatedText,
		},
	}
	if err := writeRealtimeOrderedData(state, contentPartDone); err != nil {
		return err
	}

	// 构建 output_item.done 事件的 output 数组，包含 reasoning（如果有）
	outputItems := []map[string]interface{}{
		{
			"type": "output_text",
			"text": state.accumulatedText,
		},
	}
	if state.hasReasoningItem && state.accumulatedReasoning != "" {
		outputItems = append([]map[string]interface{}{
			{
				"type": "reasoning",
				"id":   state.reasoningItemID,
				"summary": []map[string]interface{}{
					{
						"type": "summary_text",
						"text": state.accumulatedReasoning,
					},
				},
			},
		}, outputItems...)
	}

	messageItem := map[string]interface{}{
		"id":      state.itemID,
		"type":    "message",
		"role":    "assistant",
		"content": outputItems,
		"status":  "completed",
	}
	state.responsesOutput.set(msgIndex, messageItem)

	outputItemDone := map[string]interface{}{
		"type":            "response.output_item.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item":            messageItem,
	}
	if err := writeRealtimeOrderedData(state, outputItemDone); err != nil {
		return err
	}

	status := "completed"
	if finishReason == "length" {
		status = "incomplete"
	}

	responseCompleted := map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"object":     "response",
			"id":         state.responseID,
			"model":      maputil.String(chunk, "model"),
			"created_at": int(maputil.Float64(chunk, "created")),
			"status":     status,
			// 必须是真实产出：下游靠扫这个数组里有没有 function_call 判定 finish_reason。
			"output": state.responsesOutput.snapshot(),
		},
	}

	// 不能在此直接写出：OpenAI 把 usage 放在 finish_reason 之后的独立尾包里，
	// 现在发就永远带不上真实 token 数（待办 #18）。缓存起来，等 [DONE] 或流末
	// 由 flushPendingOpenAIToResponsesCompleted 合并 usage 后统一发出。
	state.pendingCompleted = responseCompleted
	state.finalize = flushPendingOpenAIToResponsesCompleted
	return nil
}

// captureOpenAIUsage 记录 chunk 里的 OpenAI 格式 usage。
// usage 既可能与 finish_reason 同包（部分上游的实现），也可能在其后的
// choices:[] 尾包里（OpenAI 规范行为），故对每个 chunk 都尝试抓取；
// 后到的非空 usage 覆盖先前的，保证拿到最终值。
func captureOpenAIUsage(state *realtimeStreamState, chunk map[string]interface{}) {
	usage, ok := chunk["usage"].(map[string]interface{})
	if !ok || len(usage) == 0 {
		return
	}
	state.pendingUsage = usage
}

// flushPendingOpenAIToResponsesCompleted 把延后的 response.completed 合并 usage 后写出。
// 幂等：写出后清空全部延后状态。三个入口（usage 尾包到达 / [DONE] / 流末 finalize）
// 只有第一个真正写出，其余因 pendingCompleted == nil 直接返回。
//
// 注：唯一的理论例外是上游在 [DONE] 之后仍发含 finish_reason 的帧（协议违规，会重新
// set pending 并在流末再发一次）；重试是整流重发而非流内续传，项目内无此路径，故不防御。
func flushPendingOpenAIToResponsesCompleted(state *realtimeStreamState) error {
	completed := state.pendingCompleted
	if completed == nil {
		return nil
	}
	// 三个字段生命周期一致（「已消费」）：一并清空，避免读代码时误判还需收尾。
	usage := state.pendingUsage
	state.pendingCompleted = nil
	state.pendingUsage = nil
	state.finalize = nil

	// sequence_number 在此刻才分配：它必须反映真实发出顺序，而非构建顺序。
	// 依赖 nextRealtimeSequence 的不变量「取号顺序 == 写出顺序」。
	completed["sequence_number"] = nextRealtimeSequence(state)

	if response, ok := completed["response"].(map[string]interface{}); ok {
		if u := buildResponsesUsage(usage); u != nil {
			response["usage"] = u
		}
	}

	return writeRealtimeOrderedData(state, completed)
}

// buildResponsesUsage 把 OpenAI Chat 的 usage 映射为 Responses 的 usage。
// 入参为 nil / 空时返回 nil，表示无 usage 可写（不写出零值，避免下游把 0 当真实统计）。
func buildResponsesUsage(usage map[string]interface{}) map[string]interface{} {
	if len(usage) == 0 {
		return nil
	}

	promptTokens := int(maputil.Float64(usage, "prompt_tokens"))
	completionTokens := int(maputil.Float64(usage, "completion_tokens"))

	usageMap := map[string]interface{}{
		"input_tokens":  promptTokens,
		"output_tokens": completionTokens,
		"total_tokens":  int(maputil.Float64(usage, "total_tokens")),
	}
	// cached_tokens / reasoning_tokens 取上游尾包里的真值而非写死 0——
	// openai-res processer 会把 cached_tokens 落库，写死 0 与 #18 同源。
	if promptTokens > 0 {
		cached := 0
		if d, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
			cached = int(maputil.Float64(d, "cached_tokens"))
		}
		usageMap["input_tokens_details"] = map[string]interface{}{
			"cached_tokens": cached,
		}
	}
	if completionTokens > 0 {
		reasoning := 0
		if d, ok := usage["completion_tokens_details"].(map[string]interface{}); ok {
			reasoning = int(maputil.Float64(d, "reasoning_tokens"))
		}
		usageMap["output_tokens_details"] = map[string]interface{}{
			"reasoning_tokens": reasoning,
		}
	}
	return usageMap
}
