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

	req.Input = convertMessagesToInput(unified.Messages)
	if len(unified.Tools) > 0 {
		req.Tools = convertUnifiedTools(unified.Tools)
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
			if content, ok := msg.Content.(string); ok {
				items = append(items, ResponsesItem{
					Type: "input_text",
					Text: &content,
				})
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
			} else if content, ok := msg.Content.(string); ok {
				items = append(items, ResponsesItem{
					Type: "output_text",
					Text: &content,
				})
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
