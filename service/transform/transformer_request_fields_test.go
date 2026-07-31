package transform

import (
	"context"
	"encoding/json"
	"github.com/qkf688/llmux/models"
	"testing"
)

// 阶段 2: 测试响应格式和工具增强
func TestTransformOpenAIToUnified_ResponseFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, *models.UnifiedRequest)
	}{
		{
			name: "response_format_text",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"response_format": {"type": "text"}
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if u.ResponseFormat == nil {
					t.Fatal("ResponseFormat is nil")
				}
				if u.ResponseFormat.Type != "text" {
					t.Errorf("expected type 'text', got %v", u.ResponseFormat.Type)
				}
			},
		},
		{
			name: "response_format_json_object",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"response_format": {"type": "json_object"}
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if u.ResponseFormat == nil {
					t.Fatal("ResponseFormat is nil")
				}
				if u.ResponseFormat.Type != "json_object" {
					t.Errorf("expected type 'json_object', got %v", u.ResponseFormat.Type)
				}
			},
		},
		{
			name: "response_format_json_schema",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"response_format": {
					"type": "json_schema",
					"json_schema": {
						"name": "test_schema",
						"schema": {"type": "object"}
					}
				}
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if u.ResponseFormat == nil {
					t.Fatal("ResponseFormat is nil")
				}
				if u.ResponseFormat.Type != "json_schema" {
					t.Errorf("expected type 'json_schema', got %v", u.ResponseFormat.Type)
				}
				if len(u.ResponseFormat.JSONSchema) == 0 {
					t.Error("JSONSchema is empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := TransformOpenAIToUnified(context.Background(), []byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, unified)
		})
	}
}

func TestTransformOpenAIToUnified_ToolChoice(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, *models.UnifiedRequest)
	}{
		{
			name: "tool_choice_auto",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"tool_choice": "auto"
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if u.ToolChoice == nil {
					t.Fatal("ToolChoice is nil")
				}
				if u.ToolChoice.StringValue == nil || *u.ToolChoice.StringValue != "auto" {
					t.Error("expected tool_choice 'auto'")
				}
			},
		},
		{
			name: "tool_choice_none",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"tool_choice": "none"
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if u.ToolChoice == nil {
					t.Fatal("ToolChoice is nil")
				}
				if u.ToolChoice.StringValue == nil || *u.ToolChoice.StringValue != "none" {
					t.Error("expected tool_choice 'none'")
				}
			},
		},
		{
			name: "tool_choice_required",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"tool_choice": "required"
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if u.ToolChoice == nil {
					t.Fatal("ToolChoice is nil")
				}
				if u.ToolChoice.StringValue == nil || *u.ToolChoice.StringValue != "required" {
					t.Error("expected tool_choice 'required'")
				}
			},
		},
		{
			name: "tool_choice_object",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"tool_choice": {
					"type": "function",
					"function": {"name": "get_weather"}
				}
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if u.ToolChoice == nil {
					t.Fatal("ToolChoice is nil")
				}
				if u.ToolChoice.ObjectValue == nil {
					t.Fatal("ToolChoice.ObjectValue is nil")
				}
				if u.ToolChoice.ObjectValue.Type != "function" {
					t.Errorf("expected type 'function', got %v", u.ToolChoice.ObjectValue.Type)
				}
				if u.ToolChoice.ObjectValue.Function == nil {
					t.Fatal("Function is nil")
				}
				if u.ToolChoice.ObjectValue.Function.Name != "get_weather" {
					t.Errorf("expected name 'get_weather', got %v", u.ToolChoice.ObjectValue.Function.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := TransformOpenAIToUnified(context.Background(), []byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, unified)
		})
	}
}

func TestTransformOpenAIToUnified_StreamOptions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, *models.UnifiedRequest)
	}{
		{
			name: "stream_options_include_usage",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"stream": true,
				"stream_options": {"include_usage": true}
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if u.StreamOptions == nil {
					t.Fatal("StreamOptions is nil")
				}
				if !u.StreamOptions.IncludeUsage {
					t.Error("expected IncludeUsage to be true")
				}
			},
		},
		{
			name: "parallel_tool_calls",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"parallel_tool_calls": false
			}`,
			validate: func(t *testing.T, u *models.UnifiedRequest) {
				if u.ParallelToolCalls == nil {
					t.Fatal("ParallelToolCalls is nil")
				}
				if *u.ParallelToolCalls {
					t.Error("expected ParallelToolCalls to be false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := TransformOpenAIToUnified(context.Background(), []byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.validate(t, unified)
		})
	}
}

func TestTransformUnifiedToOpenAI_Phase2(t *testing.T) {
	// 测试响应格式
	unified1 := &models.UnifiedRequest{
		Model:    "gpt-4",
		Messages: []models.UnifiedMessage{{Role: "user", Content: "hi"}},
		ResponseFormat: &models.UnifiedResponseFormat{
			Type:       "json_object",
			JSONSchema: json.RawMessage(`{"name":"test"}`),
		},
	}

	result1, err := TransformUnifiedToOpenAI(unified1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed1 map[string]interface{}
	if err := json.Unmarshal(result1, &parsed1); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if rf, ok := parsed1["response_format"].(map[string]interface{}); !ok {
		t.Error("response_format not found")
	} else if rf["type"] != "json_object" {
		t.Error("response_format type mismatch")
	}

	// 测试工具选择 (string)
	tcAuto := "auto"
	unified2 := &models.UnifiedRequest{
		Model:      "gpt-4",
		Messages:   []models.UnifiedMessage{{Role: "user", Content: "hi"}},
		ToolChoice: &models.UnifiedToolChoice{StringValue: &tcAuto},
	}

	result2, err := TransformUnifiedToOpenAI(unified2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed2 map[string]interface{}
	if err := json.Unmarshal(result2, &parsed2); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if parsed2["tool_choice"] != "auto" {
		t.Error("tool_choice mismatch")
	}

	// 测试工具选择 (object)
	unified3 := &models.UnifiedRequest{
		Model:    "gpt-4",
		Messages: []models.UnifiedMessage{{Role: "user", Content: "hi"}},
		ToolChoice: &models.UnifiedToolChoice{
			ObjectValue: &models.UnifiedToolChoiceObject{
				Type: "function",
				Function: &models.UnifiedToolChoiceFunction{
					Name: "get_weather",
				},
			},
		},
	}

	result3, err := TransformUnifiedToOpenAI(unified3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed3 map[string]interface{}
	if err := json.Unmarshal(result3, &parsed3); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if tc, ok := parsed3["tool_choice"].(map[string]interface{}); !ok {
		t.Error("tool_choice not found")
	} else if tc["type"] != "function" {
		t.Error("tool_choice type mismatch")
	}

	// 测试 parallel_tool_calls
	ptc := false
	unified4 := &models.UnifiedRequest{
		Model:             "gpt-4",
		Messages:          []models.UnifiedMessage{{Role: "user", Content: "hi"}},
		ParallelToolCalls: &ptc,
	}

	result4, err := TransformUnifiedToOpenAI(unified4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed4 map[string]interface{}
	if err := json.Unmarshal(result4, &parsed4); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if parsed4["parallel_tool_calls"] != false {
		t.Error("parallel_tool_calls mismatch")
	}

	// 测试 stream_options
	unified5 := &models.UnifiedRequest{
		Model:    "gpt-4",
		Messages: []models.UnifiedMessage{{Role: "user", Content: "hi"}},
		Stream:   true,
		StreamOptions: &models.UnifiedStreamOptions{
			IncludeUsage: true,
		},
	}

	result5, err := TransformUnifiedToOpenAI(unified5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed5 map[string]interface{}
	if err := json.Unmarshal(result5, &parsed5); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if so, ok := parsed5["stream_options"].(map[string]interface{}); !ok {
		t.Error("stream_options not found")
	} else if so["include_usage"] != true {
		t.Error("stream_options include_usage mismatch")
	}
}

func TestUnifiedToolChoice_JSON(t *testing.T) {
	// 测试 string 序列化
	tcAuto := "auto"
	tc1 := models.UnifiedToolChoice{StringValue: &tcAuto}
	data1, err := json.Marshal(tc1)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data1) != `"auto"` {
		t.Errorf("expected \"auto\", got %s", string(data1))
	}

	// 测试 object 序列化
	tc2 := models.UnifiedToolChoice{
		ObjectValue: &models.UnifiedToolChoiceObject{
			Type: "function",
			Function: &models.UnifiedToolChoiceFunction{
				Name: "get_weather",
			},
		},
	}
	data2, err := json.Marshal(tc2)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data2, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if parsed["type"] != "function" {
		t.Error("type mismatch")
	}

	// 测试 string 反序列化
	var tc3 models.UnifiedToolChoice
	if err := json.Unmarshal([]byte(`"none"`), &tc3); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if tc3.StringValue == nil || *tc3.StringValue != "none" {
		t.Error("string unmarshal failed")
	}

	// 测试 object 反序列化
	var tc4 models.UnifiedToolChoice
	if err := json.Unmarshal([]byte(`{"type":"function","function":{"name":"test"}}`), &tc4); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if tc4.ObjectValue == nil || tc4.ObjectValue.Type != "function" {
		t.Error("object unmarshal failed")
	}
}
