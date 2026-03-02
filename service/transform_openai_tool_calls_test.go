package service

import "testing"

func TestParseOpenAIToolCalls_ArgumentsNormalization(t *testing.T) {
	tests := []struct {
		name      string
		arguments interface{}
		expected  string
	}{
		{name: "string passthrough", arguments: `{"location":"Beijing"}`, expected: `{"location":"Beijing"}`},
		{name: "empty string defaults", arguments: "", expected: "{}"},
		{name: "object marshals", arguments: map[string]interface{}{"location": "Beijing"}, expected: `{"location":"Beijing"}`},
		{name: "array marshals", arguments: []interface{}{map[string]interface{}{"x": float64(1)}}, expected: `[{"x":1}]`},
		{name: "nil defaults", arguments: nil, expected: "{}"},
		{name: "non-json defaults", arguments: true, expected: "{}"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			funcMap := map[string]interface{}{
				"name": "get_weather",
			}
			if tc.arguments != nil || tc.name == "nil defaults" {
				funcMap["arguments"] = tc.arguments
			}

			msgMap := map[string]interface{}{
				"tool_calls": []interface{}{
					map[string]interface{}{
						"id":       "call_123",
						"type":     "function",
						"function": funcMap,
					},
				},
			}

			toolCalls := parseOpenAIToolCalls(msgMap)
			if len(toolCalls) != 1 {
				t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
			}
			if toolCalls[0].Function.Arguments != tc.expected {
				t.Fatalf("expected arguments %s, got %s", tc.expected, toolCalls[0].Function.Arguments)
			}
		})
	}
}
