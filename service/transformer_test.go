package service

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTransformOpenAIToUnified(t *testing.T) {
	openaiRequest := []byte(`{
		"model": "gpt-4",
		"messages": [
			{"role": "user", "content": "Hello"}
		],
		"max_tokens": 100,
		"temperature": 0.7,
		"stream": false
	}`)

	unified, err := TransformOpenAIToUnified(context.Background(), openaiRequest)
	if err != nil {
		t.Fatalf("TransformOpenAIToUnified failed: %v", err)
	}

	if unified.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", unified.Model)
	}

	if len(unified.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(unified.Messages))
	}

	if unified.MaxTokens != 100 {
		t.Errorf("Expected max_tokens 100, got %d", unified.MaxTokens)
	}
}

func TestTransformAnthropicToUnified(t *testing.T) {
	anthropicRequest := []byte(`{
		"model": "claude-3-opus",
		"messages": [
			{"role": "user", "content": "Hello"}
		],
		"max_tokens": 100,
		"temperature": 0.7,
		"stream": false
	}`)

	unified, err := TransformAnthropicToUnified(anthropicRequest)
	if err != nil {
		t.Fatalf("TransformAnthropicToUnified failed: %v", err)
	}

	if unified.Model != "claude-3-opus" {
		t.Errorf("Expected model 'claude-3-opus', got '%s'", unified.Model)
	}

	if len(unified.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(unified.Messages))
	}
}

func TestTransformUnifiedToOpenAI(t *testing.T) {
	temp := 0.7
	unified := &UnifiedRequest{
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: &temp,
		Stream:      false,
		Messages: []UnifiedMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	result, err := TransformUnifiedToOpenAI(unified)
	if err != nil {
		t.Fatalf("TransformUnifiedToOpenAI failed: %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}

func TestTransformUnifiedToAnthropic(t *testing.T) {
	temp := 0.7
	unified := &UnifiedRequest{
		Model:       "claude-3-opus",
		MaxTokens:   100,
		Temperature: &temp,
		Stream:      false,
		Messages: []UnifiedMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	result, err := TransformUnifiedToAnthropic(unified)
	if err != nil {
		t.Fatalf("TransformUnifiedToAnthropic failed: %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}

func TestTransformerManager(t *testing.T) {
	// 测试 OpenAI 客户端 -> Anthropic 供应商
	tm := NewTransformerManager("openai", "anthropic")

	openaiRequest := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "Hello"}],
		"max_tokens": 100
	}`)

	result, err := tm.ProcessRequest(context.Background(), openaiRequest)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}

	if len(result) == 0 {
		t.Error("Expected non-empty result")
	}
}

// 阶段 1: 测试基础高级参数
func TestTransformOpenAIToUnified_AdvancedParams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, *UnifiedRequest)
	}{
		{
			name: "frequency_penalty",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"frequency_penalty": 0.5
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.FrequencyPenalty == nil {
					t.Fatal("FrequencyPenalty is nil")
				}
				if *u.FrequencyPenalty != 0.5 {
					t.Errorf("expected 0.5, got %v", *u.FrequencyPenalty)
				}
			},
		},
		{
			name: "presence_penalty",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"presence_penalty": -0.3
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.PresencePenalty == nil {
					t.Fatal("PresencePenalty is nil")
				}
				if *u.PresencePenalty != -0.3 {
					t.Errorf("expected -0.3, got %v", *u.PresencePenalty)
				}
			},
		},
		{
			name: "seed",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"seed": 12345
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.Seed == nil {
					t.Fatal("Seed is nil")
				}
				if *u.Seed != 12345 {
					t.Errorf("expected 12345, got %v", *u.Seed)
				}
			},
		},
		{
			name: "logit_bias",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"logit_bias": {"50256": -100, "50257": 100}
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.LogitBias == nil {
					t.Fatal("LogitBias is nil")
				}
				if u.LogitBias["50256"] != -100 {
					t.Errorf("expected -100, got %v", u.LogitBias["50256"])
				}
				if u.LogitBias["50257"] != 100 {
					t.Errorf("expected 100, got %v", u.LogitBias["50257"])
				}
			},
		},
		{
			name: "stop_string",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"stop": "END"
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.Stop == nil {
					t.Fatal("Stop is nil")
				}
				if u.Stop.Single == nil {
					t.Fatal("Stop.Single is nil")
				}
				if *u.Stop.Single != "END" {
					t.Errorf("expected 'END', got %v", *u.Stop.Single)
				}
			},
		},
		{
			name: "stop_array",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"stop": ["END", "STOP"]
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.Stop == nil {
					t.Fatal("Stop is nil")
				}
				if len(u.Stop.Multiple) != 2 {
					t.Fatalf("expected 2 stops, got %d", len(u.Stop.Multiple))
				}
				if u.Stop.Multiple[0] != "END" || u.Stop.Multiple[1] != "STOP" {
					t.Errorf("unexpected stop values: %v", u.Stop.Multiple)
				}
			},
		},
		{
			name: "user",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"user": "user-123"
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.User == nil {
					t.Fatal("User is nil")
				}
				if *u.User != "user-123" {
					t.Errorf("expected 'user-123', got %v", *u.User)
				}
			},
		},
		{
			name: "metadata",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"metadata": {"key1": "value1", "key2": "value2"}
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.Metadata == nil {
					t.Fatal("Metadata is nil")
				}
				if u.Metadata["key1"] != "value1" {
					t.Errorf("expected 'value1', got %v", u.Metadata["key1"])
				}
				if u.Metadata["key2"] != "value2" {
					t.Errorf("expected 'value2', got %v", u.Metadata["key2"])
				}
			},
		},
		{
			name: "logprobs",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"logprobs": true,
				"top_logprobs": 5
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.Logprobs == nil {
					t.Fatal("Logprobs is nil")
				}
				if !*u.Logprobs {
					t.Error("expected Logprobs to be true")
				}
				if u.TopLogprobs == nil {
					t.Fatal("TopLogprobs is nil")
				}
				if *u.TopLogprobs != 5 {
					t.Errorf("expected 5, got %v", *u.TopLogprobs)
				}
			},
		},
		{
			name: "max_completion_tokens",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"max_completion_tokens": 2000
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.MaxCompletionTokens == nil {
					t.Fatal("MaxCompletionTokens is nil")
				}
				if *u.MaxCompletionTokens != 2000 {
					t.Errorf("expected 2000, got %v", *u.MaxCompletionTokens)
				}
			},
		},
		{
			name: "store",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"store": true
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.Store == nil {
					t.Fatal("Store is nil")
				}
				if !*u.Store {
					t.Error("expected Store to be true")
				}
			},
		},
		{
			name: "all_fields",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"frequency_penalty": 0.5,
				"presence_penalty": -0.3,
				"seed": 12345,
				"stop": ["END"],
				"user": "user-123",
				"metadata": {"key": "value"}
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if u.FrequencyPenalty == nil || *u.FrequencyPenalty != 0.5 {
					t.Error("FrequencyPenalty mismatch")
				}
				if u.PresencePenalty == nil || *u.PresencePenalty != -0.3 {
					t.Error("PresencePenalty mismatch")
				}
				if u.Seed == nil || *u.Seed != 12345 {
					t.Error("Seed mismatch")
				}
				if u.Stop == nil || len(u.Stop.Multiple) != 1 {
					t.Error("Stop mismatch")
				}
				if u.User == nil || *u.User != "user-123" {
					t.Error("User mismatch")
				}
				if u.Metadata == nil || u.Metadata["key"] != "value" {
					t.Error("Metadata mismatch")
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

func TestTransformUnifiedToOpenAI_AdvancedParams(t *testing.T) {
	freq := 0.5
	pres := -0.3
	seed := int64(12345)
	user := "user-123"
	logprobs := true
	topLogprobs := int64(5)
	maxCompTokens := int64(2000)
	store := true

	unified := &UnifiedRequest{
		Model:               "gpt-4",
		Messages:            []UnifiedMessage{{Role: "user", Content: "hi"}},
		FrequencyPenalty:    &freq,
		PresencePenalty:     &pres,
		Seed:                &seed,
		LogitBias:           map[string]int64{"50256": -100},
		Stop:                &UnifiedStop{Multiple: []string{"END"}},
		User:                &user,
		Metadata:            map[string]string{"key": "value"},
		Logprobs:            &logprobs,
		TopLogprobs:         &topLogprobs,
		MaxCompletionTokens: &maxCompTokens,
		Store:               &store,
	}

	result, err := TransformUnifiedToOpenAI(unified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// 验证字段存在
	if parsed["frequency_penalty"] != 0.5 {
		t.Errorf("frequency_penalty mismatch")
	}
	if parsed["presence_penalty"] != -0.3 {
		t.Errorf("presence_penalty mismatch")
	}
	if parsed["seed"] != float64(12345) {
		t.Errorf("seed mismatch")
	}
	if parsed["user"] != "user-123" {
		t.Errorf("user mismatch")
	}
	if parsed["logprobs"] != true {
		t.Errorf("logprobs mismatch")
	}
	if parsed["top_logprobs"] != float64(5) {
		t.Errorf("top_logprobs mismatch")
	}
	if parsed["max_completion_tokens"] != float64(2000) {
		t.Errorf("max_completion_tokens mismatch")
	}
	if parsed["store"] != true {
		t.Errorf("store mismatch")
	}

	// 验证 stop 数组
	if stopArr, ok := parsed["stop"].([]interface{}); !ok || len(stopArr) != 1 {
		t.Errorf("stop array mismatch")
	}

	// 验证 logit_bias
	if logitBias, ok := parsed["logit_bias"].(map[string]interface{}); !ok {
		t.Errorf("logit_bias mismatch")
	} else if logitBias["50256"] != float64(-100) {
		t.Errorf("logit_bias value mismatch")
	}

	// 验证 metadata
	if metadata, ok := parsed["metadata"].(map[string]interface{}); !ok {
		t.Errorf("metadata mismatch")
	} else if metadata["key"] != "value" {
		t.Errorf("metadata value mismatch")
	}
}

func TestTransformUnifiedToAnthropic_CompatibleFields(t *testing.T) {
	unified := &UnifiedRequest{
		Model:    "claude-3-opus",
		Messages: []UnifiedMessage{{Role: "user", Content: "hi"}},
		Stop:     &UnifiedStop{Multiple: []string{"END", "STOP"}},
		Metadata: map[string]string{"key": "value"},
		// 这些字段 Anthropic 不支持，应该被忽略
		FrequencyPenalty: func() *float64 { v := 0.5; return &v }(),
		Seed:             func() *int64 { v := int64(12345); return &v }(),
	}

	result, err := TransformUnifiedToAnthropic(unified)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	// 验证支持的字段
	if stopSeqs, ok := parsed["stop_sequences"].([]interface{}); !ok || len(stopSeqs) != 2 {
		t.Errorf("stop_sequences mismatch")
	}
	if metadata, ok := parsed["metadata"].(map[string]interface{}); !ok {
		t.Errorf("metadata mismatch")
	} else if metadata["key"] != "value" {
		t.Errorf("metadata value mismatch")
	}

	// 验证不支持的字段被忽略
	if _, ok := parsed["frequency_penalty"]; ok {
		t.Error("frequency_penalty should not be present")
	}
	if _, ok := parsed["seed"]; ok {
		t.Error("seed should not be present")
	}
}

func TestUnifiedStop_JSON(t *testing.T) {
	// 测试 string 序列化
	stop1 := UnifiedStop{Single: func() *string { s := "END"; return &s }()}
	data1, err := json.Marshal(stop1)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data1) != `"END"` {
		t.Errorf("expected \"END\", got %s", string(data1))
	}

	// 测试 []string 序列化
	stop2 := UnifiedStop{Multiple: []string{"END", "STOP"}}
	data2, err := json.Marshal(stop2)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data2) != `["END","STOP"]` {
		t.Errorf("expected [\"END\",\"STOP\"], got %s", string(data2))
	}

	// 测试 string 反序列化
	var stop3 UnifiedStop
	if err := json.Unmarshal([]byte(`"END"`), &stop3); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if stop3.Single == nil || *stop3.Single != "END" {
		t.Error("string unmarshal failed")
	}

	// 测试 []string 反序列化
	var stop4 UnifiedStop
	if err := json.Unmarshal([]byte(`["END","STOP"]`), &stop4); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(stop4.Multiple) != 2 || stop4.Multiple[0] != "END" {
		t.Error("array unmarshal failed")
	}
}

// 阶段 2: 测试响应格式和工具增强
func TestTransformOpenAIToUnified_ResponseFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, *UnifiedRequest)
	}{
		{
			name: "response_format_text",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"response_format": {"type": "text"}
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
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
			validate: func(t *testing.T, u *UnifiedRequest) {
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
			validate: func(t *testing.T, u *UnifiedRequest) {
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
		validate func(*testing.T, *UnifiedRequest)
	}{
		{
			name: "tool_choice_auto",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"tool_choice": "auto"
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
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
			validate: func(t *testing.T, u *UnifiedRequest) {
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
			validate: func(t *testing.T, u *UnifiedRequest) {
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
			validate: func(t *testing.T, u *UnifiedRequest) {
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
		validate func(*testing.T, *UnifiedRequest)
	}{
		{
			name: "stream_options_include_usage",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "hi"}],
				"stream": true,
				"stream_options": {"include_usage": true}
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
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
			validate: func(t *testing.T, u *UnifiedRequest) {
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
	unified1 := &UnifiedRequest{
		Model:    "gpt-4",
		Messages: []UnifiedMessage{{Role: "user", Content: "hi"}},
		ResponseFormat: &UnifiedResponseFormat{
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
	unified2 := &UnifiedRequest{
		Model:      "gpt-4",
		Messages:   []UnifiedMessage{{Role: "user", Content: "hi"}},
		ToolChoice: &UnifiedToolChoice{StringValue: &tcAuto},
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
	unified3 := &UnifiedRequest{
		Model:    "gpt-4",
		Messages: []UnifiedMessage{{Role: "user", Content: "hi"}},
		ToolChoice: &UnifiedToolChoice{
			ObjectValue: &UnifiedToolChoiceObject{
				Type: "function",
				Function: &UnifiedToolChoiceFunction{
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
	unified4 := &UnifiedRequest{
		Model:             "gpt-4",
		Messages:          []UnifiedMessage{{Role: "user", Content: "hi"}},
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
	unified5 := &UnifiedRequest{
		Model:    "gpt-4",
		Messages: []UnifiedMessage{{Role: "user", Content: "hi"}},
		Stream:   true,
		StreamOptions: &UnifiedStreamOptions{
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
	tc1 := UnifiedToolChoice{StringValue: &tcAuto}
	data1, err := json.Marshal(tc1)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data1) != `"auto"` {
		t.Errorf("expected \"auto\", got %s", string(data1))
	}

	// 测试 object 序列化
	tc2 := UnifiedToolChoice{
		ObjectValue: &UnifiedToolChoiceObject{
			Type: "function",
			Function: &UnifiedToolChoiceFunction{
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
	var tc3 UnifiedToolChoice
	if err := json.Unmarshal([]byte(`"none"`), &tc3); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if tc3.StringValue == nil || *tc3.StringValue != "none" {
		t.Error("string unmarshal failed")
	}

	// 测试 object 反序列化
	var tc4 UnifiedToolChoice
	if err := json.Unmarshal([]byte(`{"type":"function","function":{"name":"test"}}`), &tc4); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if tc4.ObjectValue == nil || tc4.ObjectValue.Type != "function" {
		t.Error("object unmarshal failed")
	}
}

// 阶段 3: 测试多模态内容支持
func TestTransformOpenAIToUnified_MultimodalContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, *UnifiedRequest)
	}{
		{
			name: "text_only",
			input: `{
				"model": "gpt-4",
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if len(u.Messages) != 1 {
					t.Fatal("expected 1 message")
				}
				if str, ok := u.Messages[0].Content.(string); !ok || str != "Hello" {
					t.Error("content should be string")
				}
			},
		},
		{
			name: "text_and_image",
			input: `{
				"model": "gpt-4-vision",
				"messages": [{
					"role": "user",
					"content": [
						{"type": "text", "text": "What's in this image?"},
						{"type": "image_url", "image_url": {"url": "https://example.com/image.jpg", "detail": "high"}}
					]
				}]
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if len(u.Messages) != 1 {
					t.Fatal("expected 1 message")
				}
				parts, ok := u.Messages[0].Content.([]UnifiedMessageContentPart)
				if !ok {
					t.Fatal("content should be []UnifiedMessageContentPart")
				}
				if len(parts) != 2 {
					t.Fatalf("expected 2 parts, got %d", len(parts))
				}
				if parts[0].Type != "text" || parts[0].Text == nil || *parts[0].Text != "What's in this image?" {
					t.Error("first part should be text")
				}
				if parts[1].Type != "image_url" || parts[1].ImageURL == nil {
					t.Error("second part should be image_url")
				}
				if parts[1].ImageURL.URL != "https://example.com/image.jpg" {
					t.Error("image URL mismatch")
				}
				if parts[1].ImageURL.Detail == nil || *parts[1].ImageURL.Detail != "high" {
					t.Error("image detail mismatch")
				}
			},
		},
		{
			name: "audio_input",
			input: `{
				"model": "gpt-4-audio",
				"messages": [{
					"role": "user",
					"content": [
						{"type": "text", "text": "Transcribe this audio"},
						{"type": "input_audio", "input_audio": {"data": "base64data", "format": "wav"}}
					]
				}]
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if len(u.Messages) != 1 {
					t.Fatal("expected 1 message")
				}
				parts, ok := u.Messages[0].Content.([]UnifiedMessageContentPart)
				if !ok {
					t.Fatal("content should be []UnifiedMessageContentPart")
				}
				if len(parts) != 2 {
					t.Fatalf("expected 2 parts, got %d", len(parts))
				}
				if parts[1].Type != "input_audio" || parts[1].InputAudio == nil {
					t.Error("second part should be input_audio")
				}
				if parts[1].InputAudio.Data != "base64data" {
					t.Error("audio data mismatch")
				}
				if parts[1].InputAudio.Format != "wav" {
					t.Error("audio format mismatch")
				}
			},
		},
		{
			name: "modalities_and_audio",
			input: `{
				"model": "gpt-4-audio",
				"messages": [{"role": "user", "content": "Generate audio"}],
				"modalities": ["text", "audio"],
				"audio": {"voice": "alloy", "format": "wav"}
			}`,
			validate: func(t *testing.T, u *UnifiedRequest) {
				if len(u.Modalities) != 2 {
					t.Fatalf("expected 2 modalities, got %d", len(u.Modalities))
				}
				if u.Modalities[0] != "text" || u.Modalities[1] != "audio" {
					t.Error("modalities mismatch")
				}
				if u.Audio == nil {
					t.Fatal("audio config is nil")
				}
				if u.Audio.Voice != "alloy" {
					t.Error("audio voice mismatch")
				}
				if u.Audio.Format != "wav" {
					t.Error("audio format mismatch")
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

func TestTransformUnifiedToOpenAI_MultimodalContent(t *testing.T) {
	// 测试纯文本 (向后兼容)
	unified1 := &UnifiedRequest{
		Model:    "gpt-4",
		Messages: []UnifiedMessage{{Role: "user", Content: "Hello"}},
	}

	result1, err := TransformUnifiedToOpenAI(unified1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed1 map[string]interface{}
	if err := json.Unmarshal(result1, &parsed1); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	messages1 := parsed1["messages"].([]interface{})
	msg1 := messages1[0].(map[string]interface{})
	if msg1["content"] != "Hello" {
		t.Error("text content mismatch")
	}

	// 测试多模态内容
	text := "What's in this image?"
	detail := "high"
	unified2 := &UnifiedRequest{
		Model: "gpt-4-vision",
		Messages: []UnifiedMessage{{
			Role: "user",
			Content: []UnifiedMessageContentPart{
				{Type: "text", Text: &text},
				{Type: "image_url", ImageURL: &UnifiedImageURL{
					URL:    "https://example.com/image.jpg",
					Detail: &detail,
				}},
			},
		}},
	}

	result2, err := TransformUnifiedToOpenAI(unified2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed2 map[string]interface{}
	if err := json.Unmarshal(result2, &parsed2); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	messages2 := parsed2["messages"].([]interface{})
	msg2 := messages2[0].(map[string]interface{})
	contentArray := msg2["content"].([]interface{})
	if len(contentArray) != 2 {
		t.Fatalf("expected 2 content parts, got %d", len(contentArray))
	}

	part1 := contentArray[0].(map[string]interface{})
	if part1["type"] != "text" || part1["text"] != "What's in this image?" {
		t.Error("text part mismatch")
	}

	part2 := contentArray[1].(map[string]interface{})
	if part2["type"] != "image_url" {
		t.Error("image_url type mismatch")
	}

	// 测试 Modalities 和 Audio
	unified3 := &UnifiedRequest{
		Model:      "gpt-4-audio",
		Messages:   []UnifiedMessage{{Role: "user", Content: "Generate audio"}},
		Modalities: []string{"text", "audio"},
		Audio: &UnifiedAudio{
			Voice:  "alloy",
			Format: "wav",
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

	modalities := parsed3["modalities"].([]interface{})
	if len(modalities) != 2 {
		t.Error("modalities mismatch")
	}

	audio := parsed3["audio"].(map[string]interface{})
	if audio["voice"] != "alloy" || audio["format"] != "wav" {
		t.Error("audio config mismatch")
	}
}

func TestUnifiedMessage_Helpers(t *testing.T) {
	// 测试 GetContentAsString - 纯文本
	msg1 := UnifiedMessage{Content: "Hello"}
	if msg1.GetContentAsString() != "Hello" {
		t.Error("GetContentAsString failed for string")
	}

	// 测试 GetContentAsString - 多模态
	text1 := "Part 1"
	text2 := "Part 2"
	msg2 := UnifiedMessage{
		Content: []UnifiedMessageContentPart{
			{Type: "text", Text: &text1},
			{Type: "image_url", ImageURL: &UnifiedImageURL{URL: "http://example.com/img.jpg"}},
			{Type: "text", Text: &text2},
		},
	}
	if msg2.GetContentAsString() != "Part 1Part 2" {
		t.Error("GetContentAsString failed for multimodal")
	}

	// 测试 GetContentParts - 纯文本
	parts1 := msg1.GetContentParts()
	if len(parts1) != 1 || parts1[0].Type != "text" || *parts1[0].Text != "Hello" {
		t.Error("GetContentParts failed for string")
	}

	// 测试 GetContentParts - 多模态
	parts2 := msg2.GetContentParts()
	if len(parts2) != 3 {
		t.Error("GetContentParts failed for multimodal")
	}

	// 测试 SetContentString
	msg3 := UnifiedMessage{}
	msg3.SetContentString("Test")
	if msg3.Content != "Test" {
		t.Error("SetContentString failed")
	}

	// 测试 SetContentParts
	msg4 := UnifiedMessage{}
	msg4.SetContentParts([]UnifiedMessageContentPart{
		{Type: "text", Text: &text1},
	})
	if parts, ok := msg4.Content.([]UnifiedMessageContentPart); !ok || len(parts) != 1 {
		t.Error("SetContentParts failed")
	}
}

func TestUnifiedMessageContent_JSON(t *testing.T) {
	// 测试纯文本序列化
	content1 := UnifiedMessageContent{Content: func() *string { s := "Hello"; return &s }()}
	data1, err := json.Marshal(content1)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data1) != `"Hello"` {
		t.Errorf("expected \"Hello\", got %s", string(data1))
	}

	// 测试多模态序列化
	text := "Test"
	content2 := UnifiedMessageContent{
		MultipleContent: []UnifiedMessageContentPart{
			{Type: "text", Text: &text},
		},
	}
	data2, err := json.Marshal(content2)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	// 单个 text 部分应该优化为字符串
	if string(data2) != `"Test"` {
		t.Errorf("expected \"Test\", got %s", string(data2))
	}

	// 测试多个部分序列化
	content3 := UnifiedMessageContent{
		MultipleContent: []UnifiedMessageContentPart{
			{Type: "text", Text: &text},
			{Type: "image_url", ImageURL: &UnifiedImageURL{URL: "http://example.com/img.jpg"}},
		},
	}
	data3, err := json.Marshal(content3)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed []interface{}
	if err := json.Unmarshal(data3, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(parsed) != 2 {
		t.Error("expected 2 parts in array")
	}

	// 测试纯文本反序列化
	var content4 UnifiedMessageContent
	if err := json.Unmarshal([]byte(`"Hello"`), &content4); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if content4.Content == nil || *content4.Content != "Hello" {
		t.Error("string unmarshal failed")
	}

	// 测试多模态反序列化
	var content5 UnifiedMessageContent
	if err := json.Unmarshal([]byte(`[{"type":"text","text":"Test"}]`), &content5); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(content5.MultipleContent) != 1 || content5.MultipleContent[0].Type != "text" {
		t.Error("array unmarshal failed")
	}
}

// 阶段 1: 缓存控制测试

func TestCacheControl_MessageLevel(t *testing.T) {
	// 测试消息级别的缓存控制
	anthropicRequest := []byte(`{
		"model": "claude-3-opus",
		"messages": [
			{
				"role": "user",
				"content": "Hello",
				"cache_control": {"type": "ephemeral"}
			}
		],
		"max_tokens": 100
	}`)

	unified, err := TransformAnthropicToUnified(anthropicRequest)
	if err != nil {
		t.Fatalf("TransformAnthropicToUnified failed: %v", err)
	}

	if len(unified.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(unified.Messages))
	}

	if unified.Messages[0].CacheControl == nil {
		t.Fatal("Expected CacheControl to be set")
	}

	if unified.Messages[0].CacheControl.Type != "ephemeral" {
		t.Errorf("Expected cache_control type 'ephemeral', got '%s'", unified.Messages[0].CacheControl.Type)
	}
}

func TestCacheControl_ToolLevel(t *testing.T) {
	// 测试工具级别的缓存控制
	anthropicRequest := []byte(`{
		"model": "claude-3-opus",
		"messages": [
			{"role": "user", "content": "Hello"}
		],
		"tools": [
			{
				"name": "get_weather",
				"description": "Get weather info",
				"input_schema": {"type": "object"},
				"cache_control": {"type": "ephemeral"}
			}
		],
		"max_tokens": 100
	}`)

	unified, err := TransformAnthropicToUnified(anthropicRequest)
	if err != nil {
		t.Fatalf("TransformAnthropicToUnified failed: %v", err)
	}

	if len(unified.Tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(unified.Tools))
	}

	if unified.Tools[0].CacheControl == nil {
		t.Fatal("Expected CacheControl to be set")
	}

	if unified.Tools[0].CacheControl.Type != "ephemeral" {
		t.Errorf("Expected cache_control type 'ephemeral', got '%s'", unified.Tools[0].CacheControl.Type)
	}
}

func TestCacheControl_UnifiedToAnthropic_Message(t *testing.T) {
	// 测试 Unified → Anthropic 消息级别缓存控制转换
	temp := 0.7
	unified := &UnifiedRequest{
		Model:       "claude-3-opus",
		MaxTokens:   100,
		Temperature: &temp,
		Messages: []UnifiedMessage{
			{
				Role:    "user",
				Content: "Hello",
				CacheControl: &CacheControl{
					Type: "ephemeral",
				},
			},
		},
	}

	result, err := TransformUnifiedToAnthropic(unified)
	if err != nil {
		t.Fatalf("TransformUnifiedToAnthropic failed: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	messages, ok := req["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		t.Fatal("Expected messages array")
	}

	msg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected message to be a map")
	}

	cacheControl, ok := msg["cache_control"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected cache_control to be set")
	}

	if cacheControl["type"] != "ephemeral" {
		t.Errorf("Expected cache_control type 'ephemeral', got '%v'", cacheControl["type"])
	}
}

func TestCacheControl_UnifiedToAnthropic_Tool(t *testing.T) {
	// 测试 Unified → Anthropic 工具级别缓存控制转换
	temp := 0.7
	unified := &UnifiedRequest{
		Model:       "claude-3-opus",
		MaxTokens:   100,
		Temperature: &temp,
		Messages: []UnifiedMessage{
			{Role: "user", Content: "Hello"},
		},
		Tools: []UnifiedTool{
			{
				Type: "function",
				Function: UnifiedFunc{
					Name:        "get_weather",
					Description: "Get weather info",
					Parameters:  map[string]interface{}{"type": "object"},
				},
				CacheControl: &CacheControl{
					Type: "ephemeral",
				},
			},
		},
	}

	result, err := TransformUnifiedToAnthropic(unified)
	if err != nil {
		t.Fatalf("TransformUnifiedToAnthropic failed: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	tools, ok := req["tools"].([]interface{})
	if !ok || len(tools) == 0 {
		t.Fatal("Expected tools array")
	}

	tool, ok := tools[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected tool to be a map")
	}

	cacheControl, ok := tool["cache_control"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected cache_control to be set")
	}

	if cacheControl["type"] != "ephemeral" {
		t.Errorf("Expected cache_control type 'ephemeral', got '%v'", cacheControl["type"])
	}
}

func TestCacheControl_ContentPartLevel(t *testing.T) {
	// 测试内容部分级别的缓存控制
	temp := 0.7
	text := "This is a long context that should be cached"
	unified := &UnifiedRequest{
		Model:       "claude-3-opus",
		MaxTokens:   100,
		Temperature: &temp,
		Messages: []UnifiedMessage{
			{
				Role: "user",
				Content: []UnifiedMessageContentPart{
					{
						Type: "text",
						Text: &text,
						CacheControl: &CacheControl{
							Type: "ephemeral",
						},
					},
				},
			},
		},
	}

	result, err := TransformUnifiedToAnthropic(unified)
	if err != nil {
		t.Fatalf("TransformUnifiedToAnthropic failed: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	messages, ok := req["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		t.Fatal("Expected messages array")
	}

	msg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected message to be a map")
	}

	content, ok := msg["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("Expected content array")
	}

	part, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected content part to be a map")
	}

	cacheControl, ok := part["cache_control"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected cache_control to be set")
	}

	if cacheControl["type"] != "ephemeral" {
		t.Errorf("Expected cache_control type 'ephemeral', got '%v'", cacheControl["type"])
	}
}

func TestCacheControl_OpenAI_Ignored(t *testing.T) {
	// 测试 OpenAI 格式忽略缓存控制（不报错）
	temp := 0.7
	unified := &UnifiedRequest{
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: &temp,
		Messages: []UnifiedMessage{
			{
				Role:    "user",
				Content: "Hello",
				CacheControl: &CacheControl{
					Type: "ephemeral",
				},
			},
		},
	}

	result, err := TransformUnifiedToOpenAI(unified)
	if err != nil {
		t.Fatalf("TransformUnifiedToOpenAI failed: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	messages, ok := req["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		t.Fatal("Expected messages array")
	}

	msg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected message to be a map")
	}

	// OpenAI 不支持 cache_control，应该被忽略
	if _, exists := msg["cache_control"]; exists {
		t.Error("Expected cache_control to be ignored for OpenAI format")
	}
}

// 阶段 2: 思考配置测试

func TestThinking_AnthropicToUnified(t *testing.T) {
	// 测试 Anthropic thinking 配置解析
	anthropicRequest := []byte(`{
		"model": "claude-3-opus",
		"messages": [
			{"role": "user", "content": "Solve this complex problem"}
		],
		"max_tokens": 100,
		"thinking": {
			"type": "enabled",
			"budget_tokens": 30000
		}
	}`)

	unified, err := TransformAnthropicToUnified(anthropicRequest)
	if err != nil {
		t.Fatalf("TransformAnthropicToUnified failed: %v", err)
	}

	if unified.ReasoningEffort == nil {
		t.Fatal("Expected ReasoningEffort to be set")
	}

	if *unified.ReasoningEffort != "medium" {
		t.Errorf("Expected reasoning_effort 'medium', got '%s'", *unified.ReasoningEffort)
	}

	if unified.ReasoningBudget == nil {
		t.Fatal("Expected ReasoningBudget to be set")
	}

	if *unified.ReasoningBudget != 30000 {
		t.Errorf("Expected reasoning_budget 30000, got %d", *unified.ReasoningBudget)
	}
}

func TestThinking_BudgetMapping(t *testing.T) {
	tests := []struct {
		name           string
		budgetTokens   int64
		expectedEffort string
	}{
		{"high effort", 50000, "high"},
		{"medium effort", 30000, "medium"},
		{"low effort", 5000, "low"},
		{"zero budget", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effort := thinkingBudgetToReasoningEffort(tt.budgetTokens)
			if effort != tt.expectedEffort {
				t.Errorf("Expected effort '%s', got '%s'", tt.expectedEffort, effort)
			}
		})
	}
}

func TestThinking_EffortMapping(t *testing.T) {
	tests := []struct {
		name           string
		effort         string
		expectedBudget int64
	}{
		{"high effort", "high", 50000},
		{"medium effort", "medium", 20000},
		{"low effort", "low", 1000},
		{"empty effort", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget := reasoningEffortToThinkingBudget(tt.effort)
			if budget != tt.expectedBudget {
				t.Errorf("Expected budget %d, got %d", tt.expectedBudget, budget)
			}
		})
	}
}

func TestThinking_UnifiedToAnthropic_WithBudget(t *testing.T) {
	// 测试 Unified → Anthropic 转换 (使用 ReasoningBudget)
	temp := 0.7
	budget := int64(30000)
	unified := &UnifiedRequest{
		Model:           "claude-3-opus",
		MaxTokens:       100,
		Temperature:     &temp,
		ReasoningBudget: &budget,
		Messages: []UnifiedMessage{
			{Role: "user", Content: "Solve this problem"},
		},
	}

	result, err := TransformUnifiedToAnthropic(unified)
	if err != nil {
		t.Fatalf("TransformUnifiedToAnthropic failed: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	thinking, ok := req["thinking"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected thinking to be set")
	}

	if thinking["type"] != "enabled" {
		t.Errorf("Expected thinking type 'enabled', got '%v'", thinking["type"])
	}

	budgetFloat, ok := thinking["budget_tokens"].(float64)
	if !ok {
		t.Fatal("Expected budget_tokens to be a number")
	}

	if int64(budgetFloat) != 30000 {
		t.Errorf("Expected budget_tokens 30000, got %d", int64(budgetFloat))
	}
}

func TestThinking_UnifiedToAnthropic_WithEffort(t *testing.T) {
	// 测试 Unified → Anthropic 转换 (使用 ReasoningEffort)
	temp := 0.7
	effort := "high"
	unified := &UnifiedRequest{
		Model:           "claude-3-opus",
		MaxTokens:       100,
		Temperature:     &temp,
		ReasoningEffort: &effort,
		Messages: []UnifiedMessage{
			{Role: "user", Content: "Solve this problem"},
		},
	}

	result, err := TransformUnifiedToAnthropic(unified)
	if err != nil {
		t.Fatalf("TransformUnifiedToAnthropic failed: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	thinking, ok := req["thinking"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected thinking to be set")
	}

	if thinking["type"] != "enabled" {
		t.Errorf("Expected thinking type 'enabled', got '%v'", thinking["type"])
	}

	budgetFloat, ok := thinking["budget_tokens"].(float64)
	if !ok {
		t.Fatal("Expected budget_tokens to be a number")
	}

	// "high" effort 应该映射到 50000 tokens
	if int64(budgetFloat) != 50000 {
		t.Errorf("Expected budget_tokens 50000, got %d", int64(budgetFloat))
	}
}

func TestThinking_UnifiedToAnthropic_BudgetPriority(t *testing.T) {
	// 测试 ReasoningBudget 优先于 ReasoningEffort
	temp := 0.7
	effort := "high"
	budget := int64(15000)
	unified := &UnifiedRequest{
		Model:           "claude-3-opus",
		MaxTokens:       100,
		Temperature:     &temp,
		ReasoningEffort: &effort,
		ReasoningBudget: &budget,
		Messages: []UnifiedMessage{
			{Role: "user", Content: "Solve this problem"},
		},
	}

	result, err := TransformUnifiedToAnthropic(unified)
	if err != nil {
		t.Fatalf("TransformUnifiedToAnthropic failed: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	thinking, ok := req["thinking"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected thinking to be set")
	}

	budgetFloat, ok := thinking["budget_tokens"].(float64)
	if !ok {
		t.Fatal("Expected budget_tokens to be a number")
	}

	// 应该使用 ReasoningBudget (15000) 而不是 ReasoningEffort 映射的值 (50000)
	if int64(budgetFloat) != 15000 {
		t.Errorf("Expected budget_tokens 15000 (from ReasoningBudget), got %d", int64(budgetFloat))
	}
}

func TestThinking_OpenAI_Ignored(t *testing.T) {
	// 测试 OpenAI 格式忽略 thinking 配置（不报错）
	temp := 0.7
	budget := int64(30000)
	unified := &UnifiedRequest{
		Model:           "gpt-4",
		MaxTokens:       100,
		Temperature:     &temp,
		ReasoningBudget: &budget,
		Messages: []UnifiedMessage{
			{Role: "user", Content: "Solve this problem"},
		},
	}

	result, err := TransformUnifiedToOpenAI(unified)
	if err != nil {
		t.Fatalf("TransformUnifiedToOpenAI failed: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// OpenAI 不支持 thinking，应该被忽略
	if _, exists := req["thinking"]; exists {
		t.Error("Expected thinking to be ignored for OpenAI format")
	}

	// OpenAI 不支持 reasoning_budget，应该被忽略
	if _, exists := req["reasoning_budget"]; exists {
		t.Error("Expected reasoning_budget to be ignored for OpenAI format")
	}
}
