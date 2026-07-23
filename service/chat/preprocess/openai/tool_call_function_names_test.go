package openai

import (
	"strings"
	"testing"
)

func TestValidateToolCallFunctionNames(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		wantErr   bool
		errSubstr string
	}{
		{
			name: "missing function name",
			body: []byte(`{
				"model":"m",
				"messages":[
					{
						"role":"assistant",
						"tool_calls":[
							{"type":"function","function":{"arguments":"{}"}}
						]
					}
				]
			}`),
			wantErr:   true,
			errSubstr: "messages[0].tool_calls[0].function.name",
		},
		{
			name: "missing function object",
			body: []byte(`{
				"model":"m",
				"messages":[
					{
						"role":"assistant",
						"tool_calls":[
							{"type":"function"}
						]
					}
				]
			}`),
			wantErr:   true,
			errSubstr: "messages[0].tool_calls[0].function is required but missing",
		},
		{
			name: "non-function tool call is ignored",
			body: []byte(`{
				"model":"m",
				"messages":[
					{
						"role":"assistant",
						"tool_calls":[
							{"type":"other","function":{}}
						]
					}
				]
			}`),
			wantErr: false,
		},
		{
			name: "valid tool call passes",
			body: []byte(`{
				"model":"m",
				"messages":[
					{
						"role":"assistant",
						"tool_calls":[
							{"type":"function","function":{"name":"container_shell","arguments":"{}"}}
						]
					}
				]
			}`),
			wantErr: false,
		},
		{
			name:    "invalid json is non-blocking",
			body:    []byte(`{"model":"m","messages":[`),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateToolCallFunctionNames(tc.body)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
