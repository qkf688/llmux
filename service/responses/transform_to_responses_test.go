package responses

import (
	"encoding/json"
	"testing"

	"github.com/atopos31/llmio/models"
)

func TestTransformFromUnified_FunctionCallsUseCallID_NotID(t *testing.T) {
	unified := &models.UnifiedRequest{
		Model: "gpt-4.1",
		Messages: []models.UnifiedMessage{
			{Role: "user", Content: "hi"},
			{
				Role: "assistant",
				ToolCalls: []models.UnifiedToolCall{
					{
						Type: "function",
						ID:   "callbb293e3eb9c848008916bae1",
						Function: models.UnifiedToolCallFunction{
							Name:      "tool_one",
							Arguments: `{"x":1}`,
						},
					},
					{
						Type: "function",
						ID:   "callcc293e3eb9c848008916bae2",
						Function: models.UnifiedToolCallFunction{
							Name:      "tool_two",
							Arguments: `{"y":2}`,
						},
					},
				},
			},
			{Role: "tool", ToolCallID: "callbb293e3eb9c848008916bae1", Content: `{"ok":true}`},
			{Role: "tool", ToolCallID: "callcc293e3eb9c848008916bae2", Content: `{"ok":true}`},
		},
	}

	body, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified failed: %v", err)
	}

	var req ResponsesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("failed to unmarshal responses request: %v", err)
	}

	if len(req.Input.Items) != 5 {
		t.Fatalf("expected 5 input items, got %d", len(req.Input.Items))
	}

	// user input_text
	if req.Input.Items[0].Type != "input_text" || req.Input.Items[0].Text == nil || *req.Input.Items[0].Text != "hi" {
		t.Fatalf("unexpected first item: %+v", req.Input.Items[0])
	}
	if req.Input.Items[0].ID != "" {
		t.Fatalf("input_text should not include id, got %q", req.Input.Items[0].ID)
	}

	// assistant tool calls -> function_call items
	if req.Input.Items[1].Type != "function_call" || req.Input.Items[1].CallID == nil || *req.Input.Items[1].CallID != "callbb293e3eb9c848008916bae1" {
		t.Fatalf("unexpected function_call[0]: %+v", req.Input.Items[1])
	}
	if req.Input.Items[1].ID != "" {
		t.Fatalf("function_call[0] should not include id, got %q", req.Input.Items[1].ID)
	}
	if req.Input.Items[1].Name == nil || *req.Input.Items[1].Name != "tool_one" {
		t.Fatalf("unexpected function_call[0] name: %+v", req.Input.Items[1])
	}

	if req.Input.Items[2].Type != "function_call" || req.Input.Items[2].CallID == nil || *req.Input.Items[2].CallID != "callcc293e3eb9c848008916bae2" {
		t.Fatalf("unexpected function_call[1]: %+v", req.Input.Items[2])
	}
	if req.Input.Items[2].ID != "" {
		t.Fatalf("function_call[1] should not include id, got %q", req.Input.Items[2].ID)
	}
	if req.Input.Items[2].Name == nil || *req.Input.Items[2].Name != "tool_two" {
		t.Fatalf("unexpected function_call[1] name: %+v", req.Input.Items[2])
	}

	// tool outputs -> function_call_output items
	if req.Input.Items[3].Type != "function_call_output" || req.Input.Items[3].CallID == nil || *req.Input.Items[3].CallID != "callbb293e3eb9c848008916bae1" {
		t.Fatalf("unexpected function_call_output[0]: %+v", req.Input.Items[3])
	}
	if req.Input.Items[3].ID != "" {
		t.Fatalf("function_call_output[0] should not include id, got %q", req.Input.Items[3].ID)
	}

	if req.Input.Items[4].Type != "function_call_output" || req.Input.Items[4].CallID == nil || *req.Input.Items[4].CallID != "callcc293e3eb9c848008916bae2" {
		t.Fatalf("unexpected function_call_output[1]: %+v", req.Input.Items[4])
	}
	if req.Input.Items[4].ID != "" {
		t.Fatalf("function_call_output[1] should not include id, got %q", req.Input.Items[4].ID)
	}
}

