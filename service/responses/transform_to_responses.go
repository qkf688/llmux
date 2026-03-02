package responses

import (
	"encoding/json"

	"github.com/atopos31/llmio/models"
)

// TransformFromUnified 将统一格式转换为 Responses API 请求格式。
func TransformFromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	req := ResponsesRequest{
		Model:        unified.Model,
		Instructions: unified.System,
	}

	if unified.Stream {
		stream := true
		req.Stream = &stream
	}
	if unified.MaxTokens > 0 {
		req.MaxOutputTokens = &unified.MaxTokens
	}
	if unified.Temperature != nil {
		req.Temperature = unified.Temperature
	}
	if unified.TopP != nil {
		req.TopP = unified.TopP
	}

	if unified.ResponseFormat != nil && unified.ResponseFormat.Type != "" {
		req.Text = &ResponsesText{
			Format: &ResponsesTextFormat{
				Type:       unified.ResponseFormat.Type,
				JSONSchema: unified.ResponseFormat.JSONSchema,
			},
		}
	}

	req.Input = convertMessagesToInput(unified.Messages)
	if len(unified.Tools) > 0 {
		req.Tools = convertUnifiedTools(unified.Tools)
	}

	if unified.ToolChoice != nil {
		req.ToolChoice = &ResponsesToolChoice{}
		if unified.ToolChoice.StringValue != nil {
			req.ToolChoice.StringValue = unified.ToolChoice.StringValue
		} else if unified.ToolChoice.ObjectValue != nil && unified.ToolChoice.ObjectValue.Function != nil {
			req.ToolChoice.ObjectValue = &ResponsesToolChoiceObject{
				Type: unified.ToolChoice.ObjectValue.Type,
				Function: &ResponsesToolChoiceFunction{
					Name: unified.ToolChoice.ObjectValue.Function.Name,
				},
			}
		}
	}

	if unified.ReasoningEffort != nil {
		req.Reasoning = &ResponsesReasoning{Effort: unified.ReasoningEffort}
	}

	if len(unified.Metadata) > 0 {
		req.Metadata = make(map[string]interface{}, len(unified.Metadata))
		for k, v := range unified.Metadata {
			if v == "" {
				continue
			}
			req.Metadata[k] = v
		}
	}

	if unified.ReasoningEffort != nil {
		if req.Metadata == nil {
			req.Metadata = make(map[string]interface{})
		}
		req.Metadata["reasoning_effort"] = *unified.ReasoningEffort
	}

	return json.Marshal(req)
}

func convertMessagesToInput(messages []models.UnifiedMessage) ResponsesInput {
	var items []ResponsesItem

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			switch content := msg.Content.(type) {
			case string:
				items = append(items, ResponsesItem{Type: "input_text", Text: &content})
			case []models.UnifiedMessageContentPart:
				if converted := unifiedPartsToResponsesContent("user", content); converted != nil {
					items = append(items, ResponsesItem{
						Type:    "message",
						Role:    "user",
						Content: converted,
					})
				}
			}
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					items = append(items, ResponsesItem{
						Type:      "function_call",
						ID:        tc.ID,
						CallID:    &tc.ID,
						Name:      &tc.Function.Name,
						Arguments: &tc.Function.Arguments,
					})
				}
			} else {
				switch content := msg.Content.(type) {
				case string:
					items = append(items, ResponsesItem{Type: "output_text", Text: &content})
				case []models.UnifiedMessageContentPart:
					if converted := unifiedPartsToResponsesContent("assistant", content); converted != nil {
						items = append(items, ResponsesItem{
							Type:    "message",
							Role:    "assistant",
							Content: converted,
						})
					}
				}
			}
		case "tool":
			if content, ok := msg.Content.(string); ok {
				items = append(items, ResponsesItem{
					Type:   "function_call_output",
					CallID: &msg.ToolCallID,
					Output: &content,
				})
			}
		}
	}

	return ResponsesInput{Items: items}
}

func convertUnifiedTools(tools []models.UnifiedTool) []ResponsesTool {
	var responses []ResponsesTool
	for _, tool := range tools {
		if tool.Type == "function" {
			params, _ := tool.Function.Parameters.(map[string]interface{})
			responses = append(responses, ResponsesTool{
				Type: "function",
				Function: &ResponsesToolFunction{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  params,
				},
			})
		}
	}
	return responses
}

func unifiedPartsToResponsesContent(role string, parts []models.UnifiedMessageContentPart) []map[string]interface{} {
	if len(parts) == 0 {
		return nil
	}

	out := make([]map[string]interface{}, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			if part.Text == nil {
				continue
			}
			partType := "input_text"
			if role == "assistant" {
				partType = "output_text"
			}
			out = append(out, map[string]interface{}{
				"type": partType,
				"text": *part.Text,
			})
		case "image_url":
			if role != "user" || part.ImageURL == nil || part.ImageURL.URL == "" {
				continue
			}
			item := map[string]interface{}{
				"type":      "input_image",
				"image_url": part.ImageURL.URL,
			}
			if part.ImageURL.Detail != nil && *part.ImageURL.Detail != "" {
				item["detail"] = *part.ImageURL.Detail
			}
			out = append(out, item)
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}
