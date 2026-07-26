package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/atopos31/llmio/models"
)

// 本文件是请求侧协议不变量的回归断言，对应 .local/next-do.md 的待办 26 / 27。

// decodeAnthropicMessages 把 TransformFromUnified 的输出解回消息数组。
func decodeAnthropicMessages(t *testing.T, unified *models.UnifiedRequest) []map[string]interface{} {
	t.Helper()

	raw, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified 失败: %v", err)
	}

	var req struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("解析输出失败: %v (%s)", err, raw)
	}
	return req.Messages
}

func contentBlockTypes(t *testing.T, msg map[string]interface{}) []string {
	t.Helper()

	blocks, ok := msg["content"].([]interface{})
	if !ok {
		t.Fatalf("content 不是块数组: %#v", msg["content"])
	}

	types := make([]string, 0, len(blocks))
	for _, b := range blocks {
		m, ok := b.(map[string]interface{})
		if !ok {
			t.Fatalf("content 块不是对象: %#v", b)
		}
		typ, _ := m["type"].(string)
		types = append(types, typ)
	}
	return types
}

// 待办 26：assistant 轮同时有多模态内容与 tool_calls 时，文本与图片不得被 tool_use 覆盖。
func TestTransformFromUnified_ToolCallsKeepMultimodalContent(t *testing.T) {
	text := "看看这张图的天气"
	unified := &models.UnifiedRequest{
		Model: "claude-3-5-sonnet",
		Messages: []models.UnifiedMessage{
			{
				Role: "assistant",
				Content: []models.UnifiedMessageContentPart{
					{Type: "text", Text: &text},
					{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAAA"}},
				},
				ToolCalls: []models.UnifiedToolCall{
					{
						ID:       "toolu_1",
						Type:     "function",
						Function: models.UnifiedToolCallFunction{Name: "get_weather", Arguments: `{"location":"Beijing"}`},
					},
				},
			},
		},
	}

	messages := decodeAnthropicMessages(t, unified)
	if len(messages) != 1 {
		t.Fatalf("应产出 1 条消息，实际 %d", len(messages))
	}

	got := contentBlockTypes(t, messages[0])
	want := []string{"text", "image", "tool_use"}
	if len(got) != len(want) {
		t.Fatalf("content 块类型应为 %v，实际 %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("content 块类型应为 %v，实际 %v", want, got)
		}
	}
}

// 待办 26 的既有行为守护：只有 string content + tool_calls 时仍是 text + tool_use。
func TestTransformFromUnified_ToolCallsKeepStringContent(t *testing.T) {
	unified := &models.UnifiedRequest{
		Model: "claude-3-5-sonnet",
		Messages: []models.UnifiedMessage{
			{
				Role:    "assistant",
				Content: "让我查一下",
				ToolCalls: []models.UnifiedToolCall{
					{ID: "toolu_1", Type: "function", Function: models.UnifiedToolCallFunction{Name: "get_weather"}},
				},
			},
		},
	}

	messages := decodeAnthropicMessages(t, unified)
	got := contentBlockTypes(t, messages[0])
	if len(got) != 2 || got[0] != "text" || got[1] != "tool_use" {
		t.Fatalf("content 块类型应为 [text tool_use]，实际 %v", got)
	}
}

// 待办 27：入站必须保留 thinking 块与 signature，出站必须原样带回。
// 丢掉的话，thinking 参数仍会透传给上游，多轮会话必被 400 拒绝。
func TestTransform_ThinkingBlockSurvivesRoundTrip(t *testing.T) {
	raw := []byte(`{
		"model":"claude-3-5-sonnet",
		"max_tokens":1024,
		"thinking":{"type":"enabled","budget_tokens":20000},
		"messages":[
			{"role":"user","content":"北京天气如何"},
			{"role":"assistant","content":[
				{"type":"thinking","thinking":"用户想查天气，该调用工具。","signature":"sig_abc"},
				{"type":"text","text":"我查一下。"},
				{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"location":"Beijing"}}
			]}
		]
	}`)

	unified, err := TransformToUnified(raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}

	if len(unified.Messages) != 2 {
		t.Fatalf("应解析出 2 条消息，实际 %d", len(unified.Messages))
	}
	assistant := unified.Messages[1]
	if assistant.ReasoningContent == nil || *assistant.ReasoningContent != "用户想查天气，该调用工具。" {
		t.Fatalf("thinking 文本未保留: %#v", assistant.ReasoningContent)
	}
	if assistant.ReasoningSignature == nil || *assistant.ReasoningSignature != "sig_abc" {
		t.Fatalf("thinking signature 未保留: %#v", assistant.ReasoningSignature)
	}

	messages := decodeAnthropicMessages(t, unified)
	if len(messages) != 2 {
		t.Fatalf("应还原出 2 条消息，实际 %d", len(messages))
	}

	got := contentBlockTypes(t, messages[1])
	want := []string{"thinking", "text", "tool_use"}
	if len(got) != len(want) {
		t.Fatalf("assistant content 块类型应为 %v，实际 %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("assistant content 块类型应为 %v，实际 %v", want, got)
		}
	}

	blocks := messages[1]["content"].([]interface{})
	thinking := blocks[0].(map[string]interface{})
	if thinking["thinking"] != "用户想查天气，该调用工具。" {
		t.Errorf("thinking 文本被改写: %#v", thinking["thinking"])
	}
	if thinking["signature"] != "sig_abc" {
		t.Errorf("thinking signature 丢失: %#v", thinking["signature"])
	}
}
