package transform

import "testing"
import "github.com/qkf688/llmux/models/unified"

func floatPtr(f float64) *float64 {
	return &f
}

func int64Ptr(i int64) *int64 {
	return &i
}

func ptr(s string) *string {
	return &s
}

func baseOpenAIRequest(t *testing.T) *unified.UnifiedRequest {
	t.Helper()
	text := "Hello"
	return &unified.UnifiedRequest{
		Messages: []unified.UnifiedMessage{
			{Role: "user", Content: []unified.UnifiedMessageContentPart{{Type: "text", Text: &text}}},
		},
	}
}

func baseUnifiedRequest(t *testing.T) *unified.UnifiedRequest {
	t.Helper()
	text := "Hello"
	return &unified.UnifiedRequest{
		Messages: []unified.UnifiedMessage{
			{Role: "user", Content: []unified.UnifiedMessageContentPart{{Type: "text", Text: &text}}},
		},
	}
}
