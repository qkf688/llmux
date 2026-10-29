package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/qkf688/llmux/models"
)

// Anthropic 契约要求 tool_use.input 是**对象**。空参数工具（OpenAI 兼容上游常给
// `arguments: ""`）、非法 JSON、字面量 `null` 都必须落成空对象——此前这类情况会
// 向客户端发出 `"input": null`，严格客户端判契约违规。与流式侧
// openAnthropicBlockForItem 补的 `input: {}` 是同一条约束的两个出口。
func TestFormatResponse_ToolUseInputIsAlwaysObject(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantKeys int
	}{
		{name: "空 arguments", args: "", wantKeys: 0},
		{name: "非法 JSON", args: "{not-json", wantKeys: 0},
		{name: "字面量 null", args: "null", wantKeys: 0},
		{name: "正常对象", args: `{"city":"SF"}`, wantKeys: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := FormatResponse(&models.UnifiedResponse{
				ID:    "msg_tool",
				Model: "claude-x",
				Choices: []models.UnifiedChoice{{
					Message: &models.UnifiedMessage{
						Role: "assistant",
						ToolCalls: []models.UnifiedToolCall{{
							ID:       "call_1",
							Function: models.UnifiedToolCallFunction{Name: "get_weather", Arguments: tt.args},
						}},
					},
					FinishReason: "tool_calls",
				}},
			})
			if err != nil {
				t.Fatalf("FormatResponse: %v", err)
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			content, ok := resp["content"].([]interface{})
			if !ok || len(content) != 1 {
				t.Fatalf("content 结构不符: %v", resp["content"])
			}
			block, ok := content[0].(map[string]interface{})
			if !ok {
				t.Fatalf("content[0] 不是对象: %v", content[0])
			}
			if block["type"] != "tool_use" {
				t.Fatalf("content[0] 应为 tool_use，实际 %v", block)
			}
			input, ok := block["input"].(map[string]interface{})
			if !ok {
				t.Fatalf("input 必须是对象，实际 %v（原始 JSON: %s）", block["input"], body)
			}
			if len(input) != tt.wantKeys {
				t.Fatalf("input 键数 = %d，期望 %d: %v", len(input), tt.wantKeys, input)
			}
		})
	}
}
