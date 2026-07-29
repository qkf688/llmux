package anthropic

import (
	"encoding/json"
	"errors"
	"github.com/atopos31/llmio/models"

	"github.com/atopos31/llmio/common"
)

// TransformFromUnified 将统一请求格式转换为 Anthropic 请求格式。
func TransformFromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	if unified == nil {
		return nil, errors.New("unified request cannot be nil")
	}

	reasoning := snapshotReasoning(unified.Messages)
	unified = unified.SanitizedForProvider()
	restoreReasoning(unified.Messages, reasoning)

	req := map[string]interface{}{
		"model":    unified.Model,
		"messages": []interface{}{},
		"stream":   unified.Stream,
	}

	if unified.MaxTokens > 0 {
		req["max_tokens"] = unified.MaxTokens
	} else {
		req["max_tokens"] = 8192
	}
	if unified.Temperature != nil {
		req["temperature"] = *unified.Temperature
	}
	if unified.TopP != nil {
		req["top_p"] = *unified.TopP
	}
	if system := buildSystemValue(unified); system != nil {
		req["system"] = system
	}

	req["messages"] = buildMessages(unified, req)

	if len(unified.Tools) > 0 {
		req["tools"] = buildTools(unified.Tools)
	}

	if unified.Stop != nil {
		if unified.Stop.Single != nil {
			req["stop_sequences"] = []string{*unified.Stop.Single}
		} else if len(unified.Stop.Multiple) > 0 {
			req["stop_sequences"] = unified.Stop.Multiple
		}
	}

	if len(unified.Metadata) > 0 {
		req["metadata"] = unified.Metadata
	}

	if unified.ToolChoice != nil {
		if unified.ToolChoice.StringValue != nil {
			req["tool_choice"] = map[string]interface{}{
				"type": *unified.ToolChoice.StringValue,
			}
		} else if unified.ToolChoice.ObjectValue != nil && unified.ToolChoice.ObjectValue.Function != nil {
			req["tool_choice"] = map[string]interface{}{
				"type": "tool",
				"name": unified.ToolChoice.ObjectValue.Function.Name,
			}
		}
	}

	if unified.ReasoningEffort != nil || unified.ReasoningBudget != nil {
		thinking := map[string]interface{}{
			"type": "enabled",
		}

		if unified.ReasoningBudget != nil {
			thinking["budget_tokens"] = *unified.ReasoningBudget
		} else if unified.ReasoningEffort != nil {
			thinking["budget_tokens"] = reasoningEffortToThinkingBudget(*unified.ReasoningEffort)
		}

		req["thinking"] = thinking
	}

	return json.Marshal(req)
}

func buildMessages(unified *models.UnifiedRequest, req map[string]interface{}) []interface{} {
	messages := make([]interface{}, 0, len(unified.Messages))

	nonSystemCount := 0
	for _, msg := range unified.Messages {
		if msg.Role != "system" {
			nonSystemCount++
		}
	}
	shouldExtractSystem := nonSystemCount > 0

	for _, msg := range unified.Messages {
		if msg.Role == "system" && shouldExtractSystem {
			if parts, ok := msg.Content.([]models.UnifiedMessageContentPart); ok {
				appendSystemParts(req, parts)
			} else if content, ok := msg.Content.(string); ok {
				appendSystemText(req, content)
			}
			continue
		}

		if msg.Role == "tool" {
			messages = append(messages, map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": msg.ToolCallID,
						"content":     buildToolResultContent(msg),
					},
				},
			})
			continue
		}

		msgMap := map[string]interface{}{
			"role": msg.Role,
		}

		if msg.CacheControl != nil {
			msgMap["cache_control"] = map[string]interface{}{
				"type": msg.CacheControl.Type,
			}
		}

		setMessageContent(msgMap, msg)

		messages = append(messages, msgMap)
	}

	return messages
}

// buildToolResultContent 组装 tool_result 的 content。
//
// Anthropic 原生支持块数组形式的 tool_result，所以多模态工具结果（如 computer-use 截图）
// 在这条路径上是原样保真的；string 内容保持 string，不引入无谓的形态变化。
func buildToolResultContent(msg models.UnifiedMessage) interface{} {
	parts, ok := msg.Content.([]models.UnifiedMessageContentPart)
	if !ok {
		if str, ok := msg.Content.(string); ok {
			return str
		}
		return ""
	}

	if blocks := buildContentParts(parts); len(blocks) > 0 {
		return blocks
	}

	// Anthropic 不认的块（如音频）会被 buildContentParts 全部滤掉，
	// 此时退回带占位符的纯文本，避免产出空 tool_result 被上游判 400。
	return msg.GetContentAsStringWithPlaceholders()
}

func buildContentParts(parts []models.UnifiedMessageContentPart) []interface{} {
	contentArray := make([]interface{}, 0, len(parts))

	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Text == nil {
				continue
			}
			textMap := map[string]interface{}{
				"type": "text",
				"text": *part.Text,
			}
			if part.CacheControl != nil {
				textMap["cache_control"] = map[string]interface{}{
					"type": part.CacheControl.Type,
				}
			}
			contentArray = append(contentArray, textMap)
		case "image_url":
			if part.ImageURL == nil {
				continue
			}

			imgMap := map[string]interface{}{
				"type": "image",
			}
			if mediaType, data, ok := common.ParseBase64DataURL(part.ImageURL.URL); ok {
				imgMap["source"] = map[string]interface{}{
					"type":       "base64",
					"media_type": mediaType,
					"data":       data,
				}
			} else {
				imgMap["source"] = map[string]interface{}{
					"type": "url",
					"url":  part.ImageURL.URL,
				}
			}

			if part.CacheControl != nil {
				imgMap["cache_control"] = map[string]interface{}{
					"type": part.CacheControl.Type,
				}
			}
			contentArray = append(contentArray, imgMap)
		}
	}

	return contentArray
}

type messageReasoning struct {
	content      *string
	signature    *string
	redactedData *string
}

// snapshotReasoning 在 SanitizedForProvider 之前留存各消息的推理内容。
//
// SanitizedForProvider 会调 ClearHelpFields 抹掉 Reasoning* 三个字段——
// 对多数上游这是对的（响应侧产生的辅助信息不该回灌给上游），
// 但 Anthropic 的扩展思考要求 assistant 轮原样带回 thinking 与 signature，
// 否则多轮会话被 400 拒绝。故在此处单独留存，sanitize 之后再写回。
func snapshotReasoning(messages []models.UnifiedMessage) []messageReasoning {
	if len(messages) == 0 {
		return nil
	}

	snapshot := make([]messageReasoning, len(messages))
	for i := range messages {
		if text := messages[i].GetReasoningContent(); text != "" {
			snapshot[i].content = &text
		}
		snapshot[i].signature = messages[i].ReasoningSignature
		snapshot[i].redactedData = messages[i].RedactedThinkingData
	}
	return snapshot
}

func restoreReasoning(messages []models.UnifiedMessage, snapshot []messageReasoning) {
	if len(snapshot) != len(messages) {
		return
	}
	for i := range messages {
		messages[i].ReasoningContent = snapshot[i].content
		messages[i].ReasoningSignature = snapshot[i].signature
		messages[i].RedactedThinkingData = snapshot[i].redactedData
	}
}

// setMessageContent 组装一条消息的 content。
//
// thinking / redacted_thinking / 原有内容 / tool_use 是叠加关系而非互相覆盖：
//   - 曾经 ToolCalls 非空时无条件覆盖 content，assistant 轮的文本与图片会静默消失；
//   - 开启扩展思考的多轮会话里 assistant 轮必须带回 thinking 块，
//     否则上游报 400 Expected "thinking" or "redacted_thinking"。
func setMessageContent(msgMap map[string]interface{}, msg models.UnifiedMessage) {
	thinking := buildThinkingBlock(msg)
	redactedThinking := buildRedactedThinkingBlock(msg)
	toolUse := buildToolUseBlocks(msg.ToolCalls)

	// 三者都没有时保持原样透传，不改变既有形态（含 string content 与未知负载）。
	if thinking == nil && redactedThinking == nil && len(toolUse) == 0 {
		if msg.Content == nil {
			return
		}
		if parts, ok := msg.Content.([]models.UnifiedMessageContentPart); ok {
			if contentArray := buildContentParts(parts); len(contentArray) > 0 {
				msgMap["content"] = contentArray
			}
			return
		}
		msgMap["content"] = msg.Content
		return
	}

	contentArray := make([]interface{}, 0, len(toolUse)+3)
	if thinking != nil {
		contentArray = append(contentArray, thinking)
	}
	if redactedThinking != nil {
		contentArray = append(contentArray, redactedThinking)
	}
	contentArray = append(contentArray, buildMessageContentBlocks(msg.Content)...)
	contentArray = append(contentArray, toolUse...)

	if len(contentArray) > 0 {
		msgMap["content"] = contentArray
	}
}

// buildMessageContentBlocks 把 string 与 []UnifiedMessageContentPart 两种内容形态
// 统一成 Anthropic content 块数组。
func buildMessageContentBlocks(content interface{}) []interface{} {
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": v,
			},
		}
	case []models.UnifiedMessageContentPart:
		return buildContentParts(v)
	default:
		return nil
	}
}

// buildThinkingBlock 按响应侧同一套约定（ReasoningContent + ReasoningSignature）
// 还原 thinking 块；无推理内容时返回 nil。
func buildThinkingBlock(msg models.UnifiedMessage) map[string]interface{} {
	reasoning := msg.GetReasoningContent()
	if reasoning == "" {
		return nil
	}

	thinking := map[string]interface{}{
		"type":     "thinking",
		"thinking": reasoning,
	}
	if msg.ReasoningSignature != nil && *msg.ReasoningSignature != "" {
		thinking["signature"] = *msg.ReasoningSignature
	}
	return thinking
}

// buildRedactedThinkingBlock 原样还原 Anthropic 不透明思考密文。
func buildRedactedThinkingBlock(msg models.UnifiedMessage) map[string]interface{} {
	if msg.RedactedThinkingData == nil || *msg.RedactedThinkingData == "" {
		return nil
	}
	return map[string]interface{}{
		"type": "redacted_thinking",
		"data": *msg.RedactedThinkingData,
	}
}

func buildToolUseBlocks(toolCalls []models.UnifiedToolCall) []interface{} {
	if len(toolCalls) == 0 {
		return nil
	}

	blocks := make([]interface{}, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		args := map[string]interface{}{}
		if toolCall.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				args = map[string]interface{}{}
			}
		}

		blocks = append(blocks, map[string]interface{}{
			"type":  "tool_use",
			"id":    toolCall.ID,
			"name":  toolCall.Function.Name,
			"input": args,
		})
	}
	return blocks
}

func buildTools(tools []models.UnifiedTool) []interface{} {
	result := make([]interface{}, 0, len(tools))
	for _, tool := range tools {
		toolMap := map[string]interface{}{
			"name":         tool.Function.Name,
			"description":  tool.Function.Description,
			"input_schema": tool.Function.Parameters,
		}

		if tool.CacheControl != nil {
			toolMap["cache_control"] = map[string]interface{}{
				"type": tool.CacheControl.Type,
			}
		}

		result = append(result, toolMap)
	}
	return result
}
