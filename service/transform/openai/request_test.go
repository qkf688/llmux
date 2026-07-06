package openai

import (
	"context"
	"testing"

	"github.com/atopos31/llmio/models"
)

func TestToUnified_TypedRequestCompatibility(t *testing.T) {
	input := []byte(`{
		"model": "gpt-4.1",
		"messages": [
			"not-an-object",
			{"role": "system", "content": "system prompt"},
			{
				"role": "user",
				"content": [
					"not-a-content-part",
					{"type": "text", "text": "hello"},
					{"type": "image_url", "image_url": null}
				]
			},
			{
				"role": "assistant",
				"tool_calls": [
					"not-a-tool-call",
					{
						"id": "call_1",
						"type": "function",
						"function": {
							"name": "lookup",
							"arguments": {"city": "Paris"}
						}
					}
				]
			},
			{"role": "tool", "tool_call_id": "call_1", "content": {"ok": true}}
		],
		"tools": [
			"not-a-tool",
			{
				"type": "function",
				"function": {
					"name": "lookup",
					"description": "Lookup data",
					"parameters": {"type": "object"}
				}
			}
		],
		"max_tokens": "invalid",
		"temperature": 0.4,
		"modalities": [1, "text"],
		"response_format": null,
		"tool_choice": null,
		"stream_options": null,
		"stop": null,
		"audio": null
	}`)

	unified, err := ToUnified(context.Background(), input)
	if err != nil {
		t.Fatalf("ToUnified failed: %v", err)
	}

	if unified.Model != "gpt-4.1" {
		t.Fatalf("model mismatch: %q", unified.Model)
	}
	if unified.MaxTokens != 0 {
		t.Fatalf("invalid max_tokens should be ignored, got %d", unified.MaxTokens)
	}
	if unified.Temperature == nil || *unified.Temperature != 0.4 {
		t.Fatalf("temperature mismatch: %v", unified.Temperature)
	}
	if unified.System != "system prompt" {
		t.Fatalf("system extraction mismatch: %q", unified.System)
	}
	if len(unified.Messages) != 3 {
		t.Fatalf("expected 3 non-system messages, got %d", len(unified.Messages))
	}

	parts, ok := unified.Messages[0].Content.([]models.UnifiedMessageContentPart)
	if !ok {
		t.Fatalf("expected multimodal parts, got %T", unified.Messages[0].Content)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 valid content part objects, got %d", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text == nil || *parts[0].Text != "hello" {
		t.Fatalf("text part mismatch: %+v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL != nil {
		t.Fatalf("image_url null should preserve only the part type: %+v", parts[1])
	}

	toolCalls := unified.Messages[1].ToolCalls
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 valid tool call, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call_1" || toolCalls[0].Function.Name != "lookup" {
		t.Fatalf("tool call mismatch: %+v", toolCalls[0])
	}
	if toolCalls[0].Function.Arguments != `{"city":"Paris"}` {
		t.Fatalf("tool call arguments mismatch: %q", toolCalls[0].Function.Arguments)
	}
	if unified.Messages[2].ToolCallID != "call_1" {
		t.Fatalf("tool_call_id mismatch: %q", unified.Messages[2].ToolCallID)
	}

	if len(unified.Tools) != 1 {
		t.Fatalf("expected 1 valid tool, got %d", len(unified.Tools))
	}
	if unified.Tools[0].Function.Name != "lookup" {
		t.Fatalf("tool function mismatch: %+v", unified.Tools[0].Function)
	}
	if len(unified.Modalities) != 1 || unified.Modalities[0] != "text" {
		t.Fatalf("modalities mismatch: %v", unified.Modalities)
	}
	if unified.ResponseFormat != nil || unified.ToolChoice != nil || unified.StreamOptions != nil || unified.Stop != nil || unified.Audio != nil {
		t.Fatalf("null optional fields should remain unset: response_format=%v tool_choice=%v stream_options=%v stop=%v audio=%v",
			unified.ResponseFormat, unified.ToolChoice, unified.StreamOptions, unified.Stop, unified.Audio)
	}
}

func TestToUnified_SystemOnlyMessagePreserved(t *testing.T) {
	input := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "system", "content": "only system"}]
	}`)

	unified, err := ToUnified(context.Background(), input)
	if err != nil {
		t.Fatalf("ToUnified failed: %v", err)
	}

	if unified.System != "" {
		t.Fatalf("system-only request should not extract system text, got %q", unified.System)
	}
	if len(unified.Messages) != 1 {
		t.Fatalf("expected system-only message to remain in messages, got %d messages", len(unified.Messages))
	}
	if unified.Messages[0].Role != "system" || unified.Messages[0].Content != "only system" {
		t.Fatalf("system-only message mismatch: %+v", unified.Messages[0])
	}
}
