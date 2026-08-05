package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qkf688/llmux/models"
)

// 本文件是请求侧协议不变量的回归断言，对应 .local/next-do.md 的待办 26 / 27 / 28。

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

// 待办 28：tool_result 的图片块必须保留到统一格式，不能被压成纯文本。
// computer-use / 截图类工具的结果全是图片块，压成文本后 content 变空串，部分上游判 400。
func TestTransformToUnified_ToolResultKeepsImageBlocks(t *testing.T) {
	raw := []byte(`{
		"model":"claude-3-5-sonnet",
		"max_tokens":1024,
		"messages":[
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":[
					{"type":"text","text":"截图如下"},
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAAA"}}
				]}
			]}
		]
	}`)

	unified, err := TransformToUnified(context.Background(), raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}
	if len(unified.Messages) != 1 {
		t.Fatalf("应产出 1 条 tool 消息，实际 %d", len(unified.Messages))
	}

	toolMsg := unified.Messages[0]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "toolu_1" {
		t.Fatalf("tool 消息元信息错误: %#v", toolMsg)
	}

	parts, ok := toolMsg.Content.([]models.UnifiedMessageContentPart)
	if !ok {
		t.Fatalf("含图片的 tool_result 应保留为块数组，实际 %#v", toolMsg.Content)
	}
	if len(parts) != 2 || parts[0].Type != "text" || parts[1].Type != "image_url" {
		t.Fatalf("块类型应为 [text image_url]，实际 %#v", parts)
	}
	if parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/png;base64,AAAA" {
		t.Fatalf("图片未还原为 data URL: %#v", parts[1].ImageURL)
	}
}

// 待办 28 的形态守护：全 text 的 tool_result 仍降级为 string，不引入无谓的形态变化。
func TestTransformToUnified_TextOnlyToolResultStaysString(t *testing.T) {
	raw := []byte(`{
		"model":"claude-3-5-sonnet",
		"max_tokens":1024,
		"messages":[
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":[
					{"type":"text","text":"42"}
				]}
			]}
		]
	}`)

	unified, err := TransformToUnified(context.Background(), raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}
	if len(unified.Messages) != 1 {
		t.Fatalf("应产出 1 条 tool 消息，实际 %d", len(unified.Messages))
	}
	if content, ok := unified.Messages[0].Content.(string); !ok || content != "42" {
		t.Fatalf("纯文本 tool_result 应保持 string，实际 %#v", unified.Messages[0].Content)
	}
}

// 待办 28：Anthropic 原生支持块数组形式的 tool_result，出站不得把图片丢掉。
func TestTransformFromUnified_ToolResultKeepsImageBlocks(t *testing.T) {
	text := "截图如下"
	unified := &models.UnifiedRequest{
		Model: "claude-3-5-sonnet",
		Messages: []models.UnifiedMessage{
			{
				Role:       "tool",
				ToolCallID: "toolu_1",
				Content: []models.UnifiedMessageContentPart{
					{Type: "text", Text: &text},
					{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAAA"}},
				},
			},
		},
	}

	messages := decodeAnthropicMessages(t, unified)
	if len(messages) != 1 {
		t.Fatalf("应产出 1 条消息，实际 %d", len(messages))
	}

	blocks, ok := messages[0]["content"].([]interface{})
	if !ok || len(blocks) != 1 {
		t.Fatalf("content 应是单个 tool_result 块，实际 %#v", messages[0]["content"])
	}
	toolResult, _ := blocks[0].(map[string]interface{})
	if toolResult["type"] != "tool_result" || toolResult["tool_use_id"] != "toolu_1" {
		t.Fatalf("tool_result 元信息错误: %#v", toolResult)
	}

	inner, ok := toolResult["content"].([]interface{})
	if !ok {
		t.Fatalf("含图片的 tool_result content 应是块数组，实际 %#v", toolResult["content"])
	}
	if len(inner) != 2 {
		t.Fatalf("tool_result 应含 2 个块，实际 %d", len(inner))
	}
	first, _ := inner[0].(map[string]interface{})
	second, _ := inner[1].(map[string]interface{})
	if first["type"] != "text" || first["text"] != "截图如下" {
		t.Fatalf("首块应为原文本，实际 %#v", first)
	}
	if second["type"] != "image" {
		t.Fatalf("次块应为 image，实际 %#v", second)
	}
	source, _ := second["source"].(map[string]interface{})
	if source["type"] != "base64" || source["media_type"] != "image/png" || source["data"] != "AAAA" {
		t.Fatalf("图片 source 还原错误: %#v", source)
	}
}

// 待办 28 的形态守护：string content 的 tool 消息仍输出 string，不变成块数组。
func TestTransformFromUnified_TextToolResultStaysString(t *testing.T) {
	unified := &models.UnifiedRequest{
		Model: "claude-3-5-sonnet",
		Messages: []models.UnifiedMessage{
			{Role: "tool", ToolCallID: "toolu_1", Content: "42"},
		},
	}

	messages := decodeAnthropicMessages(t, unified)
	blocks := messages[0]["content"].([]interface{})
	toolResult := blocks[0].(map[string]interface{})
	if content, ok := toolResult["content"].(string); !ok || content != "42" {
		t.Fatalf("纯文本 tool_result 应保持 string，实际 %#v", toolResult["content"])
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

	unified, err := TransformToUnified(context.Background(), raw)
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

// 待办 44：redacted_thinking.data 是不透明密文，必须与 thinking 文本分开保存并原样回灌。
func TestTransform_RedactedThinkingBlockSurvivesRoundTrip(t *testing.T) {
	const opaqueData = "EmwKAhgBEgyu3v+0/opaque=="
	raw := []byte(`{
		"model":"claude-3-5-sonnet",
		"max_tokens":1024,
		"thinking":{"type":"enabled","budget_tokens":20000},
		"messages":[
			{"role":"user","content":"北京天气如何"},
			{"role":"assistant","content":[
				{"type":"redacted_thinking","data":"` + opaqueData + `"},
				{"type":"text","text":"我查一下。"},
				{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"location":"Beijing"}}
			]}
		]
	}`)

	unified, err := TransformToUnified(context.Background(), raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}
	if len(unified.Messages) != 2 {
		t.Fatalf("应解析出 2 条消息，实际 %d", len(unified.Messages))
	}

	encoded, err := json.Marshal(unified.Messages[1])
	if err != nil {
		t.Fatalf("序列化统一消息失败: %v", err)
	}
	var messageFields map[string]interface{}
	if err := json.Unmarshal(encoded, &messageFields); err != nil {
		t.Fatalf("解析统一消息失败: %v", err)
	}
	if messageFields["redacted_thinking_data"] != opaqueData {
		t.Fatalf("统一消息未独立保留 redacted_thinking.data: %#v", messageFields)
	}

	messages := decodeAnthropicMessages(t, unified)
	got := contentBlockTypes(t, messages[1])
	want := []string{"redacted_thinking", "text", "tool_use"}
	if len(got) != len(want) {
		t.Fatalf("assistant content 块类型应为 %v，实际 %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("assistant content 块类型应为 %v，实际 %v", want, got)
		}
	}

	blocks := messages[1]["content"].([]interface{})
	redacted := blocks[0].(map[string]interface{})
	if redacted["data"] != opaqueData {
		t.Fatalf("redacted_thinking.data 被改写: %#v", redacted["data"])
	}
}

// Stage A：output_config.effort 是 Claude 4.6 adaptive thinking 的显式档位字段，
// 入站必须接住并映射到 UnifiedRequest.ReasoningEffort，不能丢失。
// 三场景：effort+budget 共存（effort 优先）、仅 budget（反推）、仅 effort。

func TestTransformToUnified_OutputConfigEffort_WithBudget_EffortWins(t *testing.T) {
	// output_config.effort=high 与 thinking.budget_tokens=5000 共存：
	// effort 取 output_config.effort（显式意图优先），budget 仍取 budget_tokens。
	raw := []byte(`{
		"model":"claude-sonnet-4-5",
		"max_tokens":1024,
		"thinking":{"type":"enabled","budget_tokens":5000},
		"output_config":{"effort":"high"},
		"messages":[{"role":"user","content":"hi"}]
	}`)

	unified, err := TransformToUnified(context.Background(), raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}
	if unified.ReasoningEffort == nil || *unified.ReasoningEffort != "high" {
		t.Fatalf("effort 应取 output_config.effort=high（显式意图优先于 budget 反推），实际 %#v", unified.ReasoningEffort)
	}
	if unified.ReasoningBudget == nil || *unified.ReasoningBudget != 5000 {
		t.Fatalf("budget 应仍取 budget_tokens=5000，实际 %#v", unified.ReasoningBudget)
	}
}

func TestTransformToUnified_OutputConfigEffort_OnlyBudget_KeepsInference(t *testing.T) {
	// 仅 thinking.budget_tokens 无 output_config.effort：保持现状用 ThinkingBudgetToReasoningEffort 反推。
	raw := []byte(`{
		"model":"claude-sonnet-4-5",
		"max_tokens":1024,
		"thinking":{"type":"enabled","budget_tokens":30000},
		"messages":[{"role":"user","content":"hi"}]
	}`)

	unified, err := TransformToUnified(context.Background(), raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}
	if unified.ReasoningEffort == nil || *unified.ReasoningEffort != "medium" {
		t.Fatalf("仅 budget 时应反推 medium，实际 %#v", unified.ReasoningEffort)
	}
	if unified.ReasoningBudget == nil || *unified.ReasoningBudget != 30000 {
		t.Fatalf("budget 应为 30000，实际 %#v", unified.ReasoningBudget)
	}
}

func TestTransformToUnified_OutputConfigEffort_OnlyEffort_NoBudget(t *testing.T) {
	// 仅 output_config.effort 无 thinking.budget_tokens：effort 注入，budget 不设。
	raw := []byte(`{
		"model":"claude-sonnet-4-5",
		"max_tokens":1024,
		"output_config":{"effort":"high"},
		"messages":[{"role":"user","content":"hi"}]
	}`)

	unified, err := TransformToUnified(context.Background(), raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}
	if unified.ReasoningEffort == nil || *unified.ReasoningEffort != "high" {
		t.Fatalf("effort 应取 output_config.effort=high，实际 %#v", unified.ReasoningEffort)
	}
	if unified.ReasoningBudget != nil {
		t.Fatalf("无 budget_tokens 时不应设 ReasoningBudget，实际 %#v", unified.ReasoningBudget)
	}
}

