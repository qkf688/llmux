package transform

import (
	"encoding/json"
	"github.com/atopos31/llmio/models"
	"testing"
)

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
		t.Fatal("Expected models.CacheControl to be set")
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
		t.Fatal("Expected models.CacheControl to be set")
	}

	if unified.Tools[0].CacheControl.Type != "ephemeral" {
		t.Errorf("Expected cache_control type 'ephemeral', got '%s'", unified.Tools[0].CacheControl.Type)
	}
}

func TestCacheControl_UnifiedToAnthropic_Message(t *testing.T) {
	// 测试 Unified → Anthropic 消息级别缓存控制转换
	temp := 0.7
	unified := &models.UnifiedRequest{
		Model:       "claude-3-opus",
		MaxTokens:   100,
		Temperature: &temp,
		Messages: []models.UnifiedMessage{
			{
				Role:    "user",
				Content: "Hello",
				CacheControl: &models.CacheControl{
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
	unified := &models.UnifiedRequest{
		Model:       "claude-3-opus",
		MaxTokens:   100,
		Temperature: &temp,
		Messages: []models.UnifiedMessage{
			{Role: "user", Content: "Hello"},
		},
		Tools: []models.UnifiedTool{
			{
				Type: "function",
				Function: models.UnifiedFunc{
					Name:        "get_weather",
					Description: "Get weather info",
					Parameters:  map[string]interface{}{"type": "object"},
				},
				CacheControl: &models.CacheControl{
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
	unified := &models.UnifiedRequest{
		Model:       "claude-3-opus",
		MaxTokens:   100,
		Temperature: &temp,
		Messages: []models.UnifiedMessage{
			{
				Role: "user",
				Content: []models.UnifiedMessageContentPart{
					{
						Type: "text",
						Text: &text,
						CacheControl: &models.CacheControl{
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
	unified := &models.UnifiedRequest{
		Model:       "gpt-4",
		MaxTokens:   100,
		Temperature: &temp,
		Messages: []models.UnifiedMessage{
			{
				Role:    "user",
				Content: "Hello",
				CacheControl: &models.CacheControl{
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
