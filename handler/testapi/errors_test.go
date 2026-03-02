package testapi

import (
	"strings"
	"testing"
)

func TestParseProviderErrorDetail(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "nested error message",
			body: `{"error":{"message":"bad key"}}`,
			want: "bad key",
		},
		{
			name: "nested error field",
			body: `{"error":{"error":"provider error"}}`,
			want: "provider error",
		},
		{
			name: "fallback raw body",
			body: `plain error`,
			want: "plain error",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := parseProviderErrorDetail([]byte(tc.body))
			if got != tc.want {
				t.Fatalf("parseProviderErrorDetail() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildDetailedError(t *testing.T) {
	message := BuildDetailedError("network", "连接失败", "dial timeout", map[string]string{
		"provider": "openai",
		"model":    "gpt-4.1",
	})

	if !strings.Contains(message, "[网络错误] 连接失败") {
		t.Fatalf("unexpected header in message: %s", message)
	}
	if !strings.Contains(message, "详细信息: dial timeout") {
		t.Fatalf("unexpected detail in message: %s", message)
	}
	if !strings.Contains(message, "provider: openai") || !strings.Contains(message, "model: gpt-4.1") {
		t.Fatalf("unexpected context in message: %s", message)
	}
}

func TestGetErrorTypeFromStatus(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{status: 401, want: "auth"},
		{status: 404, want: "provider"},
		{status: 422, want: "validation"},
		{status: 500, want: "provider"},
		{status: 302, want: "unknown"},
	}

	for _, tc := range tests {
		if got := getErrorTypeFromStatus(tc.status); got != tc.want {
			t.Fatalf("getErrorTypeFromStatus(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}
