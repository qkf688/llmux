package streaming

import (
	"encoding/json"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/service/responses"
)

func init() {
	RegisterRealtimeRoute("openai-res", "anthropic", handleRealtimeResponsesToAnthropic)
}

func handleRealtimeResponsesToAnthropic(state *realtimeStreamState, data string) error {
	// [DONE] 被忽略，因为 response.completed 已发送 message_stop
	if data == "[DONE]" {
		return nil
	}

	var ev responses.ResponsesStreamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		logRealtimeChunkParseError(state, data, err)
		return nil
	}

	eventType := state.currentEvent
	if eventType == "" {
		eventType = ev.Type
	}

	switch eventType {
	case "response.created":
		return handleResponsesToAnthropicCreatedEvent(state, &ev)
	case "response.output_item.added":
		return handleResponsesToAnthropicOutputItemAddedEvent(state, &ev)
	case "response.output_text.delta":
		return handleResponsesToAnthropicOutputTextDeltaEvent(state, &ev)
	case "response.reasoning_summary_text.delta":
		return handleResponsesToAnthropicReasoningDeltaEvent(state, &ev)
	case "response.function_call_arguments.delta":
		return handleResponsesToAnthropicFunctionArgsDeltaEvent(state, &ev)
	case "response.content_part.added":
		return handleResponsesToAnthropicContentPartAddedEvent(state, &ev)
	case "response.completed":
		return handleResponsesToAnthropicCompletedEvent(state, &ev)
	default:
		return nil
	}
}

func handleResponsesToAnthropicCreatedEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	responseID := ""
	if ev.Response != nil {
		responseID = ev.Response.ID
	}
	if responseID == "" {
		responseID = ev.ResponseID
	}

	messageStart := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":      responseID,
			"type":    "message",
			"role":    "assistant",
			"content": []interface{}{},
			"model":   "responses-api",
			"usage": map[string]interface{}{
				"input_tokens":  0,
				"output_tokens": 0,
			},
		},
	}
	return writeRealtimeEventJSONData(state, "message_start", messageStart)
}

func handleResponsesToAnthropicOutputItemAddedEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	if ev.Item == nil {
		return nil
	}
	itemType := ev.Item.Type

	blockIndex := getOrAllocAnthropicBlockIndex(state, ev.OutputIndex)
	if err := closeAnthropicActiveBlockIfNeeded(state, blockIndex); err != nil {
		return err
	}

	blockStart := map[string]interface{}{
		"type":  "content_block_start",
		"index": blockIndex,
		"content_block": map[string]interface{}{
			"type": itemType,
		},
	}

	// 映射 Responses 类型 → Anthropic 类型
	if itemType == "function_call" {
		blockStart["content_block"] = map[string]interface{}{
			"type": "tool_use",
			"id":   ev.Item.ID,
			"name": derefString(ev.Item.Name),
		}
	} else if itemType == "reasoning" {
		blockStart["content_block"] = map[string]interface{}{
			"type": "thinking",
		}
	} else if itemType == "output_text" || itemType == "text" {
		blockStart["content_block"] = map[string]interface{}{
			"type": "text",
			"text": "",
		}
	}

	if err := writeRealtimeEventJSONData(state, "content_block_start", blockStart); err != nil {
		return err
	}
	state.anthropicActiveBlockIndex = blockIndex
	return nil
}

func handleResponsesToAnthropicOutputTextDeltaEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	delta := ev.Delta
	if delta == "" {
		return nil
	}

	contentDelta := map[string]interface{}{
		"type":  "content_block_delta",
		"index": getAnthropicBlockIndex(state, ev.OutputIndex),
		"delta": map[string]interface{}{
			"type": "text_delta",
			"text": delta,
		},
	}
	return writeRealtimeEventJSONData(state, "content_block_delta", contentDelta)
}

func handleResponsesToAnthropicReasoningDeltaEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	delta := ev.Delta
	if delta == "" {
		return nil
	}

	contentDelta := map[string]interface{}{
		"type":  "content_block_delta",
		"index": getAnthropicBlockIndex(state, ev.OutputIndex),
		"delta": map[string]interface{}{
			"type":     "thinking_delta",
			"thinking": delta,
		},
	}
	return writeRealtimeEventJSONData(state, "content_block_delta", contentDelta)
}

func handleResponsesToAnthropicFunctionArgsDeltaEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	delta := ev.Delta
	if delta == "" {
		return nil
	}

	contentDelta := map[string]interface{}{
		"type":  "content_block_delta",
		"index": getAnthropicBlockIndex(state, ev.OutputIndex),
		"delta": map[string]interface{}{
			"type":         "input_json_delta",
			"partial_json": delta,
		},
	}
	return writeRealtimeEventJSONData(state, "content_block_delta", contentDelta)
}

func handleResponsesToAnthropicCompletedEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	stopReason := "end_turn"
	if ev.Response != nil && ev.Response.Status != nil && *ev.Response.Status == "incomplete" {
		stopReason = "max_tokens"
	}
	if ev.Response != nil {
		for _, item := range ev.Response.Output {
			if item.Type == "function_call" {
				stopReason = "tool_use"
				break
			}
		}
	}

	if state.anthropicActiveBlockIndex >= 0 {
		blockStop := map[string]interface{}{
			"type":  "content_block_stop",
			"index": state.anthropicActiveBlockIndex,
		}
		if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
			return err
		}
		state.anthropicActiveBlockIndex = -1
	}

	messageDelta := map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason": stopReason,
		},
	}

	// 添加 usage 信息
	if ev.Response != nil && ev.Response.Usage != nil {
		messageDelta["usage"] = map[string]interface{}{
			"input_tokens":  int(ev.Response.Usage.InputTokens),
			"output_tokens": int(ev.Response.Usage.OutputTokens),
		}
	}

	if err := writeRealtimeEventJSONData(state, "message_delta", messageDelta); err != nil {
		return err
	}

	messageStop := map[string]interface{}{"type": "message_stop"}
	return writeRealtimeEventJSONData(state, "message_stop", messageStop)
}

func handleResponsesToAnthropicContentPartAddedEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	if ev.Part == nil {
		return nil
	}
	if ev.Part.Type != "image_url" && ev.Part.Type != "output_image" && ev.Part.Type != "input_image" && ev.Part.Type != "image" {
		return nil
	}

	imageURL, detail := extractResponsesContentPartImageURL(ev.Part)
	if imageURL == "" {
		return nil
	}

	blockIndex := allocAnthropicBlockIndex(state)
	if err := closeAnthropicActiveBlockIfNeeded(state, blockIndex); err != nil {
		return err
	}

	block := map[string]interface{}{
		"type": "image",
	}
	if mediaType, data, ok := common.ParseBase64DataURL(imageURL); ok {
		block["source"] = map[string]interface{}{
			"type":       "base64",
			"media_type": mediaType,
			"data":       data,
		}
	} else {
		block["source"] = map[string]interface{}{
			"type": "url",
			"url":  imageURL,
		}
	}
	if detail != nil && *detail != "" {
		block["detail"] = *detail
	}

	blockStart := map[string]interface{}{
		"type":          "content_block_start",
		"index":         blockIndex,
		"content_block": block,
	}
	if err := writeRealtimeEventJSONData(state, "content_block_start", blockStart); err != nil {
		return err
	}

	blockStop := map[string]interface{}{
		"type":  "content_block_stop",
		"index": blockIndex,
	}
	if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
		return err
	}

	state.anthropicActiveBlockIndex = -1
	return nil
}

func closeAnthropicActiveBlockIfNeeded(state *realtimeStreamState, nextIndex int) error {
	if state.anthropicActiveBlockIndex < 0 || state.anthropicActiveBlockIndex == nextIndex {
		return nil
	}
	blockStop := map[string]interface{}{
		"type":  "content_block_stop",
		"index": state.anthropicActiveBlockIndex,
	}
	if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
		return err
	}
	state.anthropicActiveBlockIndex = -1
	return nil
}

func getOrAllocAnthropicBlockIndex(state *realtimeStreamState, outputIndex int) int {
	if state.responsesOutputIndexToAnthropicBlockIndex == nil {
		state.responsesOutputIndexToAnthropicBlockIndex = map[int]int{}
	}
	if idx, ok := state.responsesOutputIndexToAnthropicBlockIndex[outputIndex]; ok {
		return idx
	}
	idx := allocAnthropicBlockIndex(state)
	state.responsesOutputIndexToAnthropicBlockIndex[outputIndex] = idx
	return idx
}

func getAnthropicBlockIndex(state *realtimeStreamState, outputIndex int) int {
	if state.responsesOutputIndexToAnthropicBlockIndex == nil {
		return outputIndex
	}
	if idx, ok := state.responsesOutputIndexToAnthropicBlockIndex[outputIndex]; ok {
		return idx
	}
	return outputIndex
}

func allocAnthropicBlockIndex(state *realtimeStreamState) int {
	if state.anthropicNextBlockIndex < 0 {
		state.anthropicNextBlockIndex = 0
	}
	idx := state.anthropicNextBlockIndex
	state.anthropicNextBlockIndex++
	return idx
}

func extractResponsesContentPartImageURL(part *responses.ResponsesContentPart) (string, *string) {
	if part == nil {
		return "", nil
	}
	if part.ImageURL != nil && len(*part.ImageURL) > 0 {
		var urlStr string
		if err := json.Unmarshal(*part.ImageURL, &urlStr); err == nil && urlStr != "" {
			return urlStr, part.Detail
		}
		var obj struct {
			URL    string  `json:"url"`
			Detail *string `json:"detail,omitempty"`
		}
		if err := json.Unmarshal(*part.ImageURL, &obj); err == nil && obj.URL != "" {
			if obj.Detail != nil && *obj.Detail != "" {
				return obj.URL, obj.Detail
			}
			return obj.URL, part.Detail
		}
	}
	if part.URL != nil && *part.URL != "" {
		return *part.URL, part.Detail
	}
	return "", part.Detail
}
