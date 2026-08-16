package anthropic

import (
	"encoding/json"
	"fmt"
	"github.com/qkf688/llmux/models"

	"github.com/qkf688/llmux/common/maputil"
)

// ParseResponse 将 Anthropic 响应格式转换为统一格式。
func ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	unified := &models.UnifiedResponse{
		ID:      maputil.String(resp, "id"),
		Object:  "chat.completion",
		Created: 0,
		Model:   maputil.String(resp, "model"),
	}

	finishReason := maputil.String(resp, "stop_reason")
	switch finishReason {
	case "end_turn", "stop_sequence":
		finishReason = "stop"
	case "tool_use":
		finishReason = "tool_calls"
	case "max_tokens":
		finishReason = "length"
	}

	content, _ := parseMessageContentAndToolResults(resp["content"])
	toolCalls := parseToolCalls(resp["content"])

	reasoningText, reasoningSig, redactedData := extractThinking(resp["content"])
	if reasoningText != "" {
		switch v := content.(type) {
		case string:
			if v != "" {
				content = reasoningText + "\n\n---\n\n" + v
			} else {
				content = reasoningText
			}
		case []models.UnifiedMessageContentPart:
			prefix := reasoningText
			if len(v) > 0 {
				prefix += "\n\n---\n\n"
			}
			v2 := make([]models.UnifiedMessageContentPart, 0, len(v)+1)
			v2 = append(v2, models.UnifiedMessageContentPart{Type: "text", Text: &prefix})
			v2 = append(v2, v...)
			content = v2
		case nil:
			content = reasoningText
		default:
			// Keep behavior: best-effort stringify unknown content shapes.
			content = fmt.Sprintf("%v", v)
		}
	}

	message := &models.UnifiedMessage{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	}
	if reasoningText != "" {
		message.ReasoningContent = &reasoningText
	}
	if reasoningSig != "" {
		message.ReasoningSignature = &reasoningSig
	}
	if redactedData != "" {
		message.RedactedThinkingData = &redactedData
	}

	unified.Choices = []models.UnifiedChoice{{
		Index:        0,
		Message:      message,
		FinishReason: finishReason,
	}}

	if usage, ok := asMap(resp["usage"]); ok {
		// 归一走 models.UsageFromMap：anthropic 原生 input/output/cache_read 字段、
		// kimi 一类混合返回的 openai 兼容字段、上游给定的 total 口径，全部由候选路径表
		// 统一吸收，与流式侧信道 / chat processer 落库同源，不再本地手写一套解析。
		u := models.UsageFromMap(usage)
		unified.Usage = &u
	}

	return unified, nil
}

func extractThinking(raw interface{}) (thinking string, signature string, redactedData string) {
	items, ok := asSlice(raw)
	if !ok {
		return "", "", ""
	}

	for _, item := range items {
		itemMap, ok := asMap(item)
		if !ok {
			continue
		}
		switch maputil.String(itemMap, "type") {
		case "thinking":
			thinking = maputil.String(itemMap, "thinking")
			signature = maputil.String(itemMap, "signature")
		case "redacted_thinking":
			if redactedData == "" {
				redactedData = maputil.String(itemMap, "data")
			}
		}
	}

	return thinking, signature, redactedData
}
