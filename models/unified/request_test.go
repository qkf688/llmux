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

	req.Modalities = []string{"text", "image"}
	if !req.IsImageGenerationRequest() {
		t.Fatalf("IsImageGenerationRequest() should be true")
	}
}

func ptrString(v string) *string {
	return &v
}
