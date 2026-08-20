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

// Stage A 健壮性测试：output_config.effort 为非字符串类型 / output_config 非对象 /
// effort 空串时，入站不 panic、不误注入 ReasoningEffort。
// DTO 化后由 shared.Optional[string] 吞掉类型不匹配、shared.DecodeJSONObject 拒掉
// 非对象形状，语义与 map 时代（maputil.String 返回 ""、asMap 返回 false）等价，
// 此用例固化该不变量，防止后续重构把宽容改严。
func TestTransformToUnified_OutputConfigEffort_Robustness(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantNil bool // true=ReasoningEffort 应为 nil；false=应保持 budget 反推值
	}{
		{
			name:    "effort is number",
			body:    `{"model":"m","max_tokens":1024,"output_config":{"effort":3},"messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
		{
			name:    "effort is array",
			body:    `{"model":"m","max_tokens":1024,"output_config":{"effort":["high"]},"messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
		{
			name:    "effort is object",
			body:    `{"model":"m","max_tokens":1024,"output_config":{"effort":{"a":1}},"messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
		{
			name:    "effort is empty string",
			body:    `{"model":"m","max_tokens":1024,"output_config":{"effort":""},"messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
		{
			name:    "output_config is string not object",
			body:    `{"model":"m","max_tokens":1024,"output_config":"high","messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
		{
			name:    "output_config is array",
			body:    `{"model":"m","max_tokens":1024,"output_config":["high"],"messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := TransformToUnified(context.Background(), []byte(tt.body))
			if err != nil {
				t.Fatalf("TransformToUnified 失败: %v", err)
			}
			if tt.wantNil && unified.ReasoningEffort != nil {
				t.Fatalf("ReasoningEffort 应为 nil，实际 %#v", unified.ReasoningEffort)
			}
		})
	}
}

// Stage 1 的 map → DTO 迁移带来的**唯一**对外行为差异：顶层键匹配走 encoding/json 的
// 「先精确、未命中再 EqualFold」，而 map 时代 `req["max_tokens"]` 是精确查找、
// `{"Max_Tokens":100}` 会静默丢失。
//
// 本用例把这条差异钉成已知契约，避免后人拿「DTO 迁移零行为变更」这句结论去做
// 「anthropic 入站不受键名大小写影响」的推理而踩空。方向上是改善：openai 入站早已是
// DTO（同样不敏感），UnknownTopLevelKeys 的认领判定也做 EqualFold 兜底。
func TestTransformToUnified_TopLevelKeysAreCaseInsensitive(t *testing.T) {
	raw := []byte(`{
		"model":"claude-3-5-sonnet",
		"Max_Tokens":100,
		"Stop_Sequences":["END"],
		"messages":[{"role":"user","content":"hi"}]
	}`)

	unified, err := TransformToUnified(context.Background(), raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}
	if unified.MaxTokens != 100 {
		t.Fatalf("Max_Tokens 应被 EqualFold 命中，实际 MaxTokens=%d", unified.MaxTokens)
	}
	if unified.Stop == nil || len(unified.Stop.Multiple) != 1 || unified.Stop.Multiple[0] != "END" {
		t.Fatalf("Stop_Sequences 应被 EqualFold 命中，实际 %#v", unified.Stop)
	}
}

// Stage 4a 把 system / tools / thinking / output_config / tool_choice 从 map 解析改成
// DTO，**嵌套**键因此也变成大小写不敏感（map 时代 `itemMap["budget_tokens"]` 是精确
// 查找，`Budget_Tokens` 会静默丢失）。
//
// 与顶层那条差异同源、方向一致（见 TestTransformToUnified_TopLevelKeysAreCaseInsensitive），
// 此用例把它钉成已知契约：不是无意后果，改回精确匹配才是回归。
func TestTransformToUnified_NestedKeysAreCaseInsensitive(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet-4-5",
		"max_tokens":1024,
		"System":[{"Type":"text","Text":"be brief"}],
		"thinking":{"Type":"enabled","Budget_Tokens":30000},
		"output_config":{"Effort":"high"},
		"tool_choice":{"Type":"tool","Name":"calc"},
		"tools":[{"Name":"calc","Description":"do math","Input_Schema":{"type":"object"}}],
		"messages":[{"role":"user","content":"hi"}]
	}`)

	unified, err := TransformToUnified(context.Background(), raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}

	if unified.System != "be brief" || len(unified.SystemParts) != 1 {
		t.Fatalf("system 块的 Type/Text 应被 EqualFold 命中，实际 System=%q Parts=%#v", unified.System, unified.SystemParts)
	}
	if unified.ReasoningBudget == nil || *unified.ReasoningBudget != 30000 {
		t.Fatalf("thinking.Budget_Tokens 应被 EqualFold 命中，实际 %#v", unified.ReasoningBudget)
	}
	if unified.ReasoningEffort == nil || *unified.ReasoningEffort != "high" {
		t.Fatalf("output_config.Effort 应被 EqualFold 命中并压过 budget 反推，实际 %#v", unified.ReasoningEffort)
	}
	if unified.ToolChoice == nil || unified.ToolChoice.ObjectValue == nil ||
		unified.ToolChoice.ObjectValue.Function == nil || unified.ToolChoice.ObjectValue.Function.Name != "calc" {
		t.Fatalf("tool_choice 的 Type/Name 应被 EqualFold 命中，实际 %#v", unified.ToolChoice)
	}
	if len(unified.Tools) != 1 || unified.Tools[0].Function.Name != "calc" ||
		unified.Tools[0].Function.Description != "do math" || unified.Tools[0].Function.Parameters == nil {
		t.Fatalf("tools 元素的 Name/Description/Input_Schema 应被 EqualFold 命中，实际 %#v", unified.Tools)
	}
}

// 非 "tool" 的 type（auto / any / none）走 StringValue 透传，而不是被丢弃；
// 解析后 StringValue 与 ObjectValue 皆空时整个 ToolChoice 置 nil，而不是留一个空壳
// ——空壳会让出站适配器 emit 出 `"tool_choice":{}` 这种上游会拒的形状。
func TestTransformToUnified_ToolChoiceBranches(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantString string // 非空 = 期望 StringValue 等于它
		wantFunc   string // 非空 = 期望 ObjectValue.Function.Name 等于它
		wantNil    bool
	}{
		{
			name:       "bare string passes through",
			body:       `{"model":"m","max_tokens":1024,"tool_choice":"auto","messages":[{"role":"user","content":"hi"}]}`,
			wantString: "auto",
		},
		{
			name:       "unknown object type becomes StringValue",
			body:       `{"model":"m","max_tokens":1024,"tool_choice":{"type":"any"},"messages":[{"role":"user","content":"hi"}]}`,
			wantString: "any",
		},
		{
			name:     "type tool with name becomes function object",
			body:     `{"model":"m","max_tokens":1024,"tool_choice":{"type":"tool","name":"calc"},"messages":[{"role":"user","content":"hi"}]}`,
			wantFunc: "calc",
		},
		{
			name:    "type tool without name collapses to nil",
			body:    `{"model":"m","max_tokens":1024,"tool_choice":{"type":"tool"},"messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
		{
			name:    "empty object collapses to nil",
			body:    `{"model":"m","max_tokens":1024,"tool_choice":{},"messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
		{
			name:    "empty string collapses to nil",
			body:    `{"model":"m","max_tokens":1024,"tool_choice":"","messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
		{
			name:    "number is not a valid shape",
			body:    `{"model":"m","max_tokens":1024,"tool_choice":123,"messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
		{
			name:    "array is not a valid shape",
			body:    `{"model":"m","max_tokens":1024,"tool_choice":["auto"],"messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
		{
			name:    "null is treated as unset",
			body:    `{"model":"m","max_tokens":1024,"tool_choice":null,"messages":[{"role":"user","content":"hi"}]}`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := TransformToUnified(context.Background(), []byte(tt.body))
			if err != nil {
				t.Fatalf("TransformToUnified 失败: %v", err)
			}

			if tt.wantNil {
				if unified.ToolChoice != nil {
					t.Fatalf("ToolChoice 应整体为 nil，实际 %#v", unified.ToolChoice)
				}
				return
			}
			if unified.ToolChoice == nil {
				t.Fatalf("ToolChoice 不应为 nil")
			}

			if tt.wantString != "" {
				if unified.ToolChoice.StringValue == nil || *unified.ToolChoice.StringValue != tt.wantString {
					t.Fatalf("StringValue 应为 %q，实际 %#v", tt.wantString, unified.ToolChoice.StringValue)
				}
				if unified.ToolChoice.ObjectValue != nil {
					t.Fatalf("走 StringValue 分支时 ObjectValue 应为 nil，实际 %#v", unified.ToolChoice.ObjectValue)
				}
			}
			if tt.wantFunc != "" {
				obj := unified.ToolChoice.ObjectValue
				if obj == nil || obj.Type != "function" || obj.Function == nil || obj.Function.Name != tt.wantFunc {
					t.Fatalf("ObjectValue 应为 function/%s，实际 %#v", tt.wantFunc, obj)
				}
				if unified.ToolChoice.StringValue != nil {
					t.Fatalf("走 ObjectValue 分支时 StringValue 应为 nil，实际 %#v", unified.ToolChoice.StringValue)
				}
			}
		})
	}
}

// 疑虑 #4 表征测试之一（锁死现状，供 #16 struct 化重构兜底）：
// tool_result 块的 tool_use_id 为空时，parseMessageContentAndToolResults 会「按块」skip 该块，
// 既不产出 tool 消息也不进入 content parts；非空时才产出 tool 消息。
// 两个 case 都带一个 text 块以保证外层 user 消息在两种情况下都存活，
// 从而把「块是否被 skip」与规则 2「user 仅含 tool_result 则整条丢弃」隔离开。
func TestTransformToUnified_ToolResultEmptyToolUseIDSkipsBlock(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		wantMessages   int
		wantToolMsg    bool   // 是否期望存在一条 Role=="tool" 消息
		wantToolCallID string // wantToolMsg 时该 tool 消息的 ToolCallID
	}{
		{
			name: "empty tool_use_id skips the block",
			body: `{
				"model":"m","max_tokens":1024,
				"messages":[
					{"role":"user","content":[
						{"type":"tool_result","tool_use_id":"","content":"ignored"},
						{"type":"text","text":"hi"}
					]}
				]
			}`,
			wantMessages: 1,
			wantToolMsg:  false,
		},
		{
			name: "non-empty tool_use_id produces a tool message",
			body: `{
				"model":"m","max_tokens":1024,
				"messages":[
					{"role":"user","content":[
						{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"},
						{"type":"text","text":"hi"}
					]}
				]
			}`,
			wantMessages:   2,
			wantToolMsg:    true,
			wantToolCallID: "toolu_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := TransformToUnified(context.Background(), []byte(tt.body))
			if err != nil {
				t.Fatalf("TransformToUnified 失败: %v", err)
			}
			if len(unified.Messages) != tt.wantMessages {
				t.Fatalf("消息条数应为 %d，实际 %d: %#v", tt.wantMessages, len(unified.Messages), unified.Messages)
			}

			var toolMsg *models.UnifiedMessage
			var userMsg *models.UnifiedMessage
			for i := range unified.Messages {
				switch unified.Messages[i].Role {
				case "tool":
					toolMsg = &unified.Messages[i]
				case "user":
					userMsg = &unified.Messages[i]
				}
			}

			if tt.wantToolMsg {
				if toolMsg == nil {
					t.Fatalf("应存在一条 tool 消息，实际无: %#v", unified.Messages)
				}
				if toolMsg.ToolCallID != tt.wantToolCallID {
					t.Fatalf("tool 消息 ToolCallID 应为 %q，实际 %q", tt.wantToolCallID, toolMsg.ToolCallID)
				}
			} else if toolMsg != nil {
				t.Fatalf("空 tool_use_id 不应产出 tool 消息，实际 %#v", toolMsg)
			}

			// 两个 case 都含 text 块，user 消息必须存活且 content 收敛为 "hi"。
			if userMsg == nil {
				t.Fatalf("含 text 块的 user 消息应存活，实际无: %#v", unified.Messages)
			}
			if content, ok := userMsg.Content.(string); !ok || content != "hi" {
				t.Fatalf("user 消息 content 应为 \"hi\"，实际 %#v", userMsg.Content)
			}
		})
	}
}

// 疑虑 #4 表征测试之二（锁死现状，供 #16 struct 化重构兜底）：
// parseMessages 丢弃原消息的三个条件必须同时成立——role=="user" 且 content==nil 且产出过 tool 消息。
//
// 注意实际判据是「没有产出任何有效 text/image part 导致 content==nil」，
// **不是**「content 数组里只有 tool_result 类型的块」：空 text 块同样不产出 part，
// 因此 [tool_result, 空 text] 也会触发丢弃。#16 重构时不要把它错读成按块类型判断。
// tool 消息本身在丢弃判断之前已先行 append，被丢的只是原始 user 外壳消息。
func TestTransformToUnified_UserMessageOnlyToolResultDropped(t *testing.T) {
	tests := []struct {
		name            string
		body            string
		wantRoles       []string // 期望的消息角色序列（tool 消息先于原消息 append）
		wantShellString string   // 非空时校验存活的外壳消息 content
	}{
		{
			name: "user with only tool_result drops the shell message",
			body: `{
				"model":"m","max_tokens":1024,
				"messages":[
					{"role":"user","content":[
						{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}
					]}
				]
			}`,
			wantRoles: []string{"tool"},
		},
		{
			name: "user with tool_result plus text keeps the shell message",
			body: `{
				"model":"m","max_tokens":1024,
				"messages":[
					{"role":"user","content":[
						{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"},
						{"type":"text","text":"hi"}
					]}
				]
			}`,
			wantRoles:       []string{"tool", "user"},
			wantShellString: "hi",
		},
		{
			name: "empty text block yields no part so the shell is still dropped",
			body: `{
				"model":"m","max_tokens":1024,
				"messages":[
					{"role":"user","content":[
						{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"},
						{"type":"text","text":""}
					]}
				]
			}`,
			wantRoles: []string{"tool"},
		},
		{
			name: "non-user role is never dropped",
			body: `{
				"model":"m","max_tokens":1024,
				"messages":[
					{"role":"assistant","content":[
						{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}
					]}
				]
			}`,
			wantRoles: []string{"tool", "assistant"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := TransformToUnified(context.Background(), []byte(tt.body))
			if err != nil {
				t.Fatalf("TransformToUnified 失败: %v", err)
			}
			if len(unified.Messages) != len(tt.wantRoles) {
				t.Fatalf("消息条数应为 %d，实际 %d: %#v", len(tt.wantRoles), len(unified.Messages), unified.Messages)
			}
			for i, wantRole := range tt.wantRoles {
				if unified.Messages[i].Role != wantRole {
					t.Fatalf("第 %d 条消息 role 应为 %q，实际 %q", i, wantRole, unified.Messages[i].Role)
				}
			}
			if unified.Messages[0].ToolCallID != "toolu_1" {
				t.Fatalf("tool 消息 ToolCallID 应为 toolu_1，实际 %q", unified.Messages[0].ToolCallID)
			}

			if tt.wantShellString != "" {
				shell := unified.Messages[len(unified.Messages)-1]
				if content, ok := shell.Content.(string); !ok || content != tt.wantShellString {
					t.Fatalf("存活外壳消息 content 应为 %q，实际 %#v", tt.wantShellString, shell.Content)
				}
			}
		})
	}
}

// 疑虑 #4 表征测试之三（锁死现状，供 #16 struct 化重构兜底）：
// 统一模型只有一个 RedactedThinkingData 字段，parseReasoning 遇到多个 redacted_thinking 块时
// 保留**第一个 data 非空**的块——判据是「当前累积值仍为空」而不是「是第一个块」，
// 所以领头的空 data 块会被跳过、继续用后面的块填充，填上之后其余块全部忽略。
// 与 thinking 块的 signature「后者覆盖前者」语义相反，#16 重构时勿混淆两者。
func TestTransformToUnified_MultipleRedactedThinkingKeepsFirstNonEmpty(t *testing.T) {
	raw := []byte(`{
		"model":"m","max_tokens":1024,
		"messages":[
			{"role":"assistant","content":[
				{"type":"redacted_thinking","data":""},
				{"type":"redacted_thinking","data":"D1"},
				{"type":"redacted_thinking","data":"D2"}
			]}
		]
	}`)

	unified, err := TransformToUnified(context.Background(), raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}
	if len(unified.Messages) != 1 {
		t.Fatalf("应产出 1 条 assistant 消息，实际 %d: %#v", len(unified.Messages), unified.Messages)
	}

	msg := unified.Messages[0]
	if msg.Role != "assistant" {
		t.Fatalf("role 应为 assistant，实际 %q", msg.Role)
	}
	if msg.RedactedThinkingData == nil {
		t.Fatalf("RedactedThinkingData 不应为 nil")
	}
	if *msg.RedactedThinkingData != "D1" {
		t.Fatalf("应保留首个非空 data \"D1\"，实际 %q", *msg.RedactedThinkingData)
	}
	// redacted_thinking 块不产出 text/image part，外壳 content 收敛为 nil。
	if msg.Content != nil {
		t.Fatalf("仅含 redacted_thinking 时 content 应为 nil，实际 %#v", msg.Content)
	}
}

// 疑虑 #4 表征测试之四（锁死现状，供 #16 struct 化重构兜底）：
// parseToolCalls 把 arguments 初始化为 "{}"，仅当 input 断言成 JSON object 成功时才覆盖。
// 因此数组 / 标量 / 缺失 / null 形态的 input 全部静默塌成 "{}"——上游拿不到原始 input，
// 这是当前的宽容取舍。#16 用 RawMessage 承接 input 时必须维持同样的塌陷结果。
func TestTransformToUnified_ToolUseNonMapInputKeepsEmptyArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    string // tool_use 块里 input 字段的原文；空串表示不写该键
		wantArgs string
	}{
		{name: "array input collapses to empty object", input: `[1,2]`, wantArgs: "{}"},
		{name: "string input collapses to empty object", input: `"foo"`, wantArgs: "{}"},
		{name: "number input collapses to empty object", input: `5`, wantArgs: "{}"},
		{name: "null input collapses to empty object", input: `null`, wantArgs: "{}"},
		{name: "missing input collapses to empty object", input: ``, wantArgs: "{}"},
		{name: "object input is marshaled through", input: `{"a":1}`, wantArgs: `{"a":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputField := ""
			if tt.input != "" {
				inputField = `,"input":` + tt.input
			}
			body := `{
				"model":"m","max_tokens":1024,
				"messages":[
					{"role":"assistant","content":[
						{"type":"tool_use","id":"toolu_1","name":"fn"` + inputField + `}
					]}
				]
			}`

			unified, err := TransformToUnified(context.Background(), []byte(body))
			if err != nil {
				t.Fatalf("TransformToUnified 失败: %v", err)
			}
			if len(unified.Messages) != 1 {
				t.Fatalf("应产出 1 条 assistant 消息，实际 %d: %#v", len(unified.Messages), unified.Messages)
			}

			toolCalls := unified.Messages[0].ToolCalls
			if len(toolCalls) != 1 {
				t.Fatalf("应产出 1 个 tool_call，实际 %d: %#v", len(toolCalls), toolCalls)
			}
			if toolCalls[0].Function.Arguments != tt.wantArgs {
				t.Fatalf("arguments 应为 %q，实际 %q", tt.wantArgs, toolCalls[0].Function.Arguments)
			}
			if toolCalls[0].ID != "toolu_1" || toolCalls[0].Function.Name != "fn" {
				t.Fatalf("tool_call 元信息错误: %#v", toolCalls[0])
			}
		})
	}
}
