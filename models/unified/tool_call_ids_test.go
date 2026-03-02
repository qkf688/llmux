package unified

import "testing"

func TestNormalizeToolCallIDs_FillFromToolMessages(t *testing.T) {
	req := UnifiedRequest{
		Model: "m",
		Messages: []UnifiedMessage{
			{
				Role: "assistant",
				ToolCalls: []UnifiedToolCall{
					{ID: "", Type: "function", Function: UnifiedToolCallFunction{Name: "a", Arguments: "{}"}},
					{ID: "", Type: "function", Function: UnifiedToolCallFunction{Name: "b", Arguments: "{}"}},
				},
			},
			{Role: "tool", ToolCallID: "call_1", Content: "ok"},
			{Role: "tool", ToolCallID: "call_2", Content: "ok"},
		},
	}

	req.NormalizeToolCallIDs()

	if got := req.Messages[0].ToolCalls[0].ID; got != "call_1" {
		t.Fatalf("tool_calls[0].id = %q, want %q", got, "call_1")
	}
	if got := req.Messages[0].ToolCalls[1].ID; got != "call_2" {
		t.Fatalf("tool_calls[1].id = %q, want %q", got, "call_2")
	}
}

func TestNormalizeToolCallIDs_FillToolMessagesFromToolCalls(t *testing.T) {
	req := UnifiedRequest{
		Model: "m",
		Messages: []UnifiedMessage{
			{
				Role: "assistant",
				ToolCalls: []UnifiedToolCall{
					{ID: "call_1", Type: "function", Function: UnifiedToolCallFunction{Name: "a", Arguments: "{}"}},
					{ID: "call_2", Type: "function", Function: UnifiedToolCallFunction{Name: "b", Arguments: "{}"}},
				},
			},
			{Role: "tool", ToolCallID: "", Content: "ok"},
			{Role: "tool", ToolCallID: "", Content: "ok"},
		},
	}

	req.NormalizeToolCallIDs()

	if got := req.Messages[1].ToolCallID; got != "call_1" {
		t.Fatalf("tool message[0].tool_call_id = %q, want %q", got, "call_1")
	}
	if got := req.Messages[2].ToolCallID; got != "call_2" {
		t.Fatalf("tool message[1].tool_call_id = %q, want %q", got, "call_2")
	}
}

func TestNormalizeToolCallIDs_GeneratesWhenNoCandidates(t *testing.T) {
	req := UnifiedRequest{
		Model: "m",
		Messages: []UnifiedMessage{
			{
				Role: "assistant",
				ToolCalls: []UnifiedToolCall{
					{ID: "", Type: "function", Function: UnifiedToolCallFunction{Name: "a", Arguments: "{}"}},
					{ID: "", Type: "function", Function: UnifiedToolCallFunction{Name: "b", Arguments: "{}"}},
				},
			},
			{Role: "user", Content: "hi"},
		},
	}

	req.NormalizeToolCallIDs()

	id1 := req.Messages[0].ToolCalls[0].ID
	id2 := req.Messages[0].ToolCalls[1].ID
	if id1 == "" || id2 == "" {
		t.Fatalf("generated tool_call IDs should be non-empty, got %q and %q", id1, id2)
	}
	if id1 == id2 {
		t.Fatalf("generated tool_call IDs should be unique, got %q", id1)
	}
}

func TestUnifiedRequestSanitizedForProvider_DoesNotMutateToolCalls(t *testing.T) {
	req := UnifiedRequest{
		Model: "m",
		Messages: []UnifiedMessage{
			{
				Role: "assistant",
				ToolCalls: []UnifiedToolCall{
					{ID: "", Type: "function", Function: UnifiedToolCallFunction{Name: "a", Arguments: "{}"}},
				},
			},
			{Role: "tool", ToolCallID: "call_1", Content: "ok"},
		},
	}

	sanitized := req.SanitizedForProvider()
	if sanitized == nil {
		t.Fatalf("SanitizedForProvider() returned nil")
	}

	if got := req.Messages[0].ToolCalls[0].ID; got != "" {
		t.Fatalf("original tool_calls[0].id = %q, want empty", got)
	}
	if got := sanitized.Messages[0].ToolCalls[0].ID; got != "call_1" {
		t.Fatalf("sanitized tool_calls[0].id = %q, want %q", got, "call_1")
	}
}
