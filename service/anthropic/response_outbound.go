package anthropic

import (
	"encoding/json"
	"errors"
	"github.com/qkf688/llmux/models"
	"strings"
)

// FormatResponse 将统一响应格式转换为 Anthropic 响应格式。
func FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	if unified == nil {
		return nil, errors.New("unified response cannot be nil")
	}

	// 纯错误响应走官方错误信封，而不是套进一个 content 为空的 message 壳子
	// （后者形式合法但语义空洞，客户端会当成「模型什么都没说」而非「上游报错了」）。
	// 守卫沿用 openai / openai-res 出站的同一判据：Error 与 Choices 并存时以内容为主。
	if unified.Error != nil && len(unified.Choices) == 0 {
		return formatErrorEnvelope(unified.Error)
	}

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

		if choice.Message != nil {
			reasoning := choice.Message.GetReasoningContent()
			adjustedContent := stripReasoningPrefix(choice.Message.Content, reasoning)

			if thinking := buildThinkingBlock(*choice.Message); thinking != nil {
				content = append(content, thinking)
			}
			if redactedThinking := buildRedactedThinkingBlock(*choice.Message); redactedThinking != nil {
				content = append(content, redactedThinking)
			}

			if parts, ok := adjustedContent.([]models.UnifiedMessageContentPart); ok {
				content = append(content, buildContentParts(parts)...)
			} else if textStr, ok := adjustedContent.(string); ok && textStr != "" {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": textStr,
				})
			} else if adjustedContent != nil {
				// Best-effort: keep unknown content shapes as a JSON-encoded text block.
				if b, err := json.Marshal(adjustedContent); err == nil && len(b) > 0 {
					content = append(content, map[string]interface{}{
						"type": "text",
						"text": string(b),
					})
				}
			}

			for _, toolCall := range choice.Message.ToolCalls {
				var input map[string]interface{}
				if toolCall.Function.Arguments != "" {
					_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &input)
				}

				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    toolCall.ID,
					"name":  toolCall.Function.Name,
					"input": input,
				})
			}
		}

		resp["content"] = content

		stopReason := choice.FinishReason
		switch stopReason {
		case "stop":
			stopReason = "end_turn"
		case "tool_calls":
			stopReason = "tool_use"
		case "length":
			stopReason = "max_tokens"
		}
		resp["stop_reason"] = stopReason
	}

	if unified.Usage != nil {
		usage := map[string]interface{}{
			"input_tokens":  unified.Usage.PromptTokens,
			"output_tokens": unified.Usage.CompletionTokens,
		}
		// 按 Anthropic Messages 官方 usage 形状逐键决定，不照搬统一模型：
		// ① **无 total_tokens**——该协议不定义此键，写出去是非标准扩展，故 TotalTokens 刻意丢弃；
		// ② cached 命中数的官方键是顶层 cache_read_input_tokens（非 OpenAI 的嵌套 details）；
		// ③ reasoning / audio 明细在本协议无槽位（thinking 计入 output_tokens），按协议能力裁剪。
		// 仅在 >0 时写：显式 0 会被下游读成「上游报告了无缓存命中」，与「键缺失＝未知」是两种语义
		// ——与流式侧 responses_to_anthropic.go 的 message_delta usage 判据保持同一口径。
		if cached := unified.Usage.PromptTokensDetails.CachedTokens; cached > 0 {
			usage["cache_read_input_tokens"] = cached
		}
		resp["usage"] = usage
	}

	return json.Marshal(resp)
}

// formatErrorEnvelope 产出 Anthropic 官方错误形状：
//
//	{"type":"error","error":{"type":...,"message":...},"request_id":"req_..."}
//
// 按官方契约逐键决定，不照搬统一模型（与本文件 usage 段同一口径）：
// ① inner error 官方只保证 type 与 message 两键，故统一模型的 Code / Param
// **刻意丢弃**——写出去是非标准扩展键，严格校验的客户端会当成协议违规；把它们
// 拼进 message 又会污染那段人类可读文案（官方明说 message 不可 pattern-match）；
// ② request_id 是顶层键（不在 error 对象内），仅在上游给了值时写：写出空串会让
// 客户端以为拿到了一个可用于联系支持的 ID，而「键缺失＝上游没给」才是真实语义。
func formatErrorEnvelope(respErr *models.ResponseError) ([]byte, error) {
	envelope := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    mapAnthropicErrorType(respErr.Detail.Type),
			"message": respErr.Detail.Message,
		},
	}
	if respErr.Detail.RequestID != "" {
		envelope["request_id"] = respErr.Detail.RequestID
	}

	return json.Marshal(envelope)
}

func stripReasoningPrefix(content interface{}, reasoning string) interface{} {
	if content == nil || reasoning == "" {
		return content
	}

	prefix := reasoning + "\n\n---\n\n"

	switch v := content.(type) {
	case string:
		if v == reasoning {
			return ""
		}
		if strings.HasPrefix(v, prefix) {
			return strings.TrimPrefix(v, prefix)
		}
		return v

	case []models.UnifiedMessageContentPart:
		if len(v) == 0 {
			return v
		}
		first := v[0]
		if first.Type != "text" || first.Text == nil {
			return v
		}
		if *first.Text == reasoning || *first.Text == prefix {
			return v[1:]
		}
		return v

	default:
		return v
	}
}
