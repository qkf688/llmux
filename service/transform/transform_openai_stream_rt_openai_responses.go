package transform

import (
	"encoding/json"

	"github.com/atopos31/llmio/common/maputil"
)

func handleRealtimeOpenAIToResponses(state *realtimeStreamState, data string) error {
	// OpenAI 的 [DONE] 不需要转换
	if data == "[DONE]" {
		return nil
	}

	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		logRealtimeChunkParseError(state, data, err)
		return nil
	}

	choices, ok := chunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
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

func startOpenAIToResponsesStream(state *realtimeStreamState, chunk map[string]interface{}, role string) error {
	state.responseID = maputil.String(chunk, "id")
	state.itemID = "msg_" + state.responseID

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
		"output_index":    0,
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
		"output_index":    0,
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
		state.reasoningOutputIndex = 0

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

		// 首包含 id/name 时，发送 output_item.added 事件
		if id := maputil.String(toolCall, "id"); id != "" {
			itemAdded := map[string]interface{}{
				"type":         "response.output_item.added",
				"output_index": int(maputil.Float64(toolCall, "index")),
				"item": map[string]interface{}{
					"type":      "function_call",
					"id":        id,
					"call_id":   id,
					"name":      maputil.String(function, "name"),
					"arguments": "",
				},
			}
			if err := writeRealtimeEventJSONData(state, "response.output_item.added", itemAdded); err != nil {
				return err
			}
		}

		// 参数增量
		if args := maputil.String(function, "arguments"); args != "" {
			argsDelta := map[string]interface{}{
				"type":         "response.function_call_arguments.delta",
				"output_index": int(maputil.Float64(toolCall, "index")),
				"delta":        args,
			}
			if err := writeRealtimeEventJSONData(state, "response.function_call_arguments.delta", argsDelta); err != nil {
				return err
			}
		}
	}

	return nil
}

func finishOpenAIToResponsesStream(state *realtimeStreamState, chunk map[string]interface{}, finishReason string) error {
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
	}

	outputTextDone := map[string]interface{}{
		"type":            "response.output_text.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    0,
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
		"output_index":    0,
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

	outputItemDone := map[string]interface{}{
		"type":            "response.output_item.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    0,
		"item": map[string]interface{}{
			"id":      state.itemID,
			"type":    "message",
			"role":    "assistant",
			"content": outputItems,
			"status":  "completed",
		},
	}
	if err := writeRealtimeOrderedData(state, outputItemDone); err != nil {
		return err
	}

	status := "completed"
	if finishReason == "length" {
		status = "incomplete"
	}

	responseCompleted := map[string]interface{}{
		"type":            "response.completed",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"object":     "response",
			"id":         state.responseID,
			"model":      maputil.String(chunk, "model"),
			"created_at": int(maputil.Float64(chunk, "created")),
			"status":     status,
			"output":     []interface{}{},
		},
	}

	// 添加 usage 信息
	if usage, ok := chunk["usage"].(map[string]interface{}); ok {
		usageMap := map[string]interface{}{
			"input_tokens":  int(maputil.Float64(usage, "prompt_tokens")),
			"output_tokens": int(maputil.Float64(usage, "completion_tokens")),
			"total_tokens":  int(maputil.Float64(usage, "total_tokens")),
		}
		// 添加 input_tokens_details
		if promptTokens := int(maputil.Float64(usage, "prompt_tokens")); promptTokens > 0 {
			usageMap["input_tokens_details"] = map[string]interface{}{
				"cached_tokens": 0,
			}
		}
		// 添加 output_tokens_details
		if completionTokens := int(maputil.Float64(usage, "completion_tokens")); completionTokens > 0 {
			usageMap["output_tokens_details"] = map[string]interface{}{
				"reasoning_tokens": 0,
			}
		}
		responseCompleted["response"].(map[string]interface{})["usage"] = usageMap
	}

	return writeRealtimeOrderedData(state, responseCompleted)
}
