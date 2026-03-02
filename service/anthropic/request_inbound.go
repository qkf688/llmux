package anthropic

import "encoding/json"

// TransformToUnified 将 Anthropic 请求格式转换为统一格式。
func TransformToUnified(rawBody []byte) (*UnifiedRequest, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, err
	}

	unified := &UnifiedRequest{
		Model:  getString(req, "model"),
		Stream: getBool(req, "stream"),
		System: extractSystem(req["system"]),
	}

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

	if stopSeqs := getStringArray(req, "stop_sequences"); len(stopSeqs) > 0 {
		unified.Stop = &UnifiedStop{Multiple: stopSeqs}
	}

	if metadata := getStringMap(req, "metadata"); len(metadata) > 0 {
		unified.Metadata = metadata
	}

	if thinking, ok := asMap(req["thinking"]); ok {
		thinkingType := getString(thinking, "type")
		budgetTokens := getInt64(thinking, "budget_tokens")
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
			tcType := getString(tcMap, "type")
			switch tcType {
			case "tool":
				name := getString(tcMap, "name")
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

		msg := UnifiedMessage{
			Role:      getString(msgMap, "role"),
			Content:   msgMap["content"],
			ToolCalls: parseToolCalls(msgMap),
		}

		msg.ToolCallID = extractToolResultID(msgMap["content"])
		msg.CacheControl = parseCacheControl(msgMap["cache_control"])
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
				Name:        getString(toolMap, "name"),
				Description: getString(toolMap, "description"),
				Parameters:  toolMap["input_schema"],
			},
		}
		tool.CacheControl = parseCacheControl(toolMap["cache_control"])
		tools = append(tools, tool)
	}
	return tools
}

func extractToolResultID(rawContent interface{}) string {
	items, ok := asSlice(rawContent)
	if !ok {
		return ""
	}

	for _, item := range items {
		itemMap, ok := asMap(item)
		if !ok {
			continue
		}
		if getString(itemMap, "type") == "tool_result" {
			return getString(itemMap, "tool_use_id")
		}
	}
	return ""
}

func parseToolCalls(msgMap map[string]interface{}) []UnifiedToolCall {
	content, ok := asSlice(msgMap["content"])
	if !ok {
		return nil
	}

	toolCalls := make([]UnifiedToolCall, 0)
	for _, item := range content {
		itemMap, ok := asMap(item)
		if !ok || getString(itemMap, "type") != "tool_use" {
			continue
		}

		argsStr := "{}"
		if inputMap, ok := asMap(itemMap["input"]); ok {
			if argsBytes, err := json.Marshal(inputMap); err == nil {
				argsStr = string(argsBytes)
			}
		}

		toolCalls = append(toolCalls, UnifiedToolCall{
			ID:   getString(itemMap, "id"),
			Type: "function",
			Function: UnifiedToolCallFunction{
				Name:      getString(itemMap, "name"),
				Arguments: argsStr,
			},
		})
	}

	return toolCalls
}
