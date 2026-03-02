package openai

import (
	"encoding/json"
	"testing"
)

func TestFillMissingToolCallIDs_FillsAssistantToolCallIDFromToolMessage(t *testing.T) {
	body := []byte(`{
		"model": "m",
		"messages": [
			{
				"role": "assistant",
				"tool_calls": [
					{
						"type": "function",
						"function": {"name":"a","arguments":"{}"}
					}
				]
			},
			{
				"role": "tool",
				"tool_call_id": "call_1",
				"content": "ok"
			}
		]
	}`)

	updated, changed, err := FillMissingToolCallIDs(body)
	if err != nil {
		t.Fatalf("FillMissingToolCallIDs() error: %v", err)
	}
	if !changed {
		t.Fatalf("FillMissingToolCallIDs() changed=false, want true")
	}

	var req map[string]any
	if err := json.Unmarshal(updated, &req); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}
	msgs := req["messages"].([]any)
	assistant := msgs[0].(map[string]any)
	toolCalls := assistant["tool_calls"].([]any)
	toolCall := toolCalls[0].(map[string]any)
	if got, _ := toolCall["id"].(string); got != "call_1" {
		t.Fatalf("tool_calls[0].id = %q, want %q", got, "call_1")
	}
}

func TestFillMissingToolCallIDs_FillsToolMessageToolCallIDFromAssistant(t *testing.T) {
	body := []byte(`{
		"model": "m",
		"messages": [
			{
				"role": "assistant",
				"tool_calls": [
					{
						"id": "call_2",
						"type": "function",
						"function": {"name":"a","arguments":"{}"}
					}
				]
			},
			{
				"role": "tool",
				"content": "ok"
			}
		]
	}`)

	updated, changed, err := FillMissingToolCallIDs(body)
	if err != nil {
		t.Fatalf("FillMissingToolCallIDs() error: %v", err)
	}
	if !changed {
		t.Fatalf("FillMissingToolCallIDs() changed=false, want true")
	}

	var req map[string]any
	if err := json.Unmarshal(updated, &req); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}
	msgs := req["messages"].([]any)
	toolMsg := msgs[1].(map[string]any)
	if got, _ := toolMsg["tool_call_id"].(string); got != "call_2" {
		t.Fatalf("tool message tool_call_id = %q, want %q", got, "call_2")
	}
}
