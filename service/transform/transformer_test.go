package transform

import (
	"context"
	"encoding/json"
	"github.com/atopos31/llmio/models"
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

	result, err := tm.ProcessRequest(context.Background(), openaiRequest)
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
	for _, name := range []string{"openai", "openai-res", "anthropic"} {
		if _, ok := formatAdapters[name]; !ok {
			t.Fatalf("expected %q adapter to be registered", name)
		}
	}
}

func TestTransformerManager_UnknownTypesFallBackToOpenAI(t *testing.T) {
	openaiRequest := []byte(`{
		"model": "gpt-4",
		"messages": [{"role": "user", "content": "Hello"}],
		"max_tokens": 100
	}`)

	unknownClient := NewTransformerManager("unknown-client", "anthropic")
	unknownClientResult, err := unknownClient.ProcessRequest(context.Background(), openaiRequest)
	if err != nil {
		t.Fatalf("ProcessRequest with unknown client type failed: %v", err)
	}

	openAIClient := NewTransformerManager("openai", "anthropic")
	openAIClientResult, err := openAIClient.ProcessRequest(context.Background(), openaiRequest)
	if err != nil {
		t.Fatalf("ProcessRequest with openai client type failed: %v", err)
	}

	assertJSONEqual(t, unknownClientResult, openAIClientResult)

	unknownProvider := NewTransformerManager("openai", "unknown-provider")
	unknownProviderResult, err := unknownProvider.ProcessRequest(context.Background(), openaiRequest)
	if err != nil {
		t.Fatalf("ProcessRequest with unknown provider type failed: %v", err)
	}

	openAIProvider := NewTransformerManager("openai", "openai")
	openAIProviderResult, err := openAIProvider.ProcessRequest(context.Background(), openaiRequest)
	if err != nil {
		t.Fatalf("ProcessRequest with openai provider type failed: %v", err)
	}

	assertJSONEqual(t, unknownProviderResult, openAIProviderResult)
}

func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()

	var gotJSON interface{}
	if err := json.Unmarshal(got, &gotJSON); err != nil {
		t.Fatalf("unmarshal got JSON: %v", err)
	}
	var wantJSON interface{}
	if err := json.Unmarshal(want, &wantJSON); err != nil {
		t.Fatalf("unmarshal want JSON: %v", err)
	}
	if !jsonEqual(gotJSON, wantJSON) {
		t.Fatalf("JSON mismatch\ngot:  %s\nwant: %s", got, want)
	}
}

func jsonEqual(a, b interface{}) bool {
	aBytes, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bBytes, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aBytes) == string(bBytes)
}
