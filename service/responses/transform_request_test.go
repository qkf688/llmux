package responses

import (
	"encoding/json"
	"testing"
)

func TestTransformRequest_RoundTrip_StringInputPreserved(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":"hi"}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if unified.TransformOptions.ArrayInputs == nil || *unified.TransformOptions.ArrayInputs {
		t.Fatalf("expected ArrayInputs=false, got: %#v", unified.TransformOptions.ArrayInputs)
	}

	out, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified failed: %v", err)
	}

	var req ResponsesRequest
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatalf("failed to unmarshal responses request: %v", err)
	}
	if req.Input.Text == nil || *req.Input.Text != "hi" {
		t.Fatalf("expected input to be string 'hi', got: %#v", req.Input)
	}
	if req.Input.Items != nil {
		t.Fatalf("expected input items to be nil, got: %#v", req.Input.Items)
	}
}

func TestTransformRequest_RoundTrip_ArrayInputPreserved(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if unified.TransformOptions.ArrayInputs == nil || !*unified.TransformOptions.ArrayInputs {
		t.Fatalf("expected ArrayInputs=true, got: %#v", unified.TransformOptions.ArrayInputs)
	}

	out, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified failed: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	input := m["input"]
	if len(input) == 0 || input[0] != '[' {
		t.Fatalf("expected output input to be array, got: %s", string(input))
	}
}

func TestTransformRequest_ReasoningMaxTokensMappedToBudget(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":"hi","reasoning":{"effort":"low","max_tokens":123}}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if unified.ReasoningBudget == nil || *unified.ReasoningBudget != 123 {
		t.Fatalf("expected unified.ReasoningBudget=123, got: %#v", unified.ReasoningBudget)
	}

	out, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified failed: %v", err)
	}

	var req ResponsesRequest
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatalf("failed to unmarshal responses request: %v", err)
	}
	if req.Reasoning == nil || req.Reasoning.MaxTokens == nil || *req.Reasoning.MaxTokens != 123 {
		t.Fatalf("expected responses.reasoning.max_tokens=123, got: %#v", req.Reasoning)
	}
	if req.Reasoning.Effort == nil || *req.Reasoning.Effort != "low" {
		t.Fatalf("expected responses.reasoning.effort=low, got: %#v", req.Reasoning)
	}
}
