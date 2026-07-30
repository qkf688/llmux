package transform

import "testing"
import "github.com/atopos31/llmio/models/unified"

func floatPtr(f float64) *float64 {
	return &f
}

func int64Ptr(i int64) *int64 {
	return &i
}

func ptr(s string) *string {
	return &s
}

// 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
