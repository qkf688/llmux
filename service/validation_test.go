package service

import (
	"testing"
)

// 阶段 6: 参数验证测试

func TestValidateTemperature(t *testing.T) {
	tests := []struct {
		name    string
		temp    *float64
		wantErr bool
	}{
		{"nil temperature", nil, false},
		{"valid temperature 0", floatPtr(0), false},
		{"valid temperature 1", floatPtr(1), false},
		{"valid temperature 2", floatPtr(2), false},
		{"invalid temperature -0.1", floatPtr(-0.1), true},
		{"invalid temperature 2.1", floatPtr(2.1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTemperature(tt.temp)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTemperature() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTopP(t *testing.T) {
	tests := []struct {
		name    string
		topP    *float64
		wantErr bool
	}{
		{"nil top_p", nil, false},
		{"valid top_p 0", floatPtr(0), false},
		{"valid top_p 0.5", floatPtr(0.5), false},
		{"valid top_p 1", floatPtr(1), false},
		{"invalid top_p -0.1", floatPtr(-0.1), true},
		{"invalid top_p 1.1", floatPtr(1.1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTopP(tt.topP)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTopP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFrequencyPenalty(t *testing.T) {
	tests := []struct {
		name    string
		penalty *float64
		wantErr bool
	}{
		{"nil penalty", nil, false},
		{"valid penalty -2", floatPtr(-2), false},
		{"valid penalty 0", floatPtr(0), false},
		{"valid penalty 2", floatPtr(2), false},
		{"invalid penalty -2.1", floatPtr(-2.1), true},
		{"invalid penalty 2.1", floatPtr(2.1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFrequencyPenalty(tt.penalty)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFrequencyPenalty() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePresencePenalty(t *testing.T) {
	tests := []struct {
		name    string
		penalty *float64
		wantErr bool
	}{
		{"nil penalty", nil, false},
		{"valid penalty -2", floatPtr(-2), false},
		{"valid penalty 0", floatPtr(0), false},
		{"valid penalty 2", floatPtr(2), false},
		{"invalid penalty -2.1", floatPtr(-2.1), true},
		{"invalid penalty 2.1", floatPtr(2.1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePresencePenalty(tt.penalty)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePresencePenalty() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTopLogprobs(t *testing.T) {
	tests := []struct {
		name    string
		value   *int64
		wantErr bool
	}{
		{"nil value", nil, false},
		{"valid value 0", int64Ptr(0), false},
		{"valid value 10", int64Ptr(10), false},
		{"valid value 20", int64Ptr(20), false},
		{"invalid value -1", int64Ptr(-1), true},
		{"invalid value 21", int64Ptr(21), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTopLogprobs(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTopLogprobs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUnifiedRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *UnifiedRequest
		wantErr bool
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "missing model",
			req: &UnifiedRequest{
				Messages: []UnifiedMessage{{Role: "user", Content: "Hello"}},
			},
			wantErr: true,
		},
		{
			name: "empty messages",
			req: &UnifiedRequest{
				Model:    "gpt-4",
				Messages: []UnifiedMessage{},
			},
			wantErr: true,
		},
		{
			name: "valid request",
			req: &UnifiedRequest{
				Model:       "gpt-4",
				Messages:    []UnifiedMessage{{Role: "user", Content: "Hello"}},
				Temperature: floatPtr(0.7),
				TopP:        floatPtr(0.9),
			},
			wantErr: false,
		},
		{
			name: "invalid temperature",
			req: &UnifiedRequest{
				Model:       "gpt-4",
				Messages:    []UnifiedMessage{{Role: "user", Content: "Hello"}},
				Temperature: floatPtr(3.0),
			},
			wantErr: true,
		},
		{
			name: "invalid top_p",
			req: &UnifiedRequest{
				Model:    "gpt-4",
				Messages: []UnifiedMessage{{Role: "user", Content: "Hello"}},
				TopP:     floatPtr(1.5),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUnifiedRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUnifiedRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// 阶段 6: 参数修复测试

func TestClampFloat64(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{"value in range", 1.0, 0.0, 2.0, 1.0},
		{"value below min", -1.0, 0.0, 2.0, 0.0},
		{"value above max", 3.0, 0.0, 2.0, 2.0},
		{"value at min", 0.0, 0.0, 2.0, 0.0},
		{"value at max", 2.0, 0.0, 2.0, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clampFloat64(tt.value, tt.min, tt.max)
			if result != tt.expected {
				t.Errorf("clampFloat64() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestClampInt64(t *testing.T) {
	tests := []struct {
		name     string
		value    int64
		min      int64
		max      int64
		expected int64
	}{
		{"value in range", 10, 0, 20, 10},
		{"value below min", -5, 0, 20, 0},
		{"value above max", 25, 0, 20, 20},
		{"value at min", 0, 0, 20, 0},
		{"value at max", 20, 0, 20, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clampInt64(tt.value, tt.min, tt.max)
			if result != tt.expected {
				t.Errorf("clampInt64() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRepairInvalidJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid JSON", `{"key": "value"}`, `{"key": "value"}`},
		{"invalid JSON", `{invalid}`, `{}`},
		{"empty string", ``, `{}`},
		{"null", `null`, `null`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repairInvalidJSON(tt.input)
			if result != tt.expected {
				t.Errorf("repairInvalidJSON() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRepairUnifiedRequest(t *testing.T) {
	tests := []struct {
		name     string
		input    *UnifiedRequest
		validate func(*testing.T, *UnifiedRequest)
	}{
		{
			name:  "nil request",
			input: nil,
			validate: func(t *testing.T, req *UnifiedRequest) {
				// Should not panic
			},
		},
		{
			name: "repair temperature",
			input: &UnifiedRequest{
				Model:       "gpt-4",
				Messages:    []UnifiedMessage{{Role: "user", Content: "Hello"}},
				Temperature: floatPtr(3.0),
			},
			validate: func(t *testing.T, req *UnifiedRequest) {
				if req.Temperature == nil || *req.Temperature != 2.0 {
					t.Errorf("Expected temperature to be clamped to 2.0, got %v", req.Temperature)
				}
			},
		},
		{
			name: "repair top_p",
			input: &UnifiedRequest{
				Model:    "gpt-4",
				Messages: []UnifiedMessage{{Role: "user", Content: "Hello"}},
				TopP:     floatPtr(1.5),
			},
			validate: func(t *testing.T, req *UnifiedRequest) {
				if req.TopP == nil || *req.TopP != 1.0 {
					t.Errorf("Expected top_p to be clamped to 1.0, got %v", req.TopP)
				}
			},
		},
		{
			name: "repair frequency_penalty",
			input: &UnifiedRequest{
				Model:            "gpt-4",
				Messages:         []UnifiedMessage{{Role: "user", Content: "Hello"}},
				FrequencyPenalty: floatPtr(3.0),
			},
			validate: func(t *testing.T, req *UnifiedRequest) {
				if req.FrequencyPenalty == nil || *req.FrequencyPenalty != 2.0 {
					t.Errorf("Expected frequency_penalty to be clamped to 2.0, got %v", req.FrequencyPenalty)
				}
			},
		},
		{
			name: "repair presence_penalty",
			input: &UnifiedRequest{
				Model:           "gpt-4",
				Messages:        []UnifiedMessage{{Role: "user", Content: "Hello"}},
				PresencePenalty: floatPtr(-3.0),
			},
			validate: func(t *testing.T, req *UnifiedRequest) {
				if req.PresencePenalty == nil || *req.PresencePenalty != -2.0 {
					t.Errorf("Expected presence_penalty to be clamped to -2.0, got %v", req.PresencePenalty)
				}
			},
		},
		{
			name: "repair top_logprobs",
			input: &UnifiedRequest{
				Model:       "gpt-4",
				Messages:    []UnifiedMessage{{Role: "user", Content: "Hello"}},
				TopLogprobs: int64Ptr(25),
			},
			validate: func(t *testing.T, req *UnifiedRequest) {
				if req.TopLogprobs == nil || *req.TopLogprobs != 20 {
					t.Errorf("Expected top_logprobs to be clamped to 20, got %v", req.TopLogprobs)
				}
			},
		},
		{
			name: "repair negative max_tokens",
			input: &UnifiedRequest{
				Model:     "gpt-4",
				Messages:  []UnifiedMessage{{Role: "user", Content: "Hello"}},
				MaxTokens: -100,
			},
			validate: func(t *testing.T, req *UnifiedRequest) {
				if req.MaxTokens != 1 {
					t.Errorf("Expected max_tokens to be repaired to 1, got %d", req.MaxTokens)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RepairUnifiedRequest(tt.input)
			tt.validate(t, tt.input)
		})
	}
}

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func int64Ptr(i int64) *int64 {
	return &i
}
