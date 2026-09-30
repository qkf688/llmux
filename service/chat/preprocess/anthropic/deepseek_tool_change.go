package anthropic

import (
	"encoding/json"
	"fmt"
	"log/slog"

	anthropicsvc "github.com/qkf688/llmux/service/anthropic"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// NormalizeDeepSeekToolChangeBlocks 就地把 Anthropic 形状请求体里 DeepSeek Messages 的
// 工具变更块（messages[].content 内的 tool_addition / tool_removal）降级成文本块。
//
// 为什么必须在直通路径做：直通 = 客户端 body 原样出站。客户端协议若**超出**严格 Anthropic
// 规范（DeepSeek Messages 就是这样的超集），私有块会整份泄给上游，而严格上游对 content 的
// 块 type 做白名单校验，遇到私有块直接 400 InvalidParameter "Request body format invalid"；
// 更糟的是该消息一旦进入会话历史就永久留在那里，此后**每条**请求都被拒、客户端重试只是
// 重复同一条已被拒的 body（2026-09-30 实际事故，日志 11517：连续 12 条 400 = 同一会话
// 2 次请求 × 6 次尝试）。归一化把私有块换成 "Tool added: <name>" 文本——语义不丢
// （工具 schema 由 tools 数组承载），形状收敛到严格上游能接受的集合。
//
// 与同目录 OpenAI 侧补齐同口径：best-effort / 不阻断主流程——body 不是 JSON 对象、没有
// messages 数组、没有扩展块时原样返回；无扩展块时走 gjson 快判直接返回（热路径不做
// JSON 往返）。改写用 sjson 定点替换/删除、不整体重序列化，其余字节（键序、数字精度、
// 空白）保持原样，与 stripThinkingBlocksFromMessages 同一套纪律。
//
// 返回 changed 报告是否发生改写；error 只在定点改写失败时返回，调用方按非阻断处理
// （保留原 body，是否被上游拒收由上游判定，不静默吞错）。
func NormalizeDeepSeekToolChangeBlocks(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return body, false, nil
	}
	msgs := messages.Array()
	if !hasToolChangeBlock(msgs) {
		return body, false, nil
	}

	changed := false
	// 倒序处理：删块与删消息都会移动后续元素的索引，倒序保证快照索引在改写后仍然有效。
	for i := len(msgs) - 1; i >= 0; i-- {
		contentPath := fmt.Sprintf("messages.%d.content", i)
		content := gjson.GetBytes(body, contentPath)
		if !content.IsArray() {
			continue
		}

		blocks := content.Array()
		for j := len(blocks) - 1; j >= 0; j-- {
			blockType := blocks[j].Get("type").String()
			if !anthropicsvc.IsToolChangeBlockType(blockType) {
				continue
			}

			blockPath := fmt.Sprintf("%s.%d", contentPath, j)
			text := anthropicsvc.ToolChangeNoticeText(blockType, blocks[j].Get("tool.name").String())
			if text == "" {
				// 认得出是扩展块、却拿不到工具名 = 没有可承载的信息：留着只会让严格上游
				// 继续判 400，丢弃并留痕。
				slog.Warn("dropping deepseek tool change block without tool name",
					"block_type", blockType, "path", blockPath)
				next, err := sjson.DeleteBytes(body, blockPath)
				if err != nil {
					return body, changed, fmt.Errorf("delete deepseek tool change block %s: %w", blockPath, err)
				}
				body = next
				changed = true
				continue
			}

			replacement, err := json.Marshal(toolChangeNoticeBlock{Type: "text", Text: text})
			if err != nil {
				return body, changed, fmt.Errorf("marshal tool change notice block: %w", err)
			}
			next, err := sjson.SetRawBytes(body, blockPath, replacement)
			if err != nil {
				return body, changed, fmt.Errorf("replace deepseek tool change block %s: %w", blockPath, err)
			}
			body = next
			changed = true
		}

		// 块被删空的消息整条移除：Anthropic 要求 content 至少一个块，留空数组同样判 400。
		if remain := gjson.GetBytes(body, contentPath); remain.IsArray() && len(remain.Array()) == 0 {
			next, err := sjson.DeleteBytes(body, fmt.Sprintf("messages.%d", i))
			if err != nil {
				return body, changed, fmt.Errorf("delete emptied message %d: %w", i, err)
			}
			body = next
			changed = true
		}
	}

	return body, changed, nil
}

// toolChangeNoticeBlock 是归一化后写回的文本块，只在 marshal 时使用。
type toolChangeNoticeBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// hasToolChangeBlock 快判请求体里是否存在工具变更块，让无扩展的请求跳过 JSON 往返。
func hasToolChangeBlock(msgs []gjson.Result) bool {
	for _, msg := range msgs {
		content := msg.Get("content")
		if !content.IsArray() {
			continue
		}
		for _, block := range content.Array() {
			if anthropicsvc.IsToolChangeBlockType(block.Get("type").String()) {
				return true
			}
		}
	}
	return false
}
