package anthropic

import (
	"encoding/json"
	"github.com/qkf688/llmux/models"
	"testing"
)

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

func TestResponse_RedactedThinkingBlockSurvivesRoundTrip(t *testing.T) {
	const opaqueData = "EmwKAhgBEgyu3v+0/opaque=="
	raw := []byte(`{
		"id":"msg_redacted",
		"model":"claude-3-5-sonnet",
		"stop_reason":"tool_use",
		"content":[
			{"type":"redacted_thinking","data":"` + opaqueData + `"},
			{"type":"text","text":"我查一下。"},
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

	encoded, err := json.Marshal(unified.Choices[0].Message)
	if err != nil {
		t.Fatalf("marshal unified message failed: %v", err)
	}
	var messageFields map[string]interface{}
	if err := json.Unmarshal(encoded, &messageFields); err != nil {
		t.Fatalf("unmarshal unified message failed: %v", err)
	}
	if messageFields["redacted_thinking_data"] != opaqueData {
		t.Fatalf("redacted thinking data not preserved: %#v", messageFields)
	}

	body, err := FormatResponse(unified)
	if err != nil {
		t.Fatalf("FormatResponse returned error: %v", err)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}
	content, ok := resp["content"].([]interface{})
	if !ok || len(content) != 3 {
		t.Fatalf("expected redacted/text/tool blocks, got %#v", resp["content"])
	}
	wantTypes := []string{"redacted_thinking", "text", "tool_use"}
	for i, want := range wantTypes {
		block, ok := content[i].(map[string]interface{})
		if !ok || block["type"] != want {
			t.Fatalf("content block types should be %v, got %#v", wantTypes, content)
		}
	}
	redacted := content[0].(map[string]interface{})
	if redacted["data"] != opaqueData {
		t.Fatalf("redacted thinking data changed: %#v", redacted["data"])
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
