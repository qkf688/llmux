package anthropic

import (
	"github.com/qkf688/llmux/models"
	"strings"

	"github.com/qkf688/llmux/common/maputil"
)

// parseSystem parses Anthropic's "system" field, which can be either a string or an array of text blocks.
// It returns:
// - the extracted plain text (used by internal logic and other providers),
// - the structured blocks (used to preserve Anthropic's system array format when round-tripping).
func parseSystem(value interface{}) (string, []models.UnifiedMessageContentPart) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return "", nil
		}
		return v, nil
	case []interface{}:
		parts := parseSystemParts(v)
		return systemPartsToText(parts), parts
	default:
		return "", nil
	}
}

func parseSystemParts(items []interface{}) []models.UnifiedMessageContentPart {
	parts := make([]models.UnifiedMessageContentPart, 0, len(items))
	for _, item := range items {
		itemMap, ok := asMap(item)
		if !ok {
			continue
		}
		if maputil.String(itemMap, "type") != "text" {
			continue
		}

		text := ""
		if v, ok := itemMap["text"].(string); ok && v != "" {
			text = v
		} else if v, ok := itemMap["content"].(string); ok && v != "" {
			text = v
		}
		if text == "" {
			continue
		}

		partText := text
		parts = append(parts, models.UnifiedMessageContentPart{
			Type:         "text",
			Text:         &partText,
			CacheControl: parseCacheControl(itemMap["cache_control"]),
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
