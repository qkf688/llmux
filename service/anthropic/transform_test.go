package anthropic

import (
	"encoding/json"
	"github.com/atopos31/llmio/models"
	"testing"
)

func TestTransformToUnified(t *testing.T) {
	raw := []byte(`{
		"model":"claude-3-5-sonnet",
		"stream":true,
		"system":[
			{"type":"text","text":"rule-1"},
			{"type":"text","content":"rule-2"}
		],
		"messages":[
			{
				"role":"assistant",
				"content":[{"type":"tool_use","id":"tool_1","name":"calc","input":{"x":1}}],
				"cache_control":{"type":"ephemeral"}
			},
			{
				"role":"user",
				"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"done"}]
			}
		],
		"tools":[
			{
				"name":"calc",
				"description":"do math",
				"input_schema":{"type":"object"},
				"cache_control":{"type":"ephemeral"}
			}
		],
		"tool_choice":{"type":"tool","name":"calc"},
		"stop_sequences":["a","b"],
		"metadata":{"trace_id":"abc"},
		"thinking":{"type":"enabled","budget_tokens":30000}
	}`)

	unified, err := TransformToUnified(raw)
	if err != nil {
		t.Fatalf("TransformToUnified returned error: %v", err)
	}

	if unified.Model != "claude-3-5-sonnet" {
		t.Fatalf("unexpected model: %s", unified.Model)
	}
	if !unified.Stream {
		t.Fatal("expected stream=true")
	}
	if unified.System != "rule-1\nrule-2" {
		t.Fatalf("unexpected system: %q", unified.System)
	}
	if len(unified.SystemParts) != 2 {
		t.Fatalf("expected 2 system parts, got %d", len(unified.SystemParts))
	}
	if unified.SystemParts[0].Type != "text" || unified.SystemParts[0].Text == nil || *unified.SystemParts[0].Text != "rule-1" {
		t.Fatalf("unexpected system part[0]: %+v", unified.SystemParts[0])
	}
	if unified.SystemParts[1].Type != "text" || unified.SystemParts[1].Text == nil || *unified.SystemParts[1].Text != "rule-2" {
		t.Fatalf("unexpected system part[1]: %+v", unified.SystemParts[1])
	}
	if len(unified.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(unified.Messages))
	}
	if len(unified.Messages[0].ToolCalls) != 1 {
		t.Fatalf("expected tool call parsed from content, got %d", len(unified.Messages[0].ToolCalls))
	}
	if unified.Messages[1].Role != "tool" {
		t.Fatalf("expected tool_result to be mapped to role=tool, got %q", unified.Messages[1].Role)
	}
	if unified.Messages[1].ToolCallID != "tool_1" {
		t.Fatalf("expected tool_call_id=tool_1, got %q", unified.Messages[1].ToolCallID)
	}
	if content, ok := unified.Messages[1].Content.(string); !ok || content != "done" {
		t.Fatalf("expected tool message content 'done', got %#v", unified.Messages[1].Content)
	}
	if unified.Messages[0].CacheControl == nil || unified.Messages[0].CacheControl.Type != "ephemeral" {
		t.Fatal("expected message cache_control to be parsed")
	}
	if len(unified.Tools) != 1 || unified.Tools[0].CacheControl == nil || unified.Tools[0].CacheControl.Type != "ephemeral" {
		t.Fatal("expected tool cache_control to be parsed")
	}
	if unified.Stop == nil || len(unified.Stop.Multiple) != 2 {
		t.Fatal("expected stop_sequences to be mapped")
	}
	if unified.Metadata["trace_id"] != "abc" {
		t.Fatalf("unexpected metadata: %+v", unified.Metadata)
	}
	if unified.ReasoningEffort == nil || *unified.ReasoningEffort != "medium" {
		t.Fatalf("unexpected reasoning_effort: %v", unified.ReasoningEffort)
	}
	if unified.ReasoningBudget == nil || *unified.ReasoningBudget != 30000 {
		t.Fatalf("unexpected reasoning_budget: %v", unified.ReasoningBudget)
	}
	if unified.ToolChoice == nil || unified.ToolChoice.ObjectValue == nil || unified.ToolChoice.ObjectValue.Function == nil || unified.ToolChoice.ObjectValue.Function.Name != "calc" {
		t.Fatalf("unexpected tool_choice: %+v", unified.ToolChoice)
	}
}

func TestTransformToUnified_MapsAnthropicBase64ImageToOpenAIDataURL(t *testing.T) {
	raw := []byte(`{
		"model":"claude-3-5-sonnet",
		"messages":[
			{
				"role":"user",
				"content":[
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAA"}}
				]
			}
		]
	}`)

	unified, err := TransformToUnified(raw)
	if err != nil {
		t.Fatalf("TransformToUnified returned error: %v", err)
	}
	if len(unified.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(unified.Messages))
	}

	parts, ok := unified.Messages[0].Content.([]models.UnifiedMessageContentPart)
	if !ok || len(parts) != 1 {
		t.Fatalf("expected 1 content part, got %#v", unified.Messages[0].Content)
	}
	if parts[0].Type != "image_url" || parts[0].ImageURL == nil {
		t.Fatalf("expected image_url part, got %#v", parts[0])
	}
	if parts[0].ImageURL.URL != "data:image/png;base64,AAA" {
		t.Fatalf("unexpected image url: %q", parts[0].ImageURL.URL)
	}
}

func TestTransformFromUnified_MapsOpenAIDataURLToAnthropicBase64Image(t *testing.T) {
	unified := &models.UnifiedRequest{
		Model: "claude-3-5-sonnet",
		Messages: []models.UnifiedMessage{
			{
				Role: "user",
				Content: []models.UnifiedMessageContentPart{
					{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAA"}},
				},
			},
		},
	}

	body, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified returned error: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}

	msgs, ok := req["messages"].([]interface{})
	if !ok || len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %#v", req["messages"])
	}
	msg, _ := msgs[0].(map[string]interface{})
	content, ok := msg["content"].([]interface{})
	if !ok || len(content) != 1 {
		t.Fatalf("expected 1 content block, got %#v", msg["content"])
	}
	block, _ := content[0].(map[string]interface{})
	if block["type"] != "image" {
		t.Fatalf("expected type=image, got %#v", block["type"])
	}
	source, _ := block["source"].(map[string]interface{})
	if source["type"] != "base64" {
		t.Fatalf("expected source.type=base64, got %#v", source["type"])
	}
	if source["media_type"] != "image/png" {
		t.Fatalf("expected media_type=image/png, got %#v", source["media_type"])
	}
	if source["data"] != "AAA" {
		t.Fatalf("expected data=AAA, got %#v", source["data"])
	}
}

func TestTransformRoundTripPreservesSystemArray(t *testing.T) {
	raw := []byte(`{
		"model":"claude-3-5-sonnet",
		"system":[
			{"type":"text","text":"rule-1"},
			{"type":"text","content":"rule-2"}
		],
		"messages":[{"role":"user","content":"hi"}]
	}`)

	unified, err := TransformToUnified(raw)
	if err != nil {
		t.Fatalf("TransformToUnified returned error: %v", err)
	}

	body, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified returned error: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}

	system, ok := req["system"].([]interface{})
	if !ok {
		t.Fatalf("expected system to be array, got %T", req["system"])
	}
	if len(system) != 2 {
		t.Fatalf("expected 2 system blocks, got %d", len(system))
	}

	block0 := system[0].(map[string]interface{})
	if block0["type"] != "text" || block0["text"] != "rule-1" {
		t.Fatalf("unexpected system[0]: %+v", block0)
	}
	block1 := system[1].(map[string]interface{})
	if block1["type"] != "text" || block1["text"] != "rule-2" {
		t.Fatalf("unexpected system[1]: %+v", block1)
	}
}

func TestTransformFromUnified(t *testing.T) {
	stop := "done"
	effort := "high"

	unified := &models.UnifiedRequest{
		Model:  "claude-3-5-sonnet",
		Stream: true,
		Messages: []models.UnifiedMessage{
			{Role: "system", Content: "follow rules"},
			{Role: "user", Content: "hello"},
			{
				Role:    "assistant",
				Content: "calling tool",
				ToolCalls: []models.UnifiedToolCall{
					{
						ID:   "tool_1",
						Type: "function",
						Function: models.UnifiedToolCallFunction{
							Name:      "calc",
							Arguments: `{"x":1}`,
						},
					},
				},
			},
			{Role: "tool", ToolCallID: "tool_1", Content: "42"},
		},
		Stop:            &models.UnifiedStop{Single: &stop},
		ReasoningEffort: &effort,
	}

	body, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified returned error: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}

	if req["max_tokens"].(float64) != 8192 {
		t.Fatalf("expected default max_tokens=8192, got %v", req["max_tokens"])
	}
	if req["system"].(string) != "follow rules" {
		t.Fatalf("expected extracted system message, got %v", req["system"])
	}

	messages, ok := req["messages"].([]interface{})
	if !ok || len(messages) != 3 {
		t.Fatalf("expected 3 non-system messages, got %d", len(messages))
	}

	toolResultMsg := messages[2].(map[string]interface{})
	if toolResultMsg["role"] != "user" {
		t.Fatalf("expected tool role converted to user, got %v", toolResultMsg["role"])
	}

	thinking, ok := req["thinking"].(map[string]interface{})
	if !ok {
		t.Fatal("expected thinking field")
	}
	if thinking["budget_tokens"].(float64) != 50000 {
		t.Fatalf("expected high effort => 50000 budget, got %v", thinking["budget_tokens"])
	}
}

func TestParseResponse(t *testing.T) {
	raw := []byte(`{
		"id":"msg_1",
		"model":"claude-3-5-sonnet",
		"stop_reason":"end_turn",
		"content":[
			{"type":"text","text":"hello"},
			{"type":"tool_use","id":"tool_1","name":"calc","input":{"x":1}}
		],
		"usage":{"input_tokens":10,"output_tokens":20}
	}`)

	unified, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("ParseResponse returned error: %v", err)
	}

	if len(unified.Choices) != 1 {
		t.Fatalf("expected one choice, got %d", len(unified.Choices))
	}
	if unified.Choices[0].FinishReason != "stop" {
		t.Fatalf("expected finish_reason=stop, got %s", unified.Choices[0].FinishReason)
	}
	if unified.Choices[0].Message == nil || unified.Choices[0].Message.Content != "hello" {
		t.Fatalf("unexpected message content: %+v", unified.Choices[0].Message)
	}
	if len(unified.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(unified.Choices[0].Message.ToolCalls))
	}
	if unified.Usage == nil || unified.Usage.TotalTokens != 30 {
		t.Fatalf("unexpected usage: %+v", unified.Usage)
	}
}

func TestFormatResponse(t *testing.T) {
	unified := &models.UnifiedResponse{
		ID:    "msg_1",
		Model: "claude-3-5-sonnet",
		Choices: []models.UnifiedChoice{
			{
				Index: 0,
				Message: &models.UnifiedMessage{
					Role:    "assistant",
					Content: "hello",
					ToolCalls: []models.UnifiedToolCall{
						{
							ID:   "tool_1",
							Type: "function",
							Function: models.UnifiedToolCallFunction{
								Name:      "calc",
								Arguments: `{"x":1}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: &models.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
		},
	}

	body, err := FormatResponse(unified)
	if err != nil {
		t.Fatalf("FormatResponse returned error: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}

	if resp["stop_reason"] != "tool_use" {
		t.Fatalf("expected stop_reason=tool_use, got %v", resp["stop_reason"])
	}

	content, ok := resp["content"].([]interface{})
	if !ok || len(content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(content))
	}

	usage, ok := resp["usage"].(map[string]interface{})
	if !ok {
		t.Fatal("expected usage field")
	}
	if usage["input_tokens"].(float64) != 10 || usage["output_tokens"].(float64) != 20 {
		t.Fatalf("unexpected usage values: %+v", usage)
	}
}

func TestParseResponse_MapsThinkingAndMultimodalContent(t *testing.T) {
	raw := []byte(`{
		"id":"msg_1",
		"model":"claude-3-5-sonnet",
		"stop_reason":"end_turn",
		"content":[
			{"type":"thinking","thinking":"plan","signature":"sig"},
			{"type":"text","text":"hello"},
			{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAA"}},
			{"type":"tool_use","id":"tool_1","name":"calc","input":{"x":1}}
		],
		"usage":{"input_tokens":10,"output_tokens":20}
	}`)

	unified, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("ParseResponse returned error: %v", err)
	}

	if len(unified.Choices) != 1 || unified.Choices[0].Message == nil {
		t.Fatalf("expected one choice with message, got %+v", unified.Choices)
	}

	msg := unified.Choices[0].Message
	if msg.ReasoningContent == nil || *msg.ReasoningContent != "plan" {
		t.Fatalf("expected reasoning_content=plan, got %+v", msg.ReasoningContent)
	}
	if msg.ReasoningSignature == nil || *msg.ReasoningSignature != "sig" {
		t.Fatalf("expected reasoning_signature=sig, got %+v", msg.ReasoningSignature)
	}

	parts, ok := msg.Content.([]models.UnifiedMessageContentPart)
	if !ok || len(parts) < 2 {
		t.Fatalf("expected multimodal content parts, got %#v", msg.Content)
	}
	if parts[0].Type != "text" || parts[0].Text == nil || *parts[0].Text != "plan\n\n---\n\n" {
		t.Fatalf("unexpected first part: %#v", parts[0])
	}
	if parts[1].Type != "text" || parts[1].Text == nil || *parts[1].Text != "hello" {
		t.Fatalf("unexpected second part: %#v", parts[1])
	}
	foundImage := false
	for _, p := range parts {
		if p.Type == "image_url" && p.ImageURL != nil && p.ImageURL.URL == "data:image/png;base64,AAA" {
			foundImage = true
		}
	}
	if !foundImage {
		t.Fatalf("expected image_url data URL part, got %#v", parts)
	}

	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "tool_1" {
		t.Fatalf("expected tool_calls with id tool_1, got %#v", msg.ToolCalls)
	}
	if unified.Choices[0].FinishReason != "stop" {
		t.Fatalf("expected finish_reason=stop, got %s", unified.Choices[0].FinishReason)
	}
}

func TestFormatResponse_MapsContentPartsAndThinking(t *testing.T) {
	reasoning := "plan"
	signature := "sig"
	text := "hello"
	unified := &models.UnifiedResponse{
		ID:    "msg_1",
		Model: "claude-3-5-sonnet",
		Choices: []models.UnifiedChoice{
			{
				Index: 0,
				Message: &models.UnifiedMessage{
					Role:             "assistant",
					ReasoningContent: &reasoning,
					ReasoningSignature: func() *string {
						return &signature
					}(),
					Content: []models.UnifiedMessageContentPart{
						{Type: "text", Text: &text},
						{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAA"}},
					},
					ToolCalls: []models.UnifiedToolCall{
						{
							ID:   "tool_1",
							Type: "function",
							Function: models.UnifiedToolCallFunction{
								Name:      "calc",
								Arguments: `{"x":1}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
	}

	body, err := FormatResponse(unified)
	if err != nil {
		t.Fatalf("FormatResponse returned error: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}

	if resp["stop_reason"] != "tool_use" {
		t.Fatalf("expected stop_reason=tool_use, got %v", resp["stop_reason"])
	}

	content, ok := resp["content"].([]interface{})
	if !ok || len(content) < 3 {
		t.Fatalf("expected at least 3 content blocks (thinking/text/image/tool_use), got %#v", resp["content"])
	}

	// thinking block should be first
	first, ok := content[0].(map[string]interface{})
	if !ok || first["type"] != "thinking" || first["thinking"] != "plan" || first["signature"] != "sig" {
		t.Fatalf("unexpected first content block: %#v", content[0])
	}

	foundImage := false
	foundToolUse := false
	for _, item := range content {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		switch itemMap["type"] {
		case "image":
			source, ok := itemMap["source"].(map[string]interface{})
			if ok && source["type"] == "base64" && source["media_type"] == "image/png" && source["data"] == "AAA" {
				foundImage = true
			}
		case "tool_use":
			if itemMap["id"] == "tool_1" && itemMap["name"] == "calc" {
				foundToolUse = true
			}
		}
	}
	if !foundImage {
		t.Fatal("expected image base64 block in content")
	}
	if !foundToolUse {
		t.Fatal("expected tool_use block in content")
	}
}
