package anthropic

import (
	"encoding/json"
	"strings"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/shared"
)

// parseSystem parses Anthropic's "system" field, which can be either a string or an array of text blocks.
// It returns:
// - the extracted plain text (used by internal logic and other providers),
// - the structured blocks (used to preserve Anthropic's system array format when round-tripping).
//
// 数组分支刻意用 []json.RawMessage + 错误判定，而非 shared.RawArray：后者吞掉
// 「不是数组」这个错误，会让标量 system（如 `"system":123`）与空数组 `[]` 落到同一
// 分支，从而把「非数组 → SystemParts 为 nil」误改成「返回空切片」。
func parseSystem(raw json.RawMessage) (string, []models.UnifiedMessageContentPart) {
	if len(raw) == 0 || shared.IsJSONNull(raw) {
		return "", nil
	}
	if value, ok := shared.RawString(raw); ok {
		return value, nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return "", nil
	}
	parts := parseSystemParts(items)
	return systemPartsToText(parts), parts
}

func parseSystemParts(items []json.RawMessage) []models.UnifiedMessageContentPart {
	parts := make([]models.UnifiedMessageContentPart, 0, len(items))
	for _, item := range items {
		var block anthropicTextBlock
		if !shared.DecodeJSONObject(item, &block) {
			continue
		}
		if block.Type.Value != "text" {
			continue
		}

		text := block.Text.Value
		if text == "" {
			text = block.Content.Value
		}
		if text == "" {
			continue
		}

		parts = append(parts, models.UnifiedMessageContentPart{
			Type:         "text",
			Text:         &text,
			CacheControl: parseRawCacheControl(block.CacheControl),
		})
	}
	return parts
}

func systemPartsToText(parts []models.UnifiedMessageContentPart) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type != "text" || part.Text == nil || *part.Text == "" {
			continue
		}
		texts = append(texts, *part.Text)
	}
	return strings.Join(texts, "\n")
}

func buildSystemValue(unified *models.UnifiedRequest) interface{} {
	if unified == nil {
		return nil
	}
	if len(unified.SystemParts) > 0 {
		blocks := buildSystemBlocks(unified.SystemParts)
		if len(blocks) > 0 {
			return blocks
		}
	}
	if unified.System != "" {
		return unified.System
	}
	return nil
}

func buildSystemBlocks(parts []models.UnifiedMessageContentPart) []interface{} {
	blocks := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		if part.Type != "text" || part.Text == nil || *part.Text == "" {
			continue
		}

		block := map[string]interface{}{
			"type": "text",
			"text": *part.Text,
		}
		if part.CacheControl != nil {
			block["cache_control"] = map[string]interface{}{
				"type": part.CacheControl.Type,
			}
		}
		blocks = append(blocks, block)
	}
	return blocks
}

func appendSystemText(req map[string]interface{}, text string) {
	if text == "" {
		return
	}

	if blocks, ok := req["system"].([]interface{}); ok {
		req["system"] = append(blocks, map[string]interface{}{
			"type": "text",
			"text": text,
		})
		return
	}

	if existing, ok := req["system"].(string); ok && existing != "" {
		req["system"] = existing + "\n\n" + text
		return
	}

	req["system"] = text
}

func appendSystemParts(req map[string]interface{}, parts []models.UnifiedMessageContentPart) {
	if len(parts) == 0 {
		return
	}

	if blocks, ok := req["system"].([]interface{}); ok {
		newBlocks := buildSystemBlocks(parts)
		if len(newBlocks) > 0 {
			req["system"] = append(blocks, newBlocks...)
		}
		return
	}

	appendSystemText(req, systemPartsToText(parts))
}
