package streaming

import (
	"strings"
	"testing"
)

// fixtureOpenAIReasoningBeforeContentStream 模拟推理模型（如 deepseek-r1 / o1）的 OpenAI Chat 流：
// 第一个 chunk 只带 role；随后多条 reasoning_content（思考）；再随后多条 content（正文）；最后 finish_reason=stop。
// 这是 log-33 复现 bug 的最小化输入：reasoning 先于 content 到达。
const fixtureOpenAIReasoningBeforeContentStream = `data: {"id":"chatcmpl-r1","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-flash","choices":[{"index":0,"finish_reason":null,"logprobs":null,"delta":{"role":"assistant","content":null,"reasoning_content":""}}],"usage":null}

data: {"id":"chatcmpl-r1","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-flash","choices":[{"index":0,"finish_reason":null,"logprobs":null,"delta":{"content":null,"reasoning_content":"用户"}}],"usage":null}

data: {"id":"chatcmpl-r1","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-flash","choices":[{"index":0,"finish_reason":null,"logprobs":null,"delta":{"content":null,"reasoning_content":"问你是谁"}}],"usage":null}

data: {"id":"chatcmpl-r1","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-flash","choices":[{"index":0,"finish_reason":null,"logprobs":null,"delta":{"content":"我是","reasoning_content":null}}],"usage":null}

data: {"id":"chatcmpl-r1","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-flash","choices":[{"index":0,"finish_reason":null,"logprobs":null,"delta":{"content":"助手","reasoning_content":null}}],"usage":null}

data: {"id":"chatcmpl-r1","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-flash","choices":[{"index":0,"finish_reason":null,"logprobs":null,"delta":{"content":"。","reasoning_content":null}}],"usage":null}

data: {"id":"chatcmpl-r1","object":"chat.completion.chunk","created":1234567890,"model":"deepseek-v4-flash","choices":[{"index":0,"finish_reason":"stop","logprobs":null,"delta":{}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]

`

// TestStreamConversion_OpenAIToAnthropic_ReasoningBeforeContent 覆盖 log-33 复现的 bug：
// 推理模型 reasoning_content 先于 content 到达时，Anthropic SSE 输出的 text content_block
// 被提前 content_block_stop，导致正文 text_delta 全发到已关闭的 block 上。
//
// 期望顺序：message_start → cbs(thinking,0) → thinking_delta(0)*N → cb_stop(0)
//
//	→ cbs(text,1) → text_delta(1)*N → cb_stop(1) → message_delta → message_stop
//
// 不变量：
//  1. 每个 content_block_delta 必须落在「已 start、未 stop」的 block 上（block 生命周期合法）
//  2. thinking 块先开先关，text 块后开后关（顺序正确，text 不被提前关）
//  3. text_delta 的 index 与 text block 的 index 一致，thinking_delta 的 index 与 thinking block 的 index 一致
func TestStreamConversion_OpenAIToAnthropic_ReasoningBeforeContent(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAIReasoningBeforeContentStream, "openai", "anthropic")

	// 追踪 block 生命周期：index -> block 类型（"text"/"thinking"/...）
	openBlocks := map[int]string{}
	closedBlocks := map[int]bool{}
	var blockOpenOrder []int // 记录开块顺序，用于断言 thinking 先于 text

	// 记录各类型 block 的 index
	thinkingBlockIdx := -1
	textBlockIdx := -1

	// 每个 delta 必须落在已开未关的 block 上
	for i, ev := range events {
		switch ev.typ() {
		case "message_start":
			if i != 0 {
				t.Errorf("message_start 应为第一个事件，实际出现在第 %d 个", i)
			}

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
				t.Errorf("事件 #%d content_block index %d 重复 start（未 stop 就再开）", i, idx)
			}
			openBlocks[idx] = blockType
			blockOpenOrder = append(blockOpenOrder, idx)

			if blockType == "thinking" && thinkingBlockIdx == -1 {
				thinkingBlockIdx = idx
			}
			if blockType == "text" && textBlockIdx == -1 {
				textBlockIdx = idx
			}

		case "content_block_delta":
			idx := ev.index(t)
			bt, open := openBlocks[idx]
			if !open {
				t.Errorf("事件 #%d content_block_delta 落在未开的 block index %d 上（已关闭或从未开）", i, idx)
				continue
			}
			if closedBlocks[idx] {
				t.Errorf("事件 #%d content_block_delta 落在已 stop 的 block index %d 上", i, idx)
			}
			// delta 类型应与 block 类型匹配
			delta, _ := ev.data["delta"].(map[string]interface{})
			deltaType, _ := delta["type"].(string)
			if bt == "thinking" && deltaType != "thinking_delta" {
				t.Errorf("事件 #%d thinking block 上收到非 thinking_delta（%q）", i, deltaType)
			}
			if bt == "text" && deltaType != "text_delta" {
				t.Errorf("事件 #%d text block 上收到非 text_delta（%q）", i, deltaType)
			}

		case "content_block_stop":
			idx := ev.index(t)
			if _, open := openBlocks[idx]; !open {
				t.Errorf("事件 #%d content_block_stop index %d 没有对应的 start", i, idx)
			}
			delete(openBlocks, idx)
			closedBlocks[idx] = true
		}
	}

	if len(openBlocks) != 0 {
		t.Errorf("流结束时仍有未关闭的 content_block: %v", openBlocks)
	}

	// 不变量 2：thinking 块先开，text 块后开
	if thinkingBlockIdx == -1 {
		t.Error("未产出 thinking content_block（reasoning_content 应映射为 thinking 块）")
	}
	if textBlockIdx == -1 {
		t.Error("未产出 text content_block（正文 content 应映射为 text 块）")
	}
	if thinkingBlockIdx >= 0 && textBlockIdx >= 0 {
		// 找两个 block 在开块顺序里的位置
		thinkingPos, textPos := -1, -1
		for pos, idx := range blockOpenOrder {
			if idx == thinkingBlockIdx && thinkingPos == -1 {
				thinkingPos = pos
			}
			if idx == textBlockIdx && textPos == -1 {
				textPos = pos
			}
		}
		if thinkingPos >= 0 && textPos >= 0 && thinkingPos > textPos {
			t.Errorf("thinking 块（开块顺序第 %d）应在 text 块（第 %d）之前开启", thinkingPos, textPos)
		}
		if thinkingBlockIdx == textBlockIdx {
			t.Errorf("thinking 块与 text 块共用 index %d", thinkingBlockIdx)
		}
	}
}

// TestStreamConversion_OpenAIToAnthropic_ReasoningBeforeContent_NoTextDropped
// 断言正文与思考内容都完整到达，没有被丢弃或错位。
func TestStreamConversion_OpenAIToAnthropic_ReasoningBeforeContent_NoTextDropped(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAIReasoningBeforeContentStream, "openai", "anthropic")

	var textAll, thinkingAll strings.Builder
	for _, ev := range events {
		if ev.typ() != "content_block_delta" {
			continue
		}
		delta, ok := ev.data["delta"].(map[string]interface{})
		if !ok {
			continue
		}
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
	}

	wantText := "我是助手。"
	wantThinking := "用户问你是谁"

	if got := textAll.String(); got != wantText {
		t.Errorf("正文 text 被截断或错位：got %q, want %q", got, wantText)
	}
	if got := thinkingAll.String(); got != wantThinking {
		t.Errorf("思考 thinking 被截断或错位：got %q, want %q", got, wantThinking)
	}
}

// fixtureOpenAIPureTextStream 模拟普通（非推理）模型的 OpenAI Chat 流：
// 第一个 chunk 只带 role；随后多条 content（正文）；最后 finish_reason=stop。
// 用于覆盖场景 (a)：纯 text，无 reasoning、无 tool_call。
// 惰性开块的「active=-1 直接开」分支在此场景下被最干净地独立覆盖
// （场景 (b) 里该分支被「先关 thinking」带前置覆盖，无法独立验证）。
const fixtureOpenAIPureTextStream = `data: {"id":"chatcmpl-txt","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"finish_reason":null,"logprobs":null,"delta":{"role":"assistant","content":""}}],"usage":null}

data: {"id":"chatcmpl-txt","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"finish_reason":null,"logprobs":null,"delta":{"content":"Hello"}}],"usage":null}

data: {"id":"chatcmpl-txt","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"finish_reason":null,"logprobs":null,"delta":{"content":" world"}}],"usage":null}

data: {"id":"chatcmpl-txt","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"finish_reason":"stop","logprobs":null,"delta":{}}],"usage":{"prompt_tokens":8,"completion_tokens":2,"total_tokens":10}}

data: [DONE]

`

// TestStreamConversion_OpenAIToAnthropic_PureText 覆盖场景 (a)：
// 纯 text 流（无 reasoning、无 tool_call）经 openai→anthropic 转换后，
// text content_block 由第一个 output_text.delta 惰性开启（active=-1 直接开，无前置关块），
// 每个 text_delta 落在 [start, stop) 区间，正文完整不丢。
//
// 期望顺序：message_start → cbs(text,0) → text_delta(0)*N → cb_stop(0) → message_delta → message_stop
func TestStreamConversion_OpenAIToAnthropic_PureText(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAIPureTextStream, "openai", "anthropic")

	// block 生命周期追踪
	openBlocks := map[int]string{}
	closedBlocks := map[int]bool{}
	var textBlockIdx = -1

	for i, ev := range events {
		switch ev.typ() {
		case "message_start":
			if i != 0 {
				t.Errorf("message_start 应为第一个事件，实际出现在第 %d 个", i)
			}

		case "content_block_start":
			block, ok := ev.data["content_block"].(map[string]interface{})
			if !ok {
				t.Fatalf("事件 #%d content_block_start 缺少 content_block 字段: %v", i, ev.data)
			}
			blockType, _ := block["type"].(string)
			if blockType != "text" {
				t.Errorf("事件 #%d 纯 text 流只应有 text block，实际 %q", i, blockType)
			}
			idx := ev.index(t)
			if _, dup := openBlocks[idx]; dup {
				t.Errorf("事件 #%d content_block index %d 重复 start", i, idx)
			}
			openBlocks[idx] = blockType
			if textBlockIdx == -1 {
				textBlockIdx = idx
			}

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
			if dt, _ := delta["type"].(string); dt != "text_delta" {
				t.Errorf("事件 #%d 纯 text 流只应有 text_delta，实际 %q", i, dt)
			}

		case "content_block_stop":
			idx := ev.index(t)
			if _, open := openBlocks[idx]; !open {
				t.Errorf("事件 #%d content_block_stop index %d 没有对应的 start", i, idx)
			}
			delete(openBlocks, idx)
			closedBlocks[idx] = true
		}
	}

	if len(openBlocks) != 0 {
		t.Errorf("流结束时仍有未关闭的 content_block: %v", openBlocks)
	}
	if textBlockIdx == -1 {
		t.Fatal("未产出 text content_block")
	}

	// 正文完整性
	var textAll strings.Builder
	for _, ev := range events {
		if ev.typ() != "content_block_delta" {
			continue
		}
		delta, _ := ev.data["delta"].(map[string]interface{})
		if s, _ := delta["text"].(string); s != "" {
			textAll.WriteString(s)
		}
	}
	if got, want := textAll.String(), "Hello world"; got != want {
		t.Errorf("正文 text 被截断或错位：got %q, want %q", got, want)
	}
}

// fixtureOpenAINoDeltaStream 模拟「只有 role chunk、无任何 content/reasoning delta」的流：
// 第一个 chunk 带 role 但 content 为空；直接 finish_reason=stop。
// 用于覆盖场景 (f)：全程无 delta，确认不会发空 text block、不会漏关。
const fixtureOpenAINoDeltaStream = `data: {"id":"chatcmpl-none","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"finish_reason":null,"logprobs":null,"delta":{"role":"assistant","content":""}}],"usage":null}

data: {"id":"chatcmpl-none","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"finish_reason":"stop","logprobs":null,"delta":{}}],"usage":{"prompt_tokens":5,"completion_tokens":0,"total_tokens":5}}

data: [DONE]

`

// TestStreamConversion_OpenAIToAnthropic_NoDelta 覆盖场景 (f)：
// 全程无 content/reasoning delta 时，不应产出任何 content_block_start/stop/delta，
// 只应有 message_start → message_delta → message_stop。
func TestStreamConversion_OpenAIToAnthropic_NoDelta(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAINoDeltaStream, "openai", "anthropic")

	for i, ev := range events {
		switch ev.typ() {
		case "content_block_start", "content_block_stop", "content_block_delta":
			t.Errorf("事件 #%d 全程无 delta 流不应产出 %s: %v", i, ev.typ(), ev.data)
		}
	}

	// 确认基本事件存在
	wantTypes := map[string]bool{
		"message_start":  false,
		"message_delta":  false,
		"message_stop":   false,
	}
	for _, ev := range events {
		if _, ok := wantTypes[ev.typ()]; ok {
			wantTypes[ev.typ()] = true
		}
	}
	for typ, found := range wantTypes {
		if !found {
			t.Errorf("缺少必要事件 %s", typ)
		}
	}
}
