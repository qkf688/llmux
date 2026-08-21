package transform

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
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

	unified, err := TransformAnthropicToUnified(context.Background(), anthropicRequest)
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
	unified := &models.UnifiedRequest{
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: &temp,
		Stream:      false,
		Messages: []models.UnifiedMessage{
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
	unified := &models.UnifiedRequest{
		Model:       "claude-3-opus",
		MaxTokens:   100,
		Temperature: &temp,
		Stream:      false,
		Messages: []models.UnifiedMessage{
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

	result, err := tm.ProcessRequest(context.Background(), openaiRequest, nil)
	if err != nil {
		t.Fatalf("ProcessRequest failed: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("unmarshal transformed request: %v", err)
	}
	if out["model"] != "gpt-4" {
		t.Fatalf("expected model gpt-4, got %#v", out["model"])
	}
	if _, ok := out["messages"].([]interface{}); !ok {
		t.Fatalf("expected Anthropic messages array, got %#v", out["messages"])
	}
}

func TestFormatAdapterRegistry(t *testing.T) {
	for _, format := range []consts.WireFormat{consts.FormatOpenAIChat, consts.FormatOpenAIResponses, consts.FormatAnthropic} {
		if _, ok := formatAdapters[format]; !ok {
			t.Fatalf("expected %q adapter to be registered", format)
		}
	}
}

// 未注册的 wire format 必须让 ProcessRequest 显式报错。
//
// 本用例替代了原 TestTransformerManager_UnknownTypesFallBackToOpenAI：那条锁的是
// getAdapterOrDefault 的兜底行为，adapter.go 已刻意去掉该兜底（见那里的注释）。
// 静默按 OpenAI 形状构建出站 body 只会把漏配推到上游 400，根因更难定位。
func TestTransformerManager_UnregisteredFormatReturnsError(t *testing.T) {
	openaiRequest := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "Hello"}],
		"max_tokens": 100
	}`)

	cases := []struct {
		name           string
		clientFormat   consts.WireFormat
		upstreamFormat consts.WireFormat
	}{
		{
			name:           "unregistered client format",
			clientFormat:   "unregistered-format",
			upstreamFormat: consts.FormatAnthropic,
		},
		{
			name:           "unregistered upstream format",
			clientFormat:   consts.FormatOpenAIChat,
			upstreamFormat: "unregistered-format",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tm := NewTransformerManager(tc.clientFormat, tc.upstreamFormat)
			got, err := tm.ProcessRequest(context.Background(), openaiRequest, nil)
			if err == nil {
				t.Fatalf("未注册形状必须报错，实际产出 body: %s", got)
			}
		})
	}
}
