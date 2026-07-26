package responses

import (
	"testing"

	"github.com/atopos31/llmio/models"
)

// 本文件对应 .local/next-do.md 的待办 28。
//
// Responses 的 function_call_output.output 只接受字符串，故多模态 tool 结果在此降级为
// 纯文本 + 占位符；关键不变量是「不得整条丢弃」「不得降级成空串」。

func toolOutputs(t *testing.T, messages []models.UnifiedMessage) []ResponsesItem {
	t.Helper()

	input := convertMessagesToInput(messages, nil)
	outputs := make([]ResponsesItem, 0, len(input.Items))
	for _, item := range input.Items {
		if item.Type == "function_call_output" {
			outputs = append(outputs, item)
		}
	}
	return outputs
}

// 多模态 tool 消息此前被 `msg.Content.(string)` 守卫整条丢弃，
// 上游收到的对话里凭空少一轮工具结果。
func TestConvertMessagesToInput_MultimodalToolResultNotDropped(t *testing.T) {
	text := "截图如下"
	outputs := toolOutputs(t, []models.UnifiedMessage{
		{
			Role:       "tool",
			ToolCallID: "call_1",
			Content: []models.UnifiedMessageContentPart{
				{Type: "text", Text: &text},
				{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAAA"}},
			},
		},
	})

	if len(outputs) != 1 {
		t.Fatalf("多模态 tool 消息不得被丢弃，实际产出 %d 条 function_call_output", len(outputs))
	}
	if outputs[0].CallID == nil || *outputs[0].CallID != "call_1" {
		t.Fatalf("call_id 丢失: %#v", outputs[0].CallID)
	}
	if outputs[0].Output == nil || *outputs[0].Output != "截图如下[image]" {
		t.Fatalf("output 应为文本与占位符混排，实际 %#v", outputs[0].Output)
	}
}

// 纯截图的 tool_result 降级后不能是空串——空 output 会被部分上游判 400。
func TestConvertMessagesToInput_ImageOnlyToolResultNotEmpty(t *testing.T) {
	outputs := toolOutputs(t, []models.UnifiedMessage{
		{
			Role:       "tool",
			ToolCallID: "call_1",
			Content: []models.UnifiedMessageContentPart{
				{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAAA"}},
			},
		},
	})

	if len(outputs) != 1 {
		t.Fatalf("应产出 1 条 function_call_output，实际 %d", len(outputs))
	}
	if outputs[0].Output == nil || *outputs[0].Output != "[image]" {
		t.Fatalf("纯图片 tool 结果不得降级为空串，实际 %#v", outputs[0].Output)
	}
}

// 形态守护：string content 的 tool 消息行为不变。
func TestConvertMessagesToInput_StringToolResultUnchanged(t *testing.T) {
	outputs := toolOutputs(t, []models.UnifiedMessage{
		{Role: "tool", ToolCallID: "call_1", Content: `{"ok":true}`},
	})

	if len(outputs) != 1 {
		t.Fatalf("应产出 1 条 function_call_output，实际 %d", len(outputs))
	}
	if outputs[0].Output == nil || *outputs[0].Output != `{"ok":true}` {
		t.Fatalf("string tool 结果被改写: %#v", outputs[0].Output)
	}
}
