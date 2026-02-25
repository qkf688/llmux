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
