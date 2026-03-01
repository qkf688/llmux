package service

import (
	"context"
	"encoding/json"
	"testing"
)

// TestTransformResponsesToUnified_EmptyInput 测试空 input 的情况
func TestTransformResponsesToUnified_EmptyInput(t *testing.T) {
	// 测试 1: input 为 null
	requestBody1 := `{
		"model": "gpt-4",
		"input": null
	}`

	unified1, err := TransformResponsesToUnified(context.Background(), []byte(requestBody1))
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	if len(unified1.Messages) != 0 {
		t.Errorf("input 为 null 时应返回空 messages，实际: %d 条", len(unified1.Messages))
	}

	// 测试 2: input 为空字符串
	requestBody2 := `{
		"model": "gpt-4",
		"input": ""
	}`

	unified2, err := TransformResponsesToUnified(context.Background(), []byte(requestBody2))
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	// 空字符串应该转换为一条空的 user 消息
	if len(unified2.Messages) != 1 {
		t.Errorf("input 为空字符串时应返回 1 条 message，实际: %d 条", len(unified2.Messages))
	}

	// 测试 3: input 为空数组
	requestBody3 := `{
		"model": "gpt-4",
		"input": []
	}`

	unified3, err := TransformResponsesToUnified(context.Background(), []byte(requestBody3))
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	if len(unified3.Messages) != 0 {
		t.Errorf("input 为空数组时应返回空 messages，实际: %d 条", len(unified3.Messages))
	}
}

// TestTransformResponsesToUnified_ValidInput 测试正常 input 的情况
func TestTransformResponsesToUnified_ValidInput(t *testing.T) {
	// 测试 1: 简单字符串 input
	requestBody1 := `{
		"model": "gpt-4",
		"input": "Hello world"
	}`

	unified1, err := TransformResponsesToUnified(context.Background(), []byte(requestBody1))
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	if len(unified1.Messages) != 1 {
		t.Fatalf("应返回 1 条 message，实际: %d 条", len(unified1.Messages))
	}

	if unified1.Messages[0].Role != "user" {
		t.Errorf("role 应为 user，实际: %s", unified1.Messages[0].Role)
	}

	if content, ok := unified1.Messages[0].Content.(string); !ok || content != "Hello world" {
		t.Errorf("content 应为 'Hello world'，实际: %v", unified1.Messages[0].Content)
	}

	// 测试 2: 数组 input
	requestBody2 := `{
		"model": "gpt-4",
		"input": [
			{"type": "input_text", "text": "What is 2+2?"},
			{"type": "output_text", "text": "2+2 equals 4."},
			{"type": "input_text", "text": "Thanks!"}
		]
	}`

	unified2, err := TransformResponsesToUnified(context.Background(), []byte(requestBody2))
	if err != nil {
		t.Fatalf("转换失败: %v", err)
	}

	if len(unified2.Messages) != 3 {
		t.Fatalf("应返回 3 条 messages，实际: %d 条", len(unified2.Messages))
	}

	// 验证第一条消息
	if unified2.Messages[0].Role != "user" {
		t.Errorf("第一条消息 role 应为 user，实际: %s", unified2.Messages[0].Role)
	}

	// 验证第二条消息
	if unified2.Messages[1].Role != "assistant" {
		t.Errorf("第二条消息 role 应为 assistant，实际: %s", unified2.Messages[1].Role)
	}

	// 验证第三条消息
	if unified2.Messages[2].Role != "user" {
		t.Errorf("第三条消息 role 应为 user，实际: %s", unified2.Messages[2].Role)
	}
}

// TestTransformUnifiedToOpenAI_EmptyMessages 测试 messages 为空的情况
func TestTransformUnifiedToOpenAI_EmptyMessages(t *testing.T) {
	unified := &UnifiedRequest{
		Model:    "gpt-4",
		Messages: []UnifiedMessage{}, // 空数组
		Stream:   false,
	}

	_, err := TransformUnifiedToOpenAI(unified)
	if err == nil {
		t.Fatal("messages 为空时应返回错误")
	}

	expectedError := "messages cannot be empty"
	if !contains(err.Error(), expectedError) {
		t.Errorf("错误消息应包含 '%s'，实际: %s", expectedError, err.Error())
	}
}

// TestResponsesToOpenAI_Integration 集成测试：Responses → Unified → OpenAI
func TestResponsesToOpenAI_Integration(t *testing.T) {
	// 正常情况
	responsesRequest := `{
		"model": "gpt-4",
		"input": "Hello",
		"stream": false
	}`

	unified, err := TransformResponsesToUnified(context.Background(), []byte(responsesRequest))
	if err != nil {
		t.Fatalf("Responses → Unified 转换失败: %v", err)
	}

	openaiBytes, err := TransformUnifiedToOpenAI(unified)
	if err != nil {
		t.Fatalf("Unified → OpenAI 转换失败: %v", err)
	}

	var openaiReq map[string]interface{}
	if err := json.Unmarshal(openaiBytes, &openaiReq); err != nil {
		t.Fatalf("解析 OpenAI 请求失败: %v", err)
	}

	// 验证 messages 字段存在且非空
	messages, ok := openaiReq["messages"].([]interface{})
	if !ok {
		t.Fatal("OpenAI 请求缺少 messages 字段")
	}

	if len(messages) == 0 {
		t.Fatal("OpenAI 请求的 messages 为空")
	}

	// 验证第一条消息
	firstMsg := messages[0].(map[string]interface{})
	if firstMsg["role"] != "user" {
		t.Errorf("第一条消息 role 应为 user，实际: %v", firstMsg["role"])
	}
	if firstMsg["content"] != "Hello" {
		t.Errorf("第一条消息 content 应为 'Hello'，实际: %v", firstMsg["content"])
	}
}

// TestResponsesToOpenAI_EmptyInput 测试空 input 的集成转换
func TestResponsesToOpenAI_EmptyInput(t *testing.T) {
	responsesRequest := `{
		"model": "gpt-4",
		"input": null
	}`

	unified, err := TransformResponsesToUnified(context.Background(), []byte(responsesRequest))
	if err != nil {
		t.Fatalf("Responses → Unified 转换失败: %v", err)
	}

	_, err = TransformUnifiedToOpenAI(unified)
	if err == nil {
		t.Fatal("空 input 转换为 OpenAI 格式时应返回错误")
	}

	expectedError := "messages cannot be empty"
	if !contains(err.Error(), expectedError) {
		t.Errorf("错误消息应包含 '%s'，实际: %s", expectedError, err.Error())
	}
}

// TestResponsesToOpenAI_CherryStudioFormat 测试 Cherry Studio 格式的兼容性
func TestResponsesToOpenAI_CherryStudioFormat(t *testing.T) {
	// Cherry Studio 发送的格式：role-based input with content array
	cherryRequest := `{
		"model": "qwen3-next",
		"input": [
			{
				"role": "developer",
				"content": "你是一个助手"
			},
			{
				"role": "user",
				"content": [
					{
						"type": "input_text",
						"text": "你是？"
					}
				]
			}
		],
		"store": false,
		"reasoning": {
			"effort": "low"
		}
	}`

	unified, err := TransformResponsesToUnified(context.Background(), []byte(cherryRequest))
	if err != nil {
		t.Fatalf("Cherry Studio 格式转换失败: %v", err)
	}

	// developer 角色应该被转换为 system
	if unified.System == "" {
		// 或者作为 messages 的一部分
		t.Logf("System: %s", unified.System)
	}

	// 应该有消息
	if len(unified.Messages) == 0 {
		t.Fatal("转换后 messages 为空")
	}

	t.Logf("Messages count: %d", len(unified.Messages))
	for i, msg := range unified.Messages {
		t.Logf("Message %d: role=%s, content=%v", i, msg.Role, msg.Content)
	}

	// 转换为 OpenAI 格式
	openaiBytes, err := TransformUnifiedToOpenAI(unified)
	if err != nil {
		t.Fatalf("Unified → OpenAI 转换失败: %v", err)
	}

	var openaiReq map[string]interface{}
	if err := json.Unmarshal(openaiBytes, &openaiReq); err != nil {
		t.Fatalf("解析 OpenAI 请求失败: %v", err)
	}

	messages, ok := openaiReq["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		t.Fatal("OpenAI 请求的 messages 为空")
	}

	t.Logf("OpenAI messages count: %d", len(messages))
}

// TestResponsesResponse_OutputTextAnnotationsAlwaysArray 测试 output_text 的 annotations 字段始终为数组
func TestResponsesResponse_OutputTextAnnotationsAlwaysArray(t *testing.T) {
	text := "hello"
	status := "completed"

	resp := ResponsesResponse{
		Object:    "response",
		ID:        "resp_123",
		Model:     "test-model",
		CreatedAt: 1234567890,
		Output: []ResponsesItem{
			{
				ID:     "msg_123",
				Type:   "message",
				Role:   "assistant",
				Status: &status,
				Content: &ResponsesInput{Items: []ResponsesItem{
					{
						Type: "output_text",
						Text: &text,
					},
				}},
			},
		},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal 失败: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}

	output, ok := m["output"].([]interface{})
	if !ok || len(output) == 0 {
		t.Fatalf("output 缺失或为空: %#v", m["output"])
	}

	output0, ok := output[0].(map[string]interface{})
	if !ok {
		t.Fatalf("output[0] 类型错误: %#v", output[0])
	}

	content, ok := output0["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("output[0].content 缺失或为空: %#v", output0["content"])
	}

	content0, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("output[0].content[0] 类型错误: %#v", content[0])
	}

	annotationsValue, exists := content0["annotations"]
	if !exists {
		t.Fatalf("output[0].content[0].annotations 必须存在: %#v", content0)
	}

	annotations, ok := annotationsValue.([]interface{})
	if !ok {
		t.Fatalf("output[0].content[0].annotations 必须是 array，实际: %#v", annotationsValue)
	}

	_ = annotations // 允许长度为 0，只验证字段存在且类型为数组
}

// 辅助函数：检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
