package responses

import (
	"testing"

	"github.com/atopos31/llmio/models"
)

// 本文件对应 .local/next-do.md 的待办 46。
//
// Responses 的 function_call_output.output 支持 string（纯文本，老上游兼容）
// 或 input_text/input_image 数组（多模态，OpenAI Responses 协议规范）。
// 关键不变量：「不得整条丢弃」「纯文本仍输出 string（老上游兼容）」「含图片输出数组」。

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

// 含图片的多模态 tool 结果应输出 input_text/input_image 数组形式，不再降级为文本+占位符。
func TestConvertMessagesToInput_MultimodalToolResultOutputArray(t *testing.T) {
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
	arr, ok := outputs[0].Output.([]map[string]interface{})
	if !ok {
		t.Fatalf("含图片的 tool 结果应输出数组形式，实际 %#v", outputs[0].Output)
	}
	if len(arr) != 2 {
		t.Fatalf("应产出 2 个内容块（text + image），实际 %d", len(arr))
	}
	if arr[0]["type"] != "input_text" || arr[0]["text"] != "截图如下" {
		t.Fatalf("第一个块应为 input_text，实际 %#v", arr[0])
	}
	if arr[1]["type"] != "input_image" || arr[1]["image_url"] != "data:image/png;base64,AAAA" {
		t.Fatalf("第二个块应为 input_image 且保留原 URL，实际 %#v", arr[1])
	}
}

// 纯截图的 tool_result 应输出仅含 input_image 的数组，不得降级为空串。
func TestConvertMessagesToInput_ImageOnlyToolResultOutputArray(t *testing.T) {
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
	arr, ok := outputs[0].Output.([]map[string]interface{})
	if !ok {
		t.Fatalf("纯图片 tool 结果应输出数组形式，实际 %#v", outputs[0].Output)
	}
	if len(arr) != 1 || arr[0]["type"] != "input_image" {
		t.Fatalf("应产出 1 个 input_image 块，实际 %#v", arr)
	}
}

// 形态守护：string content 的 tool 消息仍输出 string（老上游兼容）。
func TestConvertMessagesToInput_StringToolResultUnchanged(t *testing.T) {
	outputs := toolOutputs(t, []models.UnifiedMessage{
		{Role: "tool", ToolCallID: "call_1", Content: `{"ok":true}`},
	})

	if len(outputs) != 1 {
		t.Fatalf("应产出 1 条 function_call_output，实际 %d", len(outputs))
	}
	s, ok := outputs[0].Output.(string)
	if !ok || s != `{"ok":true}` {
		t.Fatalf("string tool 结果应仍为 string，实际 %#v", outputs[0].Output)
	}
}
