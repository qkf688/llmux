package testapi

import (
	"net/http"
	"testing"
)

func TestBuildTestHeaders(t *testing.T) {
	source := http.Header{
		"Authorization": []string{"Bearer source-token"},
		"X-Trace-ID":    []string{"trace-1"},
	}
	trueVal := true
	falseVal := false

	tests := []struct {
		name          string
		withHeader    *bool
		customHeaders map[string]string
		wantAuth      string
		wantExtra     string
	}{
		{
			name:          "clone source and override",
			withHeader:    &trueVal,
			customHeaders: map[string]string{"Authorization": "Bearer custom-token", "X-Custom": "v"},
			wantAuth:      "Bearer custom-token",
			wantExtra:     "v",
		},
		{
			name:          "ignore source when disabled",
			withHeader:    &falseVal,
			customHeaders: map[string]string{"X-Custom": "v2"},
			wantAuth:      "",
			wantExtra:     "v2",
		},
		{
			name:          "nil withHeader means no source clone",
			withHeader:    nil,
			customHeaders: map[string]string{"X-Custom": "v3"},
			wantAuth:      "",
			wantExtra:     "v3",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := BuildTestHeaders(source, tc.withHeader, tc.customHeaders)
			if got.Get("Authorization") != tc.wantAuth {
				t.Fatalf("Authorization = %q, want %q", got.Get("Authorization"), tc.wantAuth)
			}
			if got.Get("X-Custom") != tc.wantExtra {
				t.Fatalf("X-Custom = %q, want %q", got.Get("X-Custom"), tc.wantExtra)
			}
		})
	}
}
