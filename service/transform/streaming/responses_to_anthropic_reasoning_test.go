package streaming

import (
	"strings"
	"testing"
)

// fixtureResponsesReasoningContentInterleaved 模拟 reasoning 与 content 交错的 Responses 流：
// reasoning delta 与 output_text.delta 按 output_index 交错到达（content 先出后再插 reasoning）。
// 验证 thinking delta 与 text delta 都完整到达，block 生命周期合法。
const fixtureResponsesReasoningContentInterleaved = `event: response.created
data: {"type":"response.created","response":{"id":"resp_rc","status":"in_progress","model":"o1"}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"type":"reasoning","id":"reasoning_1","status":"in_progress","summary":[]}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":1,"item":{"type":"message","id":"msg_1","status":"in_progress","role":"assistant","content":[]}}

event: response.reasoning_summary_text.delta
data: {"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"思考"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","output_index":1,"delta":"答案"}

event: response.reasoning_summary_text.delta
data: {"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"过程"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","output_index":1,"delta":"是"}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_rc","status":"completed","model":"o1","output":[{"type":"reasoning","id":"reasoning_1","summary":[{"type":"summary_text","text":"思考过程"}]},{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"答案是"}]}],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}

data: [DONE]

`

// TestStreamConversion_ResponsesToAnthropic_ReasoningContentInterleaved 覆盖场景 2：
// reasoning 与 content 交错到达（content 先出后再插 reasoning，上游协议异常）时，
// block 生命周期合法（不产出非法 SSE），text 完整到达。
// reasoning 被 text 切走后迟到的 delta 丢弃（thinking block 不重开，避免分裂），
// 这是上游异常的合理处理——优先保 text（最终输出）完整。
func TestStreamConversion_ResponsesToAnthropic_ReasoningContentInterleaved(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureResponsesReasoningContentInterleaved, "openai-res", "anthropic")

	openBlocks := map[int]string{}
	closedBlocks := map[int]bool{}

	var thinkingAll, textAll strings.Builder

	for i, ev := range events {
		switch ev.typ() {
		case "content_block_start":
			block, ok := ev.data["content_block"].(map[string]interface{})
			if !ok {
				t.Fatalf("事件 #%d content_block_start 缺少 content_block 字段: %v", i, ev.data)
			}
			blockType, _ := block["type"].(string)
			if !legalAnthropicBlockTypes[blockType] {
				t.Errorf("事件 #%d 非法 content_block 类型 %q", i, blockType)
			}
			idx := ev.index(t)
			if _, dup := openBlocks[idx]; dup {
				t.Errorf("事件 #%d content_block index %d 重复 start", i, idx)
			}
			openBlocks[idx] = blockType

		case "content_block_delta":
			idx := ev.index(t)
			if _, open := openBlocks[idx]; !open {
				t.Errorf("事件 #%d content_block_delta 落在未开的 block index %d 上", i, idx)
				continue
			}
			if closedBlocks[idx] {
				t.Errorf("事件 #%d content_block_delta 落在已 stop 的 block index %d 上", i, idx)
			}
			delta, _ := ev.data["delta"].(map[string]interface{})
			switch delta["type"] {
			case "text_delta":
				if s, _ := delta["text"].(string); s != "" {
					textAll.WriteString(s)
				}
			case "thinking_delta":
				if s, _ := delta["thinking"].(string); s != "" {
					thinkingAll.WriteString(s)
				}
			}

		case "content_block_stop":
			idx := ev.index(t)
			delete(openBlocks, idx)
			closedBlocks[idx] = true
		}
	}

	if len(openBlocks) != 0 {
		t.Errorf("流结束时仍有未关闭的 content_block: %v", openBlocks)
	}

	// text 完整到达（最终输出优先保）
	if got, want := textAll.String(), "答案是"; got != want {
		t.Errorf("text delta 不完整：got %q, want %q", got, want)
	}
	// thinking 只收到 text 切走前的部分（"思考"），切走后迟到的 "过程" 丢弃
	if got, want := thinkingAll.String(), "思考"; got != want {
		t.Errorf("thinking delta 应为切走前的部分 %q，got %q", want, got)
	}
}
