package unified

import "testing"

func TestUnifiedRequestValidate(t *testing.T) {
	text := "hello"
	cases := []struct {
		name    string
		req     UnifiedRequest
		wantErr string
	}{
		{
			name: "missing model",
			req: UnifiedRequest{
				Messages: []UnifiedMessage{{Role: "user", Content: "hi"}},
			},
			wantErr: "model is required",
		},
		{
			name: "messages and embedding both set",
			req: UnifiedRequest{
				Model:          "m",
				Messages:       []UnifiedMessage{{Role: "user", Content: "hi"}},
				EmbeddingInput: &UnifiedEmbeddingInput{Single: &text},
			},
			wantErr: "cannot specify both messages and input",
		},
		{
			name: "missing messages input and system",
			req: UnifiedRequest{
				Model: "m",
			},
			wantErr: "either messages, input, or system prompt is required",
		},
		{
			name: "empty embedding input",
			req: UnifiedRequest{
				Model:          "m",
				EmbeddingInput: &UnifiedEmbeddingInput{},
			},
			wantErr: "embedding input cannot be empty",
		},
		{
			name: "valid chat request",
			req: UnifiedRequest{
				Model:    "m",
				Messages: []UnifiedMessage{{Role: "user", Content: "hi"}},
			},
		},
		{
			name: "valid system only request",
			req: UnifiedRequest{
				Model:  "m",
				System: "you are helpful",
			},
		},
		{
			name: "valid system parts only request",
			req: UnifiedRequest{
				Model: "m",
				SystemParts: []UnifiedMessageContentPart{
					{Type: "text", Text: ptrString("you are helpful")},
				},
			},
		},
		{
			name: "valid embedding request",
			req: UnifiedRequest{
				Model:          "m",
				EmbeddingInput: &UnifiedEmbeddingInput{Single: &text},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if tc.wantErr == "" && err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Validate() expected error %q, got nil", tc.wantErr)
				}
				if err.Error() != tc.wantErr {
					t.Fatalf("Validate() error = %q, want %q", err.Error(), tc.wantErr)
				}
			}
		})
	}
}

func TestUnifiedRequestClearHelpFields(t *testing.T) {
	index := 1
	callName := "tool"
	isErr := true
	req := UnifiedRequest{
		Model:               "m",
		RawRequest:          []byte(`{}`),
		ExtraBody:           []byte(`{}`),
		Include:             []string{"x"},
		TransformerMetadata: map[string]string{"a": "b"},
		Query:               map[string][]string{"k": {"v"}},
		SystemParts: []UnifiedMessageContentPart{
			{Type: "text", Text: ptrString("sys")},
		},
		Messages: []UnifiedMessage{
			{
				Role:               "assistant",
				Reasoning:          ptrString("r"),
				ReasoningContent:   ptrString("rc"),
				ReasoningSignature: ptrString("rs"),
				MessageIndex:       &index,
				ToolCallName:       &callName,
				ToolCallIsError:    &isErr,
			},
		},
	}

	req.ClearHelpFields()

	if req.RawRequest != nil || req.ExtraBody != nil || req.Include != nil || req.TransformerMetadata != nil || req.Query != nil {
		t.Fatalf("request helper fields should be cleared")
	}
	msg := req.Messages[0]
	if msg.Reasoning != nil || msg.ReasoningContent != nil || msg.ReasoningSignature != nil || msg.MessageIndex != nil || msg.ToolCallName != nil || msg.ToolCallIsError != nil {
		t.Fatalf("message helper fields should be cleared")
	}
}

func TestUnifiedRequestSanitizedForProvider(t *testing.T) {
	index := 1
	callName := "tool"
	isErr := true
	req := UnifiedRequest{
		Model:               "m",
		RawRequest:          []byte(`{}`),
		ExtraBody:           []byte(`{}`),
		Include:             []string{"x"},
		TransformerMetadata: map[string]string{"a": "b"},
		Query:               map[string][]string{"k": {"v"}},
		SystemParts: []UnifiedMessageContentPart{
			{Type: "text", Text: ptrString("sys")},
		},
		Messages: []UnifiedMessage{
			{
				Role:               "assistant",
				Reasoning:          ptrString("r"),
				ReasoningContent:   ptrString("rc"),
				ReasoningSignature: ptrString("rs"),
				MessageIndex:       &index,
				ToolCallName:       &callName,
				ToolCallIsError:    &isErr,
			},
		},
	}

	sanitized := req.SanitizedForProvider()
	if sanitized == nil {
		t.Fatalf("SanitizedForProvider() returned nil")
	}

	// Original request should remain intact.
	if req.RawRequest == nil || req.ExtraBody == nil || req.Include == nil || req.TransformerMetadata == nil || req.Query == nil {
		t.Fatalf("original request should not be modified")
	}
	origMsg := req.Messages[0]
	if origMsg.Reasoning == nil || origMsg.ReasoningContent == nil || origMsg.ReasoningSignature == nil || origMsg.MessageIndex == nil || origMsg.ToolCallName == nil || origMsg.ToolCallIsError == nil {
		t.Fatalf("original message should not be modified")
	}

	// Sanitized copy should have helper fields cleared.
	if sanitized.RawRequest != nil || sanitized.ExtraBody != nil || sanitized.Include != nil || sanitized.TransformerMetadata != nil || sanitized.Query != nil {
		t.Fatalf("sanitized request helper fields should be cleared")
	}
	if len(sanitized.SystemParts) != 1 || sanitized.SystemParts[0].Text == nil || *sanitized.SystemParts[0].Text != "sys" {
		t.Fatalf("sanitized system parts should be preserved")
	}
	msg := sanitized.Messages[0]
	if msg.Reasoning != nil || msg.ReasoningContent != nil || msg.ReasoningSignature != nil || msg.MessageIndex != nil || msg.ToolCallName != nil || msg.ToolCallIsError != nil {
		t.Fatalf("sanitized message helper fields should be cleared")
	}

	// Ensure messages are deep-copied (no shared backing array).
	sanitized.Messages[0].Role = "user"
	if req.Messages[0].Role != "assistant" {
		t.Fatalf("original request messages should not share backing array with sanitized copy")
	}
	sanitized.SystemParts[0].Type = "changed"
	if req.SystemParts[0].Type != "text" {
		t.Fatalf("original request system parts should not share backing array with sanitized copy")
	}
}

func TestUnifiedRequestPredicates(t *testing.T) {
	req := UnifiedRequest{Model: "m"}
	if req.IsEmbeddingRequest() {
		t.Fatalf("IsEmbeddingRequest() should be false")
	}
	if req.IsChatRequest() {
		t.Fatalf("IsChatRequest() should be false")
	}
	if req.IsImageGenerationRequest() {
		t.Fatalf("IsImageGenerationRequest() should be false")
	}

	req.EmbeddingInput = &UnifiedEmbeddingInput{Single: ptrString("x")}
	if !req.IsEmbeddingRequest() {
		t.Fatalf("IsEmbeddingRequest() should be true")
	}

	req.EmbeddingInput = nil
	req.Messages = []UnifiedMessage{{Role: "user", Content: "hi"}}
	if !req.IsChatRequest() {
		t.Fatalf("IsChatRequest() should be true")
	}

	req.Messages = nil
	req.System = "system prompt"
	if !req.IsChatRequest() {
		t.Fatalf("IsChatRequest() should be true for system message")
	}

	req.System = ""
	req.SystemParts = []UnifiedMessageContentPart{{Type: "text", Text: ptrString("x")}}
	if !req.IsChatRequest() {
		t.Fatalf("IsChatRequest() should be true for structured system")
	}

	req.Modalities = []string{"text", "image"}
	if !req.IsImageGenerationRequest() {
		t.Fatalf("IsImageGenerationRequest() should be true")
	}
}

func ptrString(v string) *string {
	return &v
}
