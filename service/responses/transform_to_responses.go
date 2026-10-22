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
		Instructions: ResponsesInstructions{Text: unified.System},
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
			Format: buildResponsesTextFormat(unified.ResponseFormat),
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
		} else if obj := unified.ToolChoice.ObjectValue; obj != nil && obj.Function != nil && obj.Function.Name != "" {
			// 出站按 OpenAI Responses 官方的**扁平**形态 emit：
			// {"type":"function","name":"f"}（不是 Chat Completions 的嵌套形）。
			// 与入站能解的两形自洽——同一份请求回灌时不会再丢一次 tool_choice。
			req.ToolChoice.ObjectValue = &ResponsesToolChoiceObject{
				Type: obj.Type,
				Name: obj.Function.Name,
			}
		} else {
			// 两边都没拿到名字：整个 tool_choice 置 nil，而不是留一个空壳
			// （空壳会 emit 出 `"tool_choice":{}`，上游据此判 400）。
			req.ToolChoice = nil
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

	// 缓存与安全相关：纯透传，值域校验交上游
	req.ServiceTier = unified.ServiceTier
	req.SafetyIdentifier = unified.SafetyIdentifier
	req.PromptCacheKey = unified.PromptCacheKey

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

// buildResponsesTextFormat 把统一模型的响应格式装配成 Responses 的 text.format。
//
// **出站按官方扁平原 emit**：{"type":"json_schema","name":...,"schema":...,"strict":...}
// ——name / schema / strict 与 type 平级，无嵌套 json_schema 键（openai-go SDK v2 的
// responses.ResponseFormatTextJSONSchemaConfigParam 与 openai-python 的
// ResponseFormatTextConfigParam 都是这个形状）。
//
// 统一模型承载的是内嵌对象 {"name","schema","strict"}（Chat 协议的自然形状），
// 故出站前把内嵌对象拆平。拆不出三个键时（schema 本身不是对象）按原样塞进
// json_schema 子键——那不是官方形状，但比整个 format 丢掉更接近客户端意图。
func buildResponsesTextFormat(format *models.UnifiedResponseFormat) *ResponsesTextFormat {
	out := &ResponsesTextFormat{Type: format.Type}
	if len(format.JSONSchema) == 0 {
		return out
	}

	var embedded map[string]interface{}
	if err := json.Unmarshal(format.JSONSchema, &embedded); err != nil {
		out.JSONSchema = format.JSONSchema
		return out
	}

	if name, ok := embedded["name"].(string); ok {
		out.Name = name
	}
	if schema, ok := embedded["schema"]; ok {
		out.Schema, _ = schema.(map[string]interface{})
	}
	if strict, ok := embedded["strict"].(bool); ok {
		out.Strict = &strict
	}

	// 三个键一个都没拆出来：内嵌对象不是 {name,schema,strict} 形状，原样保留。
	if out.Name == "" && out.Schema == nil && out.Strict == nil {
		out.JSONSchema = format.JSONSchema
	}
	return out
}
