package responses

import (
	"encoding/json"
	"strings"

	"github.com/atopos31/llmio/models"
)

func unifiedMessageContentToResponses(content any) []responsesContentItem {
	switch v := content.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		text := v
		return []responsesContentItem{{
			Type:        "output_text",
			Text:        &text,
			Annotations: []ResponsesAnnotation{},
		}}
	case []models.UnifiedMessageContentPart:
		return unifiedPartsToResponsesContentItems("assistant", v)
	default:
		return nil
	}
}

func unifiedPartsToResponsesContentItems(role string, parts []models.UnifiedMessageContentPart) []responsesContentItem {
	if len(parts) == 0 {
		return nil
	}
	out := make([]responsesContentItem, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Text == nil || strings.TrimSpace(*part.Text) == "" {
				continue
			}
			partType := "input_text"
			if role == "assistant" {
				partType = "output_text"
			}
			txt := *part.Text
			out = append(out, responsesContentItem{
				Type:        partType,
				Text:        &txt,
				Annotations: []ResponsesAnnotation{},
			})
		case "image_url":
			if part.ImageURL == nil || part.ImageURL.URL == "" {
				continue
			}
			url := part.ImageURL.URL
			out = append(out, responsesContentItem{
				Type:     "image_url",
				ImageURL: &url,
				Detail:   part.ImageURL.Detail,
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseResponsesMessageContent(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, false
	}

	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if strings.TrimSpace(str) == "" {
			return nil, false
		}
		return str, true
	}

	var parts []responsesContentItemDecode
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, false
	}
	return responsesContentItemsToUnified(parts)
}

func responsesContentItemsToUnified(parts []responsesContentItemDecode) (any, bool) {
	if len(parts) == 0 {
		return nil, false
	}

	textOnly := true
	textParts := make([]string, 0, len(parts))
	unifiedParts := make([]models.UnifiedMessageContentPart, 0, len(parts))

	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text", "text":
			if part.Text == nil {
				continue
			}
			txt := *part.Text
			textParts = append(textParts, txt)
			unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{
				Type: "text",
				Text: &txt,
			})
		case "input_image", "output_image", "image_url":
			textOnly = false
			url, detail := decodeResponsesImageURL(part)
			if url == "" {
				continue
			}
			unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{
				Type: "image_url",
				ImageURL: &models.UnifiedImageURL{
					URL:    url,
					Detail: detail,
				},
			})
		default:
			if part.Type == "" {
				continue
			}
			textOnly = false
			unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{Type: part.Type})
		}
	}

	if len(unifiedParts) == 0 {
		return nil, false
	}
	if textOnly {
		return strings.Join(textParts, ""), true
	}
	return unifiedParts, true
}

func decodeResponsesImageURL(part responsesContentItemDecode) (string, *string) {
	if part.ImageURL != nil && len(*part.ImageURL) > 0 {
		var urlStr string
		if err := json.Unmarshal(*part.ImageURL, &urlStr); err == nil {
			if urlStr != "" {
				return urlStr, part.Detail
			}
		}

		var obj responsesImageURLDecode
		if err := json.Unmarshal(*part.ImageURL, &obj); err == nil && obj.URL != "" {
			if obj.Detail != nil && *obj.Detail != "" {
				return obj.URL, obj.Detail
			}
			return obj.URL, part.Detail
		}
	}

	if part.URL != nil && *part.URL != "" {
		return *part.URL, part.Detail
	}

	return "", part.Detail
}
