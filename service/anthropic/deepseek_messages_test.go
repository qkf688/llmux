package anthropic

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/models"
)

// 契约函数本身的边界：两个扩展 type 才算工具变更块，文案与客户端 UI 同源。
func TestIsToolChangeBlockTypeAndNoticeText(t *testing.T) {
	if !IsToolChangeBlockType(BlockTypeToolAddition) || !IsToolChangeBlockType(BlockTypeToolRemoval) {
		t.Fatalf("两个扩展 type 都必须被认领")
	}
	for _, other := range []string{"text", "tool_use", "tool_result", "thinking", ""} {
		if IsToolChangeBlockType(other) {
			t.Errorf("IsToolChangeBlockType(%q) = true, want false", other)
		}
	}

	tests := []struct {
		blockType string
		toolName  string
		want      string
	}{
		{BlockTypeToolAddition, "read_page", "Tool added: read_page"},
		{BlockTypeToolRemoval, "x_search", "Tool removed: x_search"},
		{BlockTypeToolAddition, "", ""}, // 无名 → 空串（调用方据此丢块）
		{"text", "read_page", ""},       // 非扩展 type 不产文案
	}
	for _, tt := range tests {
		if got := ToolChangeNoticeText(tt.blockType, tt.toolName); got != tt.want {
			t.Errorf("ToolChangeNoticeText(%q, %q) = %q, want %q", tt.blockType, tt.toolName, got, tt.want)
		}
	}
}

// 转换路径必须**认领**工具变更块：渲染成文本 part 交给统一模型，而不是落进 default 分支被
// 静默丢弃（丢了模型就看不到「会话中途新增了哪个工具」）。
func TestTransformToUnified_DeepSeekToolChangeBlockBecomesText(t *testing.T) {
	t.Run("只有扩展块：content 收敛为文本字符串", func(t *testing.T) {
		body := []byte(`{"model":"m","max_tokens":1,"messages":[{"role":"system","content":[` +
			`{"type":"tool_addition","tool":{"type":"tool_reference","name":"read_page"}}]}]}`)
		unified, err := TransformToUnified(context.Background(), body)
		if err != nil {
			t.Fatalf("TransformToUnified err = %v", err)
		}
		if len(unified.Messages) != 1 {
			t.Fatalf("messages = %d, want 1", len(unified.Messages))
		}
		got, ok := unified.Messages[0].Content.(string)
		if !ok || got != "Tool added: read_page" {
			t.Fatalf("content = %#v, want %q", unified.Messages[0].Content, "Tool added: read_page")
		}
	})

	t.Run("文本与扩展块混排：就位替换且保序", func(t *testing.T) {
		body := []byte(`{"model":"m","max_tokens":1,"messages":[{"role":"user","content":[` +
			`{"type":"text","text":"before"},` +
			`{"type":"tool_removal","tool":{"type":"tool_reference","name":"x_search"}},` +
			`{"type":"text","text":"after"}]}]}`)
		unified, err := TransformToUnified(context.Background(), body)
		if err != nil {
			t.Fatalf("TransformToUnified err = %v", err)
		}
		parts, ok := unified.Messages[0].Content.([]models.UnifiedMessageContentPart)
		if !ok {
			t.Fatalf("content = %#v, want 块数组", unified.Messages[0].Content)
		}
		want := []string{"before", "Tool removed: x_search", "after"}
		if len(parts) != len(want) {
			t.Fatalf("parts = %d, want %d (%#v)", len(parts), len(want), parts)
		}
		for i, w := range want {
			if parts[i].Text == nil || *parts[i].Text != w {
				t.Errorf("parts[%d] = %#v, want %q", i, parts[i].Text, w)
			}
		}
	})

	t.Run("无名扩展块：不产空壳 part", func(t *testing.T) {
		body := []byte(`{"model":"m","max_tokens":1,"messages":[{"role":"system","content":[{"type":"tool_addition"}]}]}`)
		unified, err := TransformToUnified(context.Background(), body)
		if err != nil {
			t.Fatalf("TransformToUnified err = %v", err)
		}
		if unified.Messages[0].Content != nil {
			t.Fatalf("content = %#v, want nil（认领但无可承载信息）", unified.Messages[0].Content)
		}
	})

	t.Run("未知块类型仍按既有行为丢弃", func(t *testing.T) {
		body := []byte(`{"model":"m","max_tokens":1,"messages":[{"role":"user","content":[{"type":"future_block","text":"x"}]}]}`)
		unified, err := TransformToUnified(context.Background(), body)
		if err != nil {
			t.Fatalf("TransformToUnified err = %v", err)
		}
		if unified.Messages[0].Content != nil {
			t.Fatalf("content = %#v, want nil（未知块不进统一模型）", unified.Messages[0].Content)
		}
	})
}
