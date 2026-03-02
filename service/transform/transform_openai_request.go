package transform

import (
	"context"
	"encoding/json"
	"fmt"
)

func TransformOpenAIToUnified(ctx context.Context, rawBody []byte) (*UnifiedRequest, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, err
	}

	unified := &UnifiedRequest{
		Model:  getString(req, "model"),
		Stream: getBool(req, "stream"),
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
		// 先统计非 system 消息的数量
		nonSystemCount := 0
		for _, msg := range messages {
			msgMap, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}
			if getString(msgMap, "role") != "system" {
				nonSystemCount++
			}
		}

		// 只有在有非 system 消息时才提取 system 消息
		// 否则保持原样以便提供商返回合适的错误
		extractSystem := nonSystemCount > 0

		for _, msg := range messages {
			msgMap, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}
			role := getString(msgMap, "role")

			// 只在有其他消息时才提取 system 消息
			if role == "system" && extractSystem {
				if content, ok := msgMap["content"].(string); ok && content != "" {
					if unified.System != "" {
						unified.System += "\n\n" + content
					} else {
						unified.System = content
					}
				}
				continue // 不将 system 消息添加到 messages 数组
			}

			msg := UnifiedMessage{
				Role:      role,
				Content:   msgMap["content"],
				ToolCalls: parseOpenAIToolCalls(msgMap),
			}

			// 阶段 3: 解析多模态内容
			if content, ok := msgMap["content"]; ok && content != nil {
				// 尝试解析为多模态内容数组
				if contentArray, ok := content.([]interface{}); ok {
					parts := make([]UnifiedMessageContentPart, 0, len(contentArray))
					for _, item := range contentArray {
						if itemMap, ok := item.(map[string]interface{}); ok {
							part := UnifiedMessageContentPart{
								Type: getString(itemMap, "type"),
							}

							switch part.Type {
							case "text":
								if text, ok := itemMap["text"].(string); ok {
									part.Text = &text
								}
							case "image_url":
								if imgMap, ok := itemMap["image_url"].(map[string]interface{}); ok {
									part.ImageURL = &UnifiedImageURL{
										URL:    getString(imgMap, "url"),
										Detail: getStringPtr(imgMap, "detail"),
									}
								}
							case "input_audio":
								if audioMap, ok := itemMap["input_audio"].(map[string]interface{}); ok {
									part.InputAudio = &UnifiedInputAudio{
										Data:   getString(audioMap, "data"),
										Format: getString(audioMap, "format"),
									}
								}
							}

							parts = append(parts, part)
						}
					}
					if len(parts) > 0 {
						msg.Content = parts
					}
				}
				// 否则保持原样 (string 或其他类型)
			}

			// 处理 tool 角色消息的 tool_call_id
			if role == "tool" {
				if toolCallID, ok := msgMap["tool_call_id"].(string); ok {
					msg.ToolCallID = toolCallID
				}
			}

			unified.Messages = append(unified.Messages, msg)
		}
	}

	// 转换工具
	if tools, ok := req["tools"].([]interface{}); ok {
		for _, tool := range tools {
			toolMap, ok := tool.(map[string]interface{})
			if !ok {
				continue
			}
			if funcMap, ok := toolMap["function"].(map[string]interface{}); ok {
				unified.Tools = append(unified.Tools, UnifiedTool{
					Type: "function",
					Function: UnifiedFunc{
						Name:        getString(funcMap, "name"),
						Description: getString(funcMap, "description"),
						Parameters:  funcMap["parameters"],
					},
				})
			}
		}
	}

	// 处理 reasoning_effort 参数
	if effort, ok := req["reasoning_effort"].(string); ok && effort != "" {
		// 检查是否启用映射
		if getReasoningEffortMappingEnabled(ctx) {
			normalized := normalizeReasoningEffort(ctx, effort)
			unified.ReasoningEffort = &normalized
		} else {
			// 不启用映射时直接透传
			unified.ReasoningEffort = &effort
		}
	}

	// 阶段 1: 解析基础高级参数
	unified.FrequencyPenalty = getFloat64Ptr(req, "frequency_penalty")
	unified.PresencePenalty = getFloat64Ptr(req, "presence_penalty")
	unified.Seed = getInt64Ptr(req, "seed")
	unified.LogitBias = getIntMap(req, "logit_bias")
	unified.User = getStringPtr(req, "user")
	unified.Metadata = getStringMap(req, "metadata")
	unified.Logprobs = getBoolPtr(req, "logprobs")
	unified.TopLogprobs = getInt64Ptr(req, "top_logprobs")
	unified.MaxCompletionTokens = getInt64Ptr(req, "max_completion_tokens")
	unified.Store = getBoolPtr(req, "store")

	// 处理 stop (可能是 string 或 []string)
	if stopVal, ok := req["stop"]; ok && stopVal != nil {
		unified.Stop = &UnifiedStop{}
		switch v := stopVal.(type) {
		case string:
			unified.Stop.Single = &v
		case []interface{}:
			unified.Stop.Multiple = getStringArray(req, "stop")
		}
	}

	// 阶段 2: 解析响应格式和工具增强参数
	// 解析 response_format
	if rfVal, ok := req["response_format"].(map[string]interface{}); ok {
		unified.ResponseFormat = &UnifiedResponseFormat{
			Type: getString(rfVal, "type"),
		}
		if schema, ok := rfVal["json_schema"]; ok {
			if schemaBytes, err := json.Marshal(schema); err == nil {
				unified.ResponseFormat.JSONSchema = schemaBytes
			}
		}
	}

	// 解析 tool_choice
	if tcVal, ok := req["tool_choice"]; ok && tcVal != nil {
		unified.ToolChoice = &UnifiedToolChoice{}
		switch v := tcVal.(type) {
		case string:
			unified.ToolChoice.StringValue = &v
		case map[string]interface{}:
			obj := UnifiedToolChoiceObject{
				Type: getString(v, "type"),
			}
			if funcMap, ok := v["function"].(map[string]interface{}); ok {
				obj.Function = &UnifiedToolChoiceFunction{
					Name: getString(funcMap, "name"),
				}
			}
			unified.ToolChoice.ObjectValue = &obj
		}
	}

	unified.ParallelToolCalls = getBoolPtr(req, "parallel_tool_calls")

	// 解析 stream_options
	if soVal, ok := req["stream_options"].(map[string]interface{}); ok {
		unified.StreamOptions = &UnifiedStreamOptions{
			IncludeUsage: getBool(soVal, "include_usage"),
		}
	}

	// 阶段 3: 解析多模态参数
	unified.Modalities = getStringArray(req, "modalities")

	// 解析 audio 配置
	if audioVal, ok := req["audio"].(map[string]interface{}); ok {
		unified.Audio = &UnifiedAudio{
			Voice:  getString(audioVal, "voice"),
			Format: getString(audioVal, "format"),
		}
	}

	return unified, nil
}

// TransformUnifiedToOpenAI 将统一格式转换为 OpenAI 格式
func TransformUnifiedToOpenAI(unified *UnifiedRequest) ([]byte, error) {
	if unified == nil {
		return nil, fmt.Errorf("unified request cannot be nil")
	}

	unified = unified.SanitizedForProvider()

	req := map[string]interface{}{
		"model":    unified.Model,
		"messages": []interface{}{},
		"stream":   unified.Stream,
	}

	if unified.MaxTokens > 0 {
		req["max_tokens"] = unified.MaxTokens
	}
	if unified.Temperature != nil {
		req["temperature"] = *unified.Temperature
	}
	if unified.TopP != nil {
		req["top_p"] = *unified.TopP
	}

	// 转换消息
	messages := []interface{}{}

	// 如果有 system 字段,添加为第一条消息
	if unified.System != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": unified.System,
		})
	}

	for _, msg := range unified.Messages {
		msgMap := map[string]interface{}{
			"role": msg.Role,
		}
		if msg.Content != nil {
			// 阶段 3: 处理多模态内容
			if parts, ok := msg.Content.([]UnifiedMessageContentPart); ok {
				// 多模态内容
				contentArray := make([]interface{}, 0, len(parts))
				for _, part := range parts {
					partMap := map[string]interface{}{
						"type": part.Type,
					}

					switch part.Type {
					case "text":
						if part.Text != nil {
							partMap["text"] = *part.Text
						}
					case "image_url":
						if part.ImageURL != nil {
							imgMap := map[string]interface{}{
								"url": part.ImageURL.URL,
							}
							if part.ImageURL.Detail != nil {
								imgMap["detail"] = *part.ImageURL.Detail
							}
							partMap["image_url"] = imgMap
						}
					case "input_audio":
						if part.InputAudio != nil {
							partMap["input_audio"] = map[string]interface{}{
								"data":   part.InputAudio.Data,
								"format": part.InputAudio.Format,
							}
						}
					}

					contentArray = append(contentArray, partMap)
				}
				msgMap["content"] = contentArray
			} else {
				// 纯文本或其他类型
				msgMap["content"] = msg.Content
			}
		}
		if len(msg.ToolCalls) > 0 {
			toolCalls := []interface{}{}
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			msgMap["tool_calls"] = toolCalls
		}
		// 处理 tool 角色消息的 tool_call_id
		if msg.Role == "tool" && msg.ToolCallID != "" {
			msgMap["tool_call_id"] = msg.ToolCallID
		}
		messages = append(messages, msgMap)
	}

	// 验证 messages 不为空
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty: OpenAI API requires at least one message")
	}

	req["messages"] = messages

	// 转换工具
	if len(unified.Tools) > 0 {
		tools := []interface{}{}
		for _, tool := range unified.Tools {
			tools = append(tools, map[string]interface{}{
				"type": tool.Type,
				"function": map[string]interface{}{
					"name":        tool.Function.Name,
					"description": tool.Function.Description,
					"parameters":  tool.Function.Parameters,
				},
			})
		}
		req["tools"] = tools
	}

	// 输出 reasoning_effort 参数
	if unified.ReasoningEffort != nil {
		req["reasoning_effort"] = *unified.ReasoningEffort
	}

	// 阶段 1: 输出基础高级参数
	if unified.FrequencyPenalty != nil {
		req["frequency_penalty"] = *unified.FrequencyPenalty
	}
	if unified.PresencePenalty != nil {
		req["presence_penalty"] = *unified.PresencePenalty
	}
	if unified.Seed != nil {
		req["seed"] = *unified.Seed
	}
	if unified.LogitBias != nil && len(unified.LogitBias) > 0 {
		req["logit_bias"] = unified.LogitBias
	}
	if unified.Stop != nil {
		if unified.Stop.Single != nil {
			req["stop"] = *unified.Stop.Single
		} else if len(unified.Stop.Multiple) > 0 {
			req["stop"] = unified.Stop.Multiple
		}
	}
	if unified.User != nil {
		req["user"] = *unified.User
	}
	if unified.Metadata != nil && len(unified.Metadata) > 0 {
		req["metadata"] = unified.Metadata
	}
	if unified.Logprobs != nil {
		req["logprobs"] = *unified.Logprobs
	}
	if unified.TopLogprobs != nil {
		req["top_logprobs"] = *unified.TopLogprobs
	}
	if unified.MaxCompletionTokens != nil {
		req["max_completion_tokens"] = *unified.MaxCompletionTokens
	}
	if unified.Store != nil {
		req["store"] = *unified.Store
	}

	// 阶段 2: 输出响应格式和工具增强参数
	if unified.ResponseFormat != nil {
		rfMap := map[string]interface{}{
			"type": unified.ResponseFormat.Type,
		}
		if len(unified.ResponseFormat.JSONSchema) > 0 {
			var schema interface{}
			if err := json.Unmarshal(unified.ResponseFormat.JSONSchema, &schema); err == nil {
				rfMap["json_schema"] = schema
			}
		}
		req["response_format"] = rfMap
	}

	if unified.ToolChoice != nil {
		if unified.ToolChoice.StringValue != nil {
			req["tool_choice"] = *unified.ToolChoice.StringValue
		} else if unified.ToolChoice.ObjectValue != nil {
			tcMap := map[string]interface{}{
				"type": unified.ToolChoice.ObjectValue.Type,
			}
			if unified.ToolChoice.ObjectValue.Function != nil {
				tcMap["function"] = map[string]interface{}{
					"name": unified.ToolChoice.ObjectValue.Function.Name,
				}
			}
			req["tool_choice"] = tcMap
		}
	}

	if unified.ParallelToolCalls != nil {
		req["parallel_tool_calls"] = *unified.ParallelToolCalls
	}

	if unified.StreamOptions != nil {
		req["stream_options"] = map[string]interface{}{
			"include_usage": unified.StreamOptions.IncludeUsage,
		}
	} else if unified.Stream {
		// 如果是流式但没有设置 StreamOptions，使用默认值
		req["stream_options"] = map[string]interface{}{"include_usage": true}
	}

	// 阶段 3: 输出多模态参数
	if len(unified.Modalities) > 0 {
		req["modalities"] = unified.Modalities
	}

	if unified.Audio != nil {
		audioMap := map[string]interface{}{}
		if unified.Audio.Voice != "" {
			audioMap["voice"] = unified.Audio.Voice
		}
		if unified.Audio.Format != "" {
			audioMap["format"] = unified.Audio.Format
		}
		if len(audioMap) > 0 {
			req["audio"] = audioMap
		}
	}

	return json.Marshal(req)
}
