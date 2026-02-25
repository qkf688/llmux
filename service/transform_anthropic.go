package service

import (
	"encoding/json"
	"strings"
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

	// 转换消息
	if messages, ok := req["messages"].([]interface{}); ok {
		for _, msg := range messages {
			msgMap := msg.(map[string]interface{})
			unifiedMsg := UnifiedMessage{
				Role:      getString(msgMap, "role"),
				Content:   msgMap["content"],
				ToolCalls: parseAnthropicToolCalls(msgMap),
			}

			// 解析 tool_result 类型的内容
			if content, ok := msgMap["content"].([]interface{}); ok {
				for _, item := range content {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if getString(itemMap, "type") == "tool_result" {
							unifiedMsg.ToolCallID = getString(itemMap, "tool_use_id")
							break
						}
					}
				}
			}

			// 阶段 1: 解析消息级别的缓存控制
			if cacheControl, ok := msgMap["cache_control"].(map[string]interface{}); ok {
				unifiedMsg.CacheControl = &CacheControl{
					Type: getString(cacheControl, "type"),
				}
			}

			unified.Messages = append(unified.Messages, unifiedMsg)
		}
	}

	// 转换工具
	if tools, ok := req["tools"].([]interface{}); ok {
		for _, tool := range tools {
			toolMap := tool.(map[string]interface{})
			unifiedTool := UnifiedTool{
				Type: "function",
				Function: UnifiedFunc{
					Name:        getString(toolMap, "name"),
					Description: getString(toolMap, "description"),
					Parameters:  toolMap["input_schema"],
				},
			}

			// 阶段 1: 解析工具级别的缓存控制
			if cacheControl, ok := toolMap["cache_control"].(map[string]interface{}); ok {
				unifiedTool.CacheControl = &CacheControl{
					Type: getString(cacheControl, "type"),
				}
			}

			unified.Tools = append(unified.Tools, unifiedTool)
		}
	}

	// 阶段 1: 解析 Anthropic 特有字段
	if stopSeqs := getStringArray(req, "stop_sequences"); len(stopSeqs) > 0 {
		unified.Stop = &UnifiedStop{Multiple: stopSeqs}
	}

	if metadata := getStringMap(req, "metadata"); len(metadata) > 0 {
		unified.Metadata = metadata
	}

	return unified, nil
}

// extractSystem 支持 Anthropic system 为字符串或数组的情况，确保 system 不被静默丢失
func extractSystem(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			switch m := item.(type) {
			case map[string]interface{}:
				if text, ok := m["text"].(string); ok && text != "" {
					parts = append(parts, text)
				} else if content, ok := m["content"].(string); ok && content != "" {
					parts = append(parts, content)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
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
		req["max_tokens"] = 8192 // 设置默认值
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

		// 处理 tool 角色消息，转换为 Anthropic 的 tool_result 格式
		if msg.Role == "tool" {
			contentArray := []interface{}{}
			var contentStr string
			if msg.Content != nil {
				if str, ok := msg.Content.(string); ok {
					contentStr = str
				}
			}
			contentArray = append(contentArray, map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": msg.ToolCallID,
				"content":     contentStr,
			})
			messages = append(messages, map[string]interface{}{
				"role":    "user",
				"content": contentArray,
			})
			continue
		}

		msgMap := map[string]interface{}{
			"role": msg.Role,
		}

		// 阶段 1: 添加消息级别的缓存控制
		if msg.CacheControl != nil {
			msgMap["cache_control"] = map[string]interface{}{
				"type": msg.CacheControl.Type,
			}
		}
		if msg.Content != nil {
			// 阶段 3: 处理多模态内容
			if parts, ok := msg.Content.([]UnifiedMessageContentPart); ok {
				// 多模态内容 - Anthropic 格式
				contentArray := make([]interface{}, 0, len(parts))
				for _, part := range parts {
					switch part.Type {
					case "text":
						if part.Text != nil {
							textMap := map[string]interface{}{
								"type": "text",
								"text": *part.Text,
							}
							// 阶段 1: 添加内容部分级别的缓存控制
							if part.CacheControl != nil {
								textMap["cache_control"] = map[string]interface{}{
									"type": part.CacheControl.Type,
								}
							}
							contentArray = append(contentArray, textMap)
						}
					case "image_url":
						if part.ImageURL != nil {
							// Anthropic 使用 image 类型
							imgMap := map[string]interface{}{
								"type": "image",
							}
							// 判断是 URL 还是 base64
							if strings.HasPrefix(part.ImageURL.URL, "data:") {
								// Base64 格式
								imgMap["source"] = map[string]interface{}{
									"type": "base64",
									"data": strings.TrimPrefix(part.ImageURL.URL, "data:image/jpeg;base64,"),
								}
							} else {
								// URL 格式
								imgMap["source"] = map[string]interface{}{
									"type": "url",
									"url":  part.ImageURL.URL,
								}
							}
							// 阶段 1: 添加内容部分级别的缓存控制
							if part.CacheControl != nil {
								imgMap["cache_control"] = map[string]interface{}{
									"type": part.CacheControl.Type,
								}
							}
							contentArray = append(contentArray, imgMap)
						}
					// Anthropic 不支持 input_audio，跳过
					}
				}
				if len(contentArray) > 0 {
					msgMap["content"] = contentArray
				}
			} else {
				// 纯文本
				msgMap["content"] = msg.Content
			}
		}
		if len(msg.ToolCalls) > 0 {
			// 如果有工具调用，需要构建包含文本和工具调用的内容数组
			contentArray := []interface{}{}

			// 如果有文本内容，先添加文本块
			if msg.Content != nil {
				if contentStr, ok := msg.Content.(string); ok && contentStr != "" {
					contentArray = append(contentArray, map[string]interface{}{
						"type": "text",
						"text": contentStr,
					})
				}
			}

			// 添加工具调用块
			for _, tc := range msg.ToolCalls {
				var args map[string]interface{}
				if tc.Function.Arguments != "" {
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
						args = map[string]interface{}{}
					}
				} else {
					args = map[string]interface{}{}
				}
				contentArray = append(contentArray, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": args,
				})
			}
			msgMap["content"] = contentArray
		}
		messages = append(messages, msgMap)
	}
	req["messages"] = messages

	// 转换工具
	if len(unified.Tools) > 0 {
		tools := []interface{}{}
		for _, tool := range unified.Tools {
			toolMap := map[string]interface{}{
				"name":         tool.Function.Name,
				"description":  tool.Function.Description,
				"input_schema": tool.Function.Parameters,
			}

			// 阶段 1: 添加工具级别的缓存控制
			if tool.CacheControl != nil {
				toolMap["cache_control"] = map[string]interface{}{
					"type": tool.CacheControl.Type,
				}
			}

			tools = append(tools, toolMap)
		}
		req["tools"] = tools
	}

	// 阶段 1: 映射兼容字段
	// Anthropic 支持 stop_sequences (只支持数组)
	if unified.Stop != nil {
		if unified.Stop.Single != nil {
			req["stop_sequences"] = []string{*unified.Stop.Single}
		} else if len(unified.Stop.Multiple) > 0 {
			req["stop_sequences"] = unified.Stop.Multiple
		}
	}

	// Anthropic 支持 metadata
	if unified.Metadata != nil && len(unified.Metadata) > 0 {
		req["metadata"] = unified.Metadata
	}

	// 注意: Anthropic 不支持以下字段，静默忽略
	// - frequency_penalty
	// - presence_penalty
	// - seed
	// - logit_bias
	// - user
	// - logprobs
	// - top_logprobs
	// - max_completion_tokens
	// - store

	// 阶段 2: 映射兼容字段
	// Anthropic 不支持 response_format (JSON Schema)，但支持部分功能
	// 可以通过 system prompt 引导模型输出 JSON
	// 这里静默忽略，不报错

	// Anthropic 支持 tool_choice
	if unified.ToolChoice != nil {
		if unified.ToolChoice.StringValue != nil {
			// "auto", "any", "none" 等
			req["tool_choice"] = map[string]interface{}{
				"type": *unified.ToolChoice.StringValue,
			}
		} else if unified.ToolChoice.ObjectValue != nil && unified.ToolChoice.ObjectValue.Function != nil {
			// 指定特定工具
			req["tool_choice"] = map[string]interface{}{
				"type": "tool",
				"name": unified.ToolChoice.ObjectValue.Function.Name,
			}
		}
	}

	// Anthropic 不支持 parallel_tool_calls，静默忽略
	// Anthropic 不支持 stream_options，静默忽略

	// 阶段 3: 多模态支持
	// Anthropic 支持图像 (通过 content 数组中的 image 类型)
	// Anthropic 不支持 modalities 字段，静默忽略
	// Anthropic 不支持 audio 输出配置，静默忽略
	// Anthropic 不支持 input_audio，静默忽略

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
					var argsStr string
					if input := itemMap["input"]; input != nil {
						if inputMap, ok := input.(map[string]interface{}); ok {
							if argsBytes, err := json.Marshal(inputMap); err == nil {
								argsStr = string(argsBytes)
							}
						}
					}
					if argsStr == "" {
						argsStr = "{}"
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
			}
		}
	}
	return toolCalls
}
