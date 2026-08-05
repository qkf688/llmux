package transform

import (
	"encoding/json"
	"github.com/qkf688/llmux/models"
	"testing"
)

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

func TestThinking_UnifiedToAnthropic_WithBudget(t *testing.T) {
	// 测试 Unified → Anthropic 转换 (使用 ReasoningBudget)
	temp := 0.7
	budget := int64(30000)
	unified := &models.UnifiedRequest{
		Model:           "claude-3-opus",
		MaxTokens:       100,
		Temperature:     &temp,
		ReasoningBudget: &budget,
		Messages: []models.UnifiedMessage{
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
	unified := &models.UnifiedRequest{
		Model:           "claude-3-opus",
		MaxTokens:       100,
		Temperature:     &temp,
		ReasoningEffort: &effort,
		Messages: []models.UnifiedMessage{
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
	unified := &models.UnifiedRequest{
		Model:           "claude-3-opus",
		MaxTokens:       100,
		Temperature:     &temp,
		ReasoningEffort: &effort,
		ReasoningBudget: &budget,
		Messages: []models.UnifiedMessage{
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
	unified := &models.UnifiedRequest{
		Model:           "gpt-4",
		MaxTokens:       100,
		Temperature:     &temp,
		ReasoningBudget: &budget,
		Messages: []models.UnifiedMessage{
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
