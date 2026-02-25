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
