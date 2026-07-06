package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/transform/shared"
)

// ToUnified converts an OpenAI Chat Completion request body into the unified
// request representation.
func ToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	var req openAIChatCompletionRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, err
	}

	unified := &models.UnifiedRequest{
		Model:  req.Model.Value,
		Stream: req.Stream.Value,
	}

	if req.MaxTokens.Set {
		unified.MaxTokens = req.MaxTokens.Value
	}
	if req.Temperature.Set {
		unified.Temperature = &req.Temperature.Value
	}
	if req.TopP.Set {
		unified.TopP = &req.TopP.Value
	}

	if len(req.Messages) > 0 {
		var system string
		unified.Messages, system = parseOpenAIChatMessages(req.Messages)
		unified.System = system
	}

	if len(req.Tools) > 0 {
		unified.Tools = parseOpenAIChatTools(req.Tools)
	}

	// 处理 reasoning_effort 参数
	if req.ReasoningEffort.Set && req.ReasoningEffort.Value != "" {
		// 检查是否启用映射
		if shared.GetReasoningEffortMappingEnabled(ctx) {
			normalized := shared.NormalizeReasoningEffort(ctx, req.ReasoningEffort.Value)
			unified.ReasoningEffort = &normalized
		} else {
			// 不启用映射时直接透传
			effort := req.ReasoningEffort.Value
			unified.ReasoningEffort = &effort
		}
	}

	// 阶段 1: 解析基础高级参数
	if req.FrequencyPenalty.Set {
		unified.FrequencyPenalty = &req.FrequencyPenalty.Value
	}
	if req.PresencePenalty.Set {
		unified.PresencePenalty = &req.PresencePenalty.Value
	}
	if req.Seed.Set {
		unified.Seed = &req.Seed.Value
	}
	if req.LogitBias.Set {
		unified.LogitBias = req.LogitBias.Value
	}
	if req.User.Set {
		unified.User = &req.User.Value
	}
	if req.Metadata.Set {
		unified.Metadata = req.Metadata.Value
	}
	if req.Logprobs.Set {
		unified.Logprobs = &req.Logprobs.Value
	}
	if req.TopLogprobs.Set {
		unified.TopLogprobs = &req.TopLogprobs.Value
	}
	if req.MaxCompletionTokens.Set {
		unified.MaxCompletionTokens = &req.MaxCompletionTokens.Value
	}
	if req.Store.Set {
		unified.Store = &req.Store.Value
	}

	// 处理 stop (可能是 string 或 []string)
	unified.Stop = parseOpenAIChatStop(req.Stop)

	// 阶段 2: 解析响应格式和工具增强参数
	unified.ResponseFormat = parseOpenAIChatResponseFormat(req.ResponseFormat)
	unified.ToolChoice = parseOpenAIChatToolChoice(req.ToolChoice)
	if req.ParallelToolCalls.Set {
		unified.ParallelToolCalls = &req.ParallelToolCalls.Value
	}
	unified.StreamOptions = parseOpenAIChatStreamOptions(req.StreamOptions)

	// 阶段 3: 解析多模态参数
	if req.Modalities.Set {
		unified.Modalities = req.Modalities.Value
	}
	unified.Audio = parseOpenAIChatAudio(req.Audio)

	return unified, nil
}

// FromUnified converts a unified request into an OpenAI Chat Completion
// request body.
func FromUnified(unified *models.UnifiedRequest) ([]byte, error) {
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
			if parts, ok := msg.Content.([]models.UnifiedMessageContentPart); ok {
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
