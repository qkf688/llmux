package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/atopos31/llmio/common/maputil"
)

// TransformToUnified 将 Anthropic 请求格式转换为统一格式。
func TransformToUnified(rawBody []byte) (*UnifiedRequest, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, err
	}

	unified := &UnifiedRequest{
		Model:  maputil.String(req, "model"),
		Stream: maputil.Bool(req, "stream"),
	}
	unified.System, unified.SystemParts = parseSystem(req["system"])

	if maxTokens, ok := req["max_tokens"].(float64); ok {
		unified.MaxTokens = int(maxTokens)
	}
	if temp, ok := req["temperature"].(float64); ok {
		unified.Temperature = &temp
	}
	if topP, ok := req["top_p"].(float64); ok {
		unified.TopP = &topP
	}

	unified.Messages = parseMessages(req["messages"])
	unified.Tools = parseTools(req["tools"])

	if stopSeqs := maputil.StringSlice(req, "stop_sequences"); len(stopSeqs) > 0 {
		unified.Stop = &UnifiedStop{Multiple: stopSeqs}
	}

	if metadata := maputil.StringMap(req, "metadata"); len(metadata) > 0 {
		unified.Metadata = metadata
	}

	if thinking, ok := asMap(req["thinking"]); ok {
		thinkingType := maputil.String(thinking, "type")
		budgetTokens := maputil.Int64(thinking, "budget_tokens")
		if thinkingType == "enabled" && budgetTokens > 0 {
			effort := thinkingBudgetToReasoningEffort(budgetTokens)
			if effort != "" {
				unified.ReasoningEffort = &effort
			}
			unified.ReasoningBudget = &budgetTokens
		}
	}

	// tool_choice (best-effort): keep unified semantics as OpenAI-style tool_choice.
	if rawToolChoice, exists := req["tool_choice"]; exists && rawToolChoice != nil {
		unified.ToolChoice = &UnifiedToolChoice{}

		if v, ok := rawToolChoice.(string); ok && v != "" {
			unified.ToolChoice.StringValue = &v
		} else if tcMap, ok := asMap(rawToolChoice); ok {
			tcType := maputil.String(tcMap, "type")
			switch tcType {
			case "tool":
				name := maputil.String(tcMap, "name")
				if name != "" {
					unified.ToolChoice.ObjectValue = &UnifiedToolChoiceObject{
						Type: "function",
						Function: &UnifiedToolChoiceFunction{
							Name: name,
						},
					}
				}
			default:
				if tcType != "" {
					unified.ToolChoice.StringValue = &tcType
				}
			}
		}

		if unified.ToolChoice.StringValue == nil && unified.ToolChoice.ObjectValue == nil {
			unified.ToolChoice = nil
		}
	}

	return unified, nil
}

func parseMessages(raw interface{}) []UnifiedMessage {
	items, ok := asSlice(raw)
	if !ok {
		return nil
	}

	messages := make([]UnifiedMessage, 0, len(items))
	for _, item := range items {
		msgMap, ok := asMap(item)
		if !ok {
			continue
		}

		content, toolResultMessages := parseMessageContentAndToolResults(msgMap["content"])
		msg := UnifiedMessage{
			Role:      maputil.String(msgMap, "role"),
			Content:   content,
			ToolCalls: parseToolCalls(msgMap["content"]),
		}
		msg.CacheControl = parseCacheControl(msgMap["cache_control"])

		// tool_result blocks are mapped to OpenAI-style tool messages to enable cross-format conversion.
		if len(toolResultMessages) > 0 {
			messages = append(messages, toolResultMessages...)
		}

		// If a user message only contains tool_result blocks, skip the original message.
		if msg.Role == "user" && content == nil && len(toolResultMessages) > 0 {
			continue
		}

		messages = append(messages, msg)
	}
	return messages
}

func parseTools(raw interface{}) []UnifiedTool {
	items, ok := asSlice(raw)
	if !ok {
		return nil
	}

	tools := make([]UnifiedTool, 0, len(items))
	for _, item := range items {
		toolMap, ok := asMap(item)
		if !ok {
			continue
		}

		tool := UnifiedTool{
			Type: "function",
			Function: UnifiedFunc{
				Name:        maputil.String(toolMap, "name"),
				Description: maputil.String(toolMap, "description"),
				Parameters:  toolMap["input_schema"],
			},
		}
		tool.CacheControl = parseCacheControl(toolMap["cache_control"])
		tools = append(tools, tool)
	}
	return tools
}

func parseToolCalls(rawContent interface{}) []UnifiedToolCall {
	content, ok := asSlice(rawContent)
	if !ok {
		return nil
	}

	toolCalls := make([]UnifiedToolCall, 0)
	for _, item := range content {
		itemMap, ok := asMap(item)
		if !ok || maputil.String(itemMap, "type") != "tool_use" {
			continue
		}

		argsStr := "{}"
		if inputMap, ok := asMap(itemMap["input"]); ok {
			if argsBytes, err := json.Marshal(inputMap); err == nil {
				argsStr = string(argsBytes)
			}
		}

		index := len(toolCalls)
		toolCalls = append(toolCalls, UnifiedToolCall{
			ID:    maputil.String(itemMap, "id"),
			Type:  "function",
			Index: index,
			Function: UnifiedToolCallFunction{
				Name:      maputil.String(itemMap, "name"),
				Arguments: argsStr,
			},
			CacheControl: parseCacheControl(itemMap["cache_control"]),
		})
	}

	return toolCalls
}

func parseMessageContentAndToolResults(raw interface{}) (content interface{}, toolResultMessages []UnifiedMessage) {
	if raw == nil {
		return nil, nil
	}

	if str, ok := raw.(string); ok {
		return str, nil
	}

	items, ok := asSlice(raw)
	if !ok {
		// Keep behavior: passthrough unknown payloads (but this may reduce conversion quality).
		return raw, nil
	}

	parts := make([]UnifiedMessageContentPart, 0, len(items))

	for _, item := range items {
		itemMap, ok := asMap(item)
		if !ok {
			continue
		}

		switch maputil.String(itemMap, "type") {
		case "text":
			text := maputil.String(itemMap, "text")
			if text == "" {
				// Some callers use "content" for text blocks.
				text = maputil.String(itemMap, "content")
			}
			if text == "" {
				continue
			}

			part := UnifiedMessageContentPart{
				Type: "text",
				Text: &text,
			}
			part.CacheControl = parseCacheControl(itemMap["cache_control"])
			parts = append(parts, part)

		case "image":
			source, ok := asMap(itemMap["source"])
			if !ok {
				continue
			}

			var url string
			switch maputil.String(source, "type") {
			case "base64":
				mediaType := maputil.String(source, "media_type")
				data := maputil.String(source, "data")
				if mediaType == "" || data == "" {
					continue
				}
				url = fmt.Sprintf("data:%s;base64,%s", mediaType, data)
			case "url":
				url = maputil.String(source, "url")
			}
			if url == "" {
				continue
			}

			part := UnifiedMessageContentPart{
				Type: "image_url",
				ImageURL: &UnifiedImageURL{
					URL: url,
				},
			}
			part.CacheControl = parseCacheControl(itemMap["cache_control"])
			parts = append(parts, part)

		case "tool_result":
			toolUseID := maputil.String(itemMap, "tool_use_id")
			if toolUseID == "" {
				continue
			}

			toolContent := ""
			switch v := itemMap["content"].(type) {
			case string:
				toolContent = v
			case []interface{}:
				toolContent = extractTextFromContentBlocks(v)
			}

			toolMsg := UnifiedMessage{
				Role:         "tool",
				ToolCallID:   toolUseID,
				Content:      toolContent,
				CacheControl: parseCacheControl(itemMap["cache_control"]),
			}
			if isErr, ok := itemMap["is_error"].(bool); ok {
				toolMsg.ToolCallIsError = &isErr
			}

			toolResultMessages = append(toolResultMessages, toolMsg)
		}
	}

	if len(parts) == 0 {
		return nil, toolResultMessages
	}

	if len(parts) == 1 && parts[0].Type == "text" && parts[0].Text != nil && parts[0].CacheControl == nil {
		return *parts[0].Text, toolResultMessages
	}

	return parts, toolResultMessages
}

func extractTextFromContentBlocks(items []interface{}) string {
	var b strings.Builder
	for _, item := range items {
		itemMap, ok := asMap(item)
		if !ok || maputil.String(itemMap, "type") != "text" {
			continue
		}

		text := maputil.String(itemMap, "text")
		if text == "" {
			text = maputil.String(itemMap, "content")
		}
		if text == "" {
			continue
		}
		b.WriteString(text)
	}
	return b.String()
}
