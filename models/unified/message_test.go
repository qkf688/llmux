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

// 待办 28：不支持多模态 tool 结果的上游需要一个纯文本降级，
// 且不能把「只有截图的 tool_result」降级成空串——空 content 会被部分上游判 400。
func TestUnifiedMessageGetContentAsStringWithPlaceholders(t *testing.T) {
	msg := UnifiedMessage{}
	if got := msg.GetContentAsStringWithPlaceholders(); got != "" {
		t.Fatalf("空内容应降级为空串，实际 %q", got)
	}

	msg.SetContentString("plain")
	if got := msg.GetContentAsStringWithPlaceholders(); got != "plain" {
		t.Fatalf("string 内容应原样返回，实际 %q", got)
	}

	text := "截图如下"
	msg.SetContentParts([]UnifiedMessageContentPart{
		{Type: "text", Text: &text},
		{Type: "image_url", ImageURL: &UnifiedImageURL{URL: "data:image/png;base64,AAAA"}},
	})
	if got := msg.GetContentAsStringWithPlaceholders(); got != "截图如下[image]" {
		t.Fatalf("文本与占位符应按顺序混排，实际 %q", got)
	}

	msg.SetContentParts([]UnifiedMessageContentPart{
		{Type: "image_url", ImageURL: &UnifiedImageURL{URL: "data:image/png;base64,AAAA"}},
	})
	if got := msg.GetContentAsStringWithPlaceholders(); got != "[image]" {
		t.Fatalf("纯图片内容不得降级为空串，实际 %q", got)
	}

	msg.SetContentParts([]UnifiedMessageContentPart{
		{Type: "input_audio", InputAudio: &UnifiedInputAudio{Data: "AAAA", Format: "wav"}},
		{Type: "video_url"},
	})
	if got := msg.GetContentAsStringWithPlaceholders(); got != "[audio][video_url]" {
		t.Fatalf("未知块应按类型名生成占位符，实际 %q", got)
	}
}

func ptrMessageString(v string) *string {
	return &v
}
