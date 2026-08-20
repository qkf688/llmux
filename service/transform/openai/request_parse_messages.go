package openai

import (
	"encoding/json"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/shared"
)

func parseOpenAIChatMessages(messages shared.RawArray) ([]models.UnifiedMessage, string) {
	decoded := make([]openAIChatMessage, 0, len(messages))
	for _, raw := range messages {
		var msg openAIChatMessage
		if !decodeOpenAIChatObject(raw, &msg) {
			continue
		}
		decoded = append(decoded, msg)
	}

	nonSystemCount := 0
	for _, msg := range decoded {
		if msg.Role.Value != "system" {
			nonSystemCount++
		}
	}
	extractSystem := nonSystemCount > 0

	var unifiedMessages []models.UnifiedMessage
	system := ""
	for _, msg := range decoded {
		role := msg.Role.Value
		if role == "system" && extractSystem {
			if content, ok := rawOpenAIString(msg.Content); ok && content != "" {
				if system != "" {
					system += "\n\n" + content
				} else {
					system = content
				}
			}
			continue
		}

		unified := models.UnifiedMessage{
			Role:      role,
			Content:   parseOpenAIChatMessageContent(msg.Content),
			ToolCalls: parseOpenAIChatToolCalls(msg.ToolCalls),
		}
		if role == "tool" && msg.ToolCallID.Set {
			unified.ToolCallID = msg.ToolCallID.Value
		}
		unifiedMessages = append(unifiedMessages, unified)
	}

	return unifiedMessages, system
}

func parseOpenAIChatMessageContent(raw json.RawMessage) interface{} {
	content := shared.RawJSONValue(raw)
	if content == nil {
		return nil
	}

	var rawParts []json.RawMessage
	if err := json.Unmarshal(raw, &rawParts); err != nil {
		return content
	}

	parts := make([]models.UnifiedMessageContentPart, 0, len(rawParts))
	for _, rawPart := range rawParts {
		var part openAIChatContentPart
		if !decodeOpenAIChatObject(rawPart, &part) {
			continue
		}

		unifiedPart := models.UnifiedMessageContentPart{Type: part.Type.Value}
		switch unifiedPart.Type {
		case "text":
			if part.Text.Set {
				text := part.Text.Value
				unifiedPart.Text = &text
			}
		case "image_url":
			var image openAIChatImageURL
			if decodeOpenAIChatObject(part.ImageURL, &image) {
				var detail *string
				if image.Detail.Set {
					detail = &image.Detail.Value
				}
				unifiedPart.ImageURL = &models.UnifiedImageURL{
					URL:    image.URL.Value,
					Detail: detail,
				}
			}
		case "input_audio":
			var audio openAIChatInputAudio
			if decodeOpenAIChatObject(part.InputAudio, &audio) {
				unifiedPart.InputAudio = &models.UnifiedInputAudio{
					Data:   audio.Data.Value,
					Format: audio.Format.Value,
				}
			}
		}

		parts = append(parts, unifiedPart)
	}
	if len(parts) == 0 {
		return content
	}
	return parts
}
