package openai

import (
	"encoding/json"
	"testing"

	"github.com/atopos31/llmio/models"
)

// 本文件对应 .local/next-do.md 的待办 28。
//
// OpenAI Chat 的 tool 消息 content 不接受 image_url 块，故多模态 tool 结果在此
// 降级为纯文本 + 占位符；关键不变量是「不得把 image_url 块塞给上游」「不得降级成空串」。

func decodeOpenAIMessages(t *testing.T, unified *models.UnifiedRequest) []map[string]interface{} {
	t.Helper()

	raw, err := FromUnified(unified)
	if err != nil {
		t.Fatalf("FromUnified 失败: %v", err)
	}

	var req struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("解析输出失败: %v (%s)", err, raw)
	}
	return req.Messages
}

func TestFromUnified_ToolMessageDowngradesMultimodalContent(t *testing.T) {
	text := "截图如下"
	messages := decodeOpenAIMessages(t, &models.UnifiedRequest{
		Model: "gpt-4o",
		Messages: []models.UnifiedMessage{
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content: []models.UnifiedMessageContentPart{
					{Type: "text", Text: &text},
					{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAAA"}},
				},
			},
		},
	})

	if len(messages) != 1 {
		t.Fatalf("应产出 1 条消息，实际 %d", len(messages))
	}
	content, ok := messages[0]["content"].(string)
	if !ok {
		t.Fatalf("tool 消息 content 必须降级为字符串，实际 %#v", messages[0]["content"])
	}
	if content != "截图如下[image]" {
		t.Fatalf("content 应为文本与占位符混排，实际 %q", content)
	}
	if messages[0]["tool_call_id"] != "call_1" {
		t.Fatalf("tool_call_id 丢失: %#v", messages[0]["tool_call_id"])
	}
}

func TestFromUnified_ImageOnlyToolMessageNotEmpty(t *testing.T) {
	messages := decodeOpenAIMessages(t, &models.UnifiedRequest{
		Model: "gpt-4o",
		Messages: []models.UnifiedMessage{
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content: []models.UnifiedMessageContentPart{
					{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAAA"}},
				},
			},
		},
	})

	if content, ok := messages[0]["content"].(string); !ok || content != "[image]" {
		t.Fatalf("纯图片 tool 结果不得降级为空串，实际 %#v", messages[0]["content"])
	}
}

// 形态守护：非 tool 角色的多模态内容仍按块数组透传。
func TestFromUnified_UserMultimodalContentUnchanged(t *testing.T) {
	text := "这是什么"
	messages := decodeOpenAIMessages(t, &models.UnifiedRequest{
		Model: "gpt-4o",
		Messages: []models.UnifiedMessage{
			{
				Role: "user",
				Content: []models.UnifiedMessageContentPart{
					{Type: "text", Text: &text},
					{Type: "image_url", ImageURL: &models.UnifiedImageURL{URL: "data:image/png;base64,AAAA"}},
				},
			},
		},
	})

	parts, ok := messages[0]["content"].([]interface{})
	if !ok || len(parts) != 2 {
		t.Fatalf("user 多模态内容应保持块数组，实际 %#v", messages[0]["content"])
	}
	second, _ := parts[1].(map[string]interface{})
	if second["type"] != "image_url" {
		t.Fatalf("图片块被改写: %#v", second)
	}
}
