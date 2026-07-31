package responses

import (
	"encoding/json"
	"errors"

	"github.com/qkf688/llmux/models"
)

// TransformFromUnified 将统一格式转换为 Responses API 请求格式。
func TransformFromUnified(unified *models.UnifiedRequest) ([]byte, error) {
	if unified == nil {
		return nil, errors.New("unified request cannot be nil")
	}

	unified = unified.SanitizedForProvider()

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

	req.Input = convertMessagesToInput(unified.Messages, unified.TransformOptions.ArrayInputs)
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

	if unified.ReasoningEffort != nil || unified.ReasoningBudget != nil {
		req.Reasoning = &ResponsesReasoning{
			Effort:    unified.ReasoningEffort,
			MaxTokens: unified.ReasoningBudget,
		}
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

func convertMessagesToInput(messages []models.UnifiedMessage, arrayInputs *bool) ResponsesInput {
	var items []ResponsesItem

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			switch content := msg.Content.(type) {
			case string:
				text := content
				items = append(items, ResponsesItem{
					Type:    "message",
					Role:    "user",
					Content: []map[string]interface{}{{"type": "input_text", "text": text}},
				})
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
					callID := tc.ID
					var callIDPtr *string
					if callID != "" {
						callIDPtr = &callID
					}
					name := tc.Function.Name
					arguments := tc.Function.Arguments
					items = append(items, ResponsesItem{
						Type:      "function_call",
						CallID:    callIDPtr,
						Name:      &name,
						Arguments: &arguments,
					})
				}
			} else {
				switch content := msg.Content.(type) {
				case string:
					text := content
					items = append(items, ResponsesItem{
						Type:    "message",
						Role:    "assistant",
						Content: []map[string]interface{}{{"type": "output_text", "text": text}},
					})
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
			// function_call_output.output 支持 string（纯文本，老上游兼容）
			// 或 input_text/input_image 数组（多模态，OpenAI Responses 协议规范）。
			callID := msg.ToolCallID
			var callIDPtr *string
			if callID != "" {
				callIDPtr = &callID
			}
			item := ResponsesItem{
				Type:   "function_call_output",
				CallID: callIDPtr,
			}
			switch content := msg.Content.(type) {
			case string:
				item.Output = content
			case []models.UnifiedMessageContentPart:
				if arr := unifiedPartsToToolOutput(content); arr != nil {
					item.Output = arr
				} else {
					// 块数组但无有效块，降级为占位符避免空 output
					placeholder := msg.GetContentAsStringWithPlaceholders()
					item.Output = placeholder
				}
			default:
				// 未知 content 类型降级为文本 + 占位符
				item.Output = msg.GetContentAsStringWithPlaceholders()
			}
			items = append(items, item)
		}
	}

	if arrayInputs != nil && !*arrayInputs {
		if text := responsesTextInput(items); text != nil {
			return ResponsesInput{Text: text}
		}
	}

	return ResponsesInput{Items: items}
}

func responsesTextInput(items []ResponsesItem) *string {
	if len(items) != 1 {
		return nil
	}
	first := items[0]
	if first.Type != "message" || first.Role != "user" {
		return nil
	}

	content, ok := first.Content.([]map[string]interface{})
	if !ok || len(content) != 1 {
		return nil
	}
	if content[0]["type"] != "input_text" {
		return nil
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		return nil
	}
	if text == "" {
		return nil
	}
	return &text
}

func convertUnifiedTools(tools []models.UnifiedTool) []ResponsesTool {
	var responses []ResponsesTool
	for _, tool := range tools {
		if tool.Type == "function" {
			params, _ := tool.Function.Parameters.(map[string]interface{})
			responses = append(responses, ResponsesTool{
				Type:        "function",
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  params,
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
