package unified

import "testing"

func TestUnifiedMessageReasoningHelpers(t *testing.T) {
	msg := UnifiedMessage{}
	if msg.GetReasoningContent() != "" {
		t.Fatalf("expected empty reasoning content")
	}

	msg.Reasoning = ptrMessageString("fallback")
	if got := msg.GetReasoningContent(); got != "fallback" {
		t.Fatalf("GetReasoningContent() = %q, want fallback", got)
	}

	msg.ReasoningContent = ptrMessageString("primary")
	if got := msg.GetReasoningContent(); got != "primary" {
		t.Fatalf("GetReasoningContent() = %q, want primary", got)
	}

	msg.SetReasoningContent("set")
	if msg.ReasoningContent == nil || *msg.ReasoningContent != "set" {
		t.Fatalf("SetReasoningContent() did not set value")
	}
}

func TestUnifiedMessageContentHelpers(t *testing.T) {
	msg := UnifiedMessage{}
	if got := msg.GetContentAsString(); got != "" {
		t.Fatalf("GetContentAsString() = %q, want empty", got)
	}
	if parts := msg.GetContentParts(); parts != nil {
		t.Fatalf("GetContentParts() = %#v, want nil", parts)
	}

	msg.SetContentString("hello")
	if got := msg.GetContentAsString(); got != "hello" {
		t.Fatalf("GetContentAsString() = %q, want hello", got)
	}
	parts := msg.GetContentParts()
	if len(parts) != 1 || parts[0].Type != "text" || parts[0].Text == nil || *parts[0].Text != "hello" {
		t.Fatalf("GetContentParts() unexpected result: %#v", parts)
	}

	textA := "A"
	textB := "B"
	msg.SetContentParts([]UnifiedMessageContentPart{
		{Type: "text", Text: &textA},
		{Type: "image_url", ImageURL: &UnifiedImageURL{URL: "https://example.com/image.png"}},
		{Type: "text", Text: &textB},
	})
	if got := msg.GetContentAsString(); got != "AB" {
		t.Fatalf("GetContentAsString() = %q, want AB", got)
	}
	parts = msg.GetContentParts()
	if len(parts) != 3 {
		t.Fatalf("GetContentParts() length = %d, want 3", len(parts))
	}
}

func ptrMessageString(v string) *string {
	return &v
}
