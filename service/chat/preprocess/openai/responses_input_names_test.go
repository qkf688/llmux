package openai

import (
	"encoding/json"
	"testing"
)

func TestStripEmptyResponsesInputNames_RemovesEmptyNameForMessageItems(t *testing.T) {
	req := map[string]any{
		"model": "gpt-4.1",
		"input": []any{
			map[string]any{"role": "user", "content": "hi", "name": ""},
			map[string]any{"role": "assistant", "content": "ok"},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	updated, changed, err := StripEmptyResponsesInputNames(body)
	if err != nil {
		t.Fatalf("StripEmptyResponsesInputNames returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}

	var parsed map[string]any
	if err := json.Unmarshal(updated, &parsed); err != nil {
		t.Fatalf("unmarshal updated failed: %v", err)
	}

	items, ok := parsed["input"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("unexpected input: %#v", parsed["input"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if _, exists := first["name"]; exists {
		t.Fatalf("expected name field to be removed, got: %#v", first["name"])
	}
}

func TestStripEmptyResponsesInputNames_ErrorsOnEmptyNameForFunctionCall(t *testing.T) {
	req := map[string]any{
		"model": "gpt-4.1",
		"input": []any{
			map[string]any{"type": "function_call", "name": "", "arguments": "{}"},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	_, _, err = StripEmptyResponsesInputNames(body)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestStripEmptyResponsesInputNames_NoChangeWhenNameMissingOrNonEmpty(t *testing.T) {
	req := map[string]any{
		"model": "gpt-4.1",
		"input": []any{
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{"role": "assistant", "content": "ok", "name": "alice"},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	updated, changed, err := StripEmptyResponsesInputNames(body)
	if err != nil {
		t.Fatalf("StripEmptyResponsesInputNames returned error: %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false")
	}
	if string(updated) != string(body) {
		t.Fatalf("expected body unchanged")
	}
}
