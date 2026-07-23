package openai

import (
	"encoding/json"
	"testing"
)

func TestFillMissingMessageContent_FillsMissingContentOnToolAndAssistant(t *testing.T) {
	body := []byte(`{
		"model": "m",
		"messages": [
			{
				"role": "assistant",
				"tool_calls": [
					{
						"id": "call_1",
						"type": "function",
						"function": {"name":"a","arguments":"{}"}
					}
				]
			},
			{
				"role": "tool",
				"tool_call_id": "call_1"
			},
			{
				"role": "assistant",
				"content": "ok"
			}
		]
	}`)

	updated, changed, err := FillMissingMessageContent(body)
	if err != nil {
		t.Fatalf("FillMissingMessageContent() error: %v", err)
	}
	if !changed {
		t.Fatalf("FillMissingMessageContent() changed=false, want true")
	}

	var req map[string]any
	if err := json.Unmarshal(updated, &req); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}
	msgs := req["messages"].([]any)

	assistant1 := msgs[0].(map[string]any)
	if got, _ := assistant1["content"].(string); got != "" {
		t.Fatalf("messages[0].content = %q, want empty string", got)
	}

	toolMsg := msgs[1].(map[string]any)
	if got, _ := toolMsg["content"].(string); got != "" {
		t.Fatalf("messages[1].content = %q, want empty string", got)
	}

	assistant2 := msgs[2].(map[string]any)
	if got, _ := assistant2["content"].(string); got != "ok" {
		t.Fatalf("messages[2].content = %q, want %q", got, "ok")
	}
}

func TestFillMissingMessageContent_FillsNullContent(t *testing.T) {
	body := []byte(`{
		"model": "m",
		"messages": [
			{"role":"user","content":null}
		]
	}`)

	updated, changed, err := FillMissingMessageContent(body)
	if err != nil {
		t.Fatalf("FillMissingMessageContent() error: %v", err)
	}
	if !changed {
		t.Fatalf("FillMissingMessageContent() changed=false, want true")
	}

	var req map[string]any
	if err := json.Unmarshal(updated, &req); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}
	msgs := req["messages"].([]any)
	userMsg := msgs[0].(map[string]any)
	if got, _ := userMsg["content"].(string); got != "" {
		t.Fatalf("messages[0].content = %q, want empty string", got)
	}
}

func TestFillMissingMessageContent_NoChangeWhenPresent(t *testing.T) {
	body := []byte(`{
		"model": "m",
		"messages": [
			{"role":"user","content":""},
			{"role":"assistant","content":"x"}
		]
	}`)

	updated, changed, err := FillMissingMessageContent(body)
	if err != nil {
		t.Fatalf("FillMissingMessageContent() error: %v", err)
	}
	if changed {
		t.Fatalf("FillMissingMessageContent() changed=true, want false")
	}
	if string(updated) != string(body) {
		t.Fatalf("expected unchanged body")
	}
}

func TestFillMissingMessageContent_ParseErrorIsNonBreaking(t *testing.T) {
	body := []byte(`{`)

	updated, changed, err := FillMissingMessageContent(body)
	if err != nil {
		t.Fatalf("FillMissingMessageContent() error: %v", err)
	}
	if changed {
		t.Fatalf("FillMissingMessageContent() changed=true, want false")
	}
	if string(updated) != string(body) {
		t.Fatalf("expected unchanged body")
	}
}
