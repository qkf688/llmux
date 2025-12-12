package service

import (
	"encoding/json"
	"time"

	"github.com/atopos31/llmio/models"
)

// TransformAnthropicToUnified 将 Anthropic 格式转换为统一格式
func TransformAnthropicToUnified(rawBody []byte) (*UnifiedRequest, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, err
	}

	unified := &UnifiedRequest{
		Model:  getString(req, "model"),
		Stream: getBool(req, "stream"),
		System: getString(req, "system"),
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

	// 转换消息
	if messages, ok := req["messages"].([]interface{}); ok {
		for _, msg := range messages {
			msgMap := msg.(map[string]interface{})
			unified.Messages = append(unified.Messages, UnifiedMessage{
				Role:      getString(msgMap, "role"),
				Content:   msgMap["content"],
				ToolCalls: parseAnthropicToolCalls(msgMap),
			})
		}
	}

	// 转换工具
	if tools, ok := req["tools"].([]interface{}); ok {
		for _, tool := range tools {
			toolMap := tool.(map[string]interface{})
			unified.Tools = append(unified.Tools, UnifiedTool{
				Type: "function",
				Function: UnifiedFunc{
					Name:        getString(toolMap, "name"),
					Description: getString(toolMap, "description"),
					Parameters:  toolMap["input_schema"],
				},
			})
		}
	}

	return unified, nil
}

// TransformUnifiedToAnthropic 将统一格式转换为 Anthropic 格式
func TransformUnifiedToAnthropic(unified *UnifiedRequest) ([]byte, error) {
	req := map[string]interface{}{
		"model":    unified.Model,
		"messages": []interface{}{},
		"stream":   unified.Stream,
	}

	// Anthropic 格式要求必须有 max_tokens,如果没有设置则使用默认值
	if unified.MaxTokens > 0 {
		req["max_tokens"] = unified.MaxTokens
	} else {
		req["max_tokens"] = 4096 // 设置默认值
	}
	if unified.Temperature != nil {
		req["temperature"] = *unified.Temperature
	}
	if unified.TopP != nil {
		req["top_p"] = *unified.TopP
	}
	if unified.System != "" {
		req["system"] = unified.System
	}

	// 转换消息
	messages := []interface{}{}
	
	// 先统计非 system 消息的数量
	nonSystemCount := 0
	for _, msg := range unified.Messages {
		if msg.Role != "system" {
			nonSystemCount++
		}
	}
	
	// 只有在有非 system 消息时才提取 system 消息
	extractSystem := nonSystemCount > 0
	
	for _, msg := range unified.Messages {
		// Anthropic 格式只接受 user 和 assistant 角色
		// 只在有其他消息时才将 system 消息提取到单独字段
		if msg.Role == "system" && extractSystem {
			// 将 system 消息内容合并到 system 字段
			if msg.Content != nil {
				if contentStr, ok := msg.Content.(string); ok && contentStr != "" {
					if existing, ok := req["system"].(string); ok && existing != "" {
						req["system"] = existing + "\n\n" + contentStr
					} else {
						req["system"] = contentStr
					}
				}
			}
			continue // 跳过此消息，不添加到 messages 数组
		}

		msgMap := map[string]interface{}{
			"role": msg.Role,
		}
		if msg.Content != nil {
			msgMap["content"] = msg.Content
		}
		if len(msg.ToolCalls) > 0 {
			toolUse := []interface{}{}
			for _, tc := range msg.ToolCalls {
				var args map[string]interface{}
				if tc.Function.Arguments != "" {
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				toolUse = append(toolUse, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": args,
				})
			}
			msgMap["content"] = toolUse
		}
		messages = append(messages, msgMap)
	}
	req["messages"] = messages

	// 转换工具
	if len(unified.Tools) > 0 {
		tools := []interface{}{}
		for _, tool := range unified.Tools {
			tools = append(tools, map[string]interface{}{
				"name":         tool.Function.Name,
				"description":  tool.Function.Description,
				"input_schema": tool.Function.Parameters,
			})
		}
		req["tools"] = tools
	}

	return json.Marshal(req)
}

func parseAnthropicResponse(body []byte) (*UnifiedResponse, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	unified := &UnifiedResponse{
		ID:      getString(resp, "id"),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   getString(resp, "model"),
	}

	// 解析内容
	var textContent string
	var toolCalls []UnifiedToolCall

	if content, ok := resp["content"].([]interface{}); ok {
		for _, item := range content {
			itemMap := item.(map[string]interface{})
			itemType := getString(itemMap, "type")

			if itemType == "text" {
				textContent += getString(itemMap, "text")
			} else if itemType == "tool_use" {
				args, _ := json.Marshal(itemMap["input"])
				toolCalls = append(toolCalls, UnifiedToolCall{
					ID:   getString(itemMap, "id"),
					Type: "function",
					Function: UnifiedToolCallFunction{
						Name:      getString(itemMap, "name"),
						Arguments: string(args),
					},
				})
			}
		}
	}

	finishReason := getString(resp, "stop_reason")
	if finishReason == "end_turn" {
		finishReason = "stop"
	} else if finishReason == "tool_use" {
		finishReason = "tool_calls"
	}

	unified.Choices = []UnifiedChoice{{
		Index: 0,
		Message: &UnifiedMessage{
			Role:      "assistant",
			Content:   textContent,
			ToolCalls: toolCalls,
		},
		FinishReason: finishReason,
	}}

	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		unified.Usage = &models.Usage{
			PromptTokens:     int64(getFloat(usage, "input_tokens")),
			CompletionTokens: int64(getFloat(usage, "output_tokens")),
			TotalTokens:      int64(getFloat(usage, "input_tokens") + getFloat(usage, "output_tokens")),
		}
	}

	return unified, nil
}

func formatAnthropicResponse(unified *UnifiedResponse) ([]byte, error) {
	resp := map[string]interface{}{
		"id":      unified.ID,
		"type":    "message",
		"role":    "assistant",
		"model":   unified.Model,
		"content": []interface{}{},
	}

	if len(unified.Choices) > 0 {
		choice := unified.Choices[0]
		content := []interface{}{}

		// 添加文本内容
		if choice.Message.Content != nil {
			if textStr, ok := choice.Message.Content.(string); ok && textStr != "" {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": textStr,
				})
			}
		}

		// 添加工具调用
		for _, tc := range choice.Message.ToolCalls {
			var input map[string]interface{}
			if tc.Function.Arguments != "" {
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
			}
			content = append(content, map[string]interface{}{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Function.Name,
				"input": input,
			})
		}

		resp["content"] = content

		// 转换结束原因
		stopReason := choice.FinishReason
		if stopReason == "stop" {
			stopReason = "end_turn"
		} else if stopReason == "tool_calls" {
			stopReason = "tool_use"
		}
		resp["stop_reason"] = stopReason
	}

	if unified.Usage != nil {
		resp["usage"] = map[string]interface{}{
			"input_tokens":  unified.Usage.PromptTokens,
			"output_tokens": unified.Usage.CompletionTokens,
		}
	}

	return json.Marshal(resp)
}

func parseAnthropicToolCalls(msgMap map[string]interface{}) []UnifiedToolCall {
	var toolCalls []UnifiedToolCall
	if content, ok := msgMap["content"].([]interface{}); ok {
		for _, item := range content {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if getString(itemMap, "type") == "tool_use" {
					args, _ := json.Marshal(itemMap["input"])
					toolCalls = append(toolCalls, UnifiedToolCall{
						ID:   getString(itemMap, "id"),
						Type: "function",
						Function: UnifiedToolCallFunction{
							Name:      getString(itemMap, "name"),
							Arguments: string(args),
						},
					})
				}
			}
		}
	}
	return toolCalls
}
