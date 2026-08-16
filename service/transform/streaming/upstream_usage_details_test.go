package streaming

import "testing"

// 本文件锁定「读上游方向」的 usage 明细完整性：
// 上游给了 cache / reasoning 真值时，转换后的下游流不得丢失或写死 0。
// 这些断言是 usage 从上游归集（而非从下游流反解）的前置保证——
// 解析漏字段，后续无论从哪里捕获都拿不到真值。

// anthropicSSEWithCacheUsage 模拟真实 Anthropic 上游的 usage 拆分方式：
// input 侧（含 cache 明细）只在 message_start 出现，message_delta 只带 output_tokens。
const anthropicSSEWithCacheUsage = `event: message_start
data: {"type":"message_start","message":{"id":"msg_cache","model":"claude-test","created_at":0,"usage":{"input_tokens":100,"cache_read_input_tokens":80,"cache_creation_input_tokens":12}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":42}}

event: message_stop
data: {"type":"message_stop"}

`

// anthropicSSEWithoutCacheUsage 无 prompt caching，用于确认不再输出写死 0 的 details。
const anthropicSSEWithoutCacheUsage = `event: message_start
data: {"type":"message_start","message":{"id":"msg_plain","model":"claude-test","created_at":0,"usage":{"input_tokens":10}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`

// responsesSSEWithTokenDetails 模拟 openai-res 上游返回完整 token 明细。
const responsesSSEWithTokenDetails = `event: response.created
data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_1","object":"response","model":"res-test","created_at":0,"status":"in_progress","output":[]}}

event: response.output_item.added
data: {"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress","content":[]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","sequence_number":2,"output_index":0,"content_index":0,"item_id":"msg_1","delta":"hi"}

event: response.output_item.done
data: {"type":"response.output_item.done","sequence_number":3,"output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hi"}]}}

event: response.completed
data: {"type":"response.completed","sequence_number":4,"response":{"id":"resp_1","object":"response","model":"res-test","created_at":0,"status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hi"}]}],"usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":80},"output_tokens":42,"output_tokens_details":{"reasoning_tokens":7},"total_tokens":142}}}

`

// TestAnthropicUsage_InputSideComesFromMessageStart 断言 input 侧 token 与 cache 明细
// 从 message_start 取，不因 message_delta 只带 output_tokens 而归零。
func TestAnthropicUsage_InputSideComesFromMessageStart(t *testing.T) {
	events := runRealtimeTransform(t, anthropicSSEWithCacheUsage, "anthropic", "openai-res")

	completed := findEvent(t, events, "response.completed")
	usage := nestedMap(t, completed.data, "response", "usage")

	assertNumber(t, usage, "input_tokens", 100)
	assertNumber(t, usage, "output_tokens", 42)
	// total 沿用全仓统一口径 input+output（cache 明细不计入 total，见 models.ResolveTotalTokens）。
	assertNumber(t, usage, "total_tokens", 142)

	inputDetails := nestedMap(t, completed.data, "response", "usage", "input_tokens_details")
	assertNumber(t, inputDetails, "cached_tokens", 80)
}

// TestAnthropicUsage_NoDetailsWhenUpstreamHasNone 断言上游没有 cache / reasoning 时
// 不再输出写死 0 的 details——否则下游会把「没数据」误读成「上游明确报告了 0」。
func TestAnthropicUsage_NoDetailsWhenUpstreamHasNone(t *testing.T) {
	events := runRealtimeTransform(t, anthropicSSEWithoutCacheUsage, "anthropic", "openai-res")

	completed := findEvent(t, events, "response.completed")
	usage := nestedMap(t, completed.data, "response", "usage")

	assertNumber(t, usage, "input_tokens", 10)
	assertNumber(t, usage, "output_tokens", 5)

	if _, ok := usage["input_tokens_details"]; ok {
		t.Fatalf("上游无 cache 命中时不应输出 input_tokens_details: %v", usage)
	}
	// Anthropic 协议无 reasoning token（thinking 计入 output_tokens），永不输出该明细。
	if _, ok := usage["output_tokens_details"]; ok {
		t.Fatalf("anthropic 上游不应输出 output_tokens_details: %v", usage)
	}
}

// TestOpenAIUsage_NoZeroDetailsWhenUpstreamHasNone 断言 openai 上游没给 details 时
// 不写出 cached_tokens:0 / reasoning_tokens:0。与 anthropic 路径同一规则：显式 0
// 会被下游读成「上游明确报告为 0」，而字段缺失才表示「未知」。
func TestOpenAIUsage_NoZeroDetailsWhenUpstreamHasNone(t *testing.T) {
	sse := `data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m",` +
		`"choices":[{"index":0,"delta":{"content":"hi"}}]}` + "\n\n" +
		`data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m",` +
		`"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
		`data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m",` +
		`"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}` + "\n\n" +
		"data: [DONE]\n\n"

	events := runRealtimeTransform(t, sse, "openai", "openai-res")

	completed := findEvent(t, events, "response.completed")
	usage := nestedMap(t, completed.data, "response", "usage")

	assertNumber(t, usage, "input_tokens", 10)
	assertNumber(t, usage, "output_tokens", 5)

	if _, ok := usage["input_tokens_details"]; ok {
		t.Fatalf("上游无 prompt_tokens_details 时不应输出 input_tokens_details: %v", usage)
	}
	if _, ok := usage["output_tokens_details"]; ok {
		t.Fatalf("上游无 completion_tokens_details 时不应输出 output_tokens_details: %v", usage)
	}
}

// TestResponsesUsage_DetailsReachOpenAIClient 断言 openai-res 上游的 cache / reasoning
// 明细能到达 OpenAI Chat 客户端。此前 ResponsesUsage 的 json tag 误用单数
// input_token_details，解码恒 nil，本测试同时锁定 tag 修复。
func TestResponsesUsage_DetailsReachOpenAIClient(t *testing.T) {
	events := runRealtimeTransform(t, responsesSSEWithTokenDetails, "openai-res", "openai")

	var usage map[string]interface{}
	for _, ev := range events {
		if u, ok := ev.data["usage"].(map[string]interface{}); ok {
			usage = u
		}
	}
	if usage == nil {
		t.Fatalf("输出流中未出现带 usage 的 chunk，事件序列: %v", eventTypes(events))
	}

	assertNumber(t, usage, "prompt_tokens", 100)
	assertNumber(t, usage, "completion_tokens", 42)
	assertNumber(t, usage, "total_tokens", 142)

	promptDetails, ok := usage["prompt_tokens_details"].(map[string]interface{})
	if !ok {
		t.Fatalf("usage 缺少 prompt_tokens_details: %v", usage)
	}
	assertNumber(t, promptDetails, "cached_tokens", 80)

	completionDetails, ok := usage["completion_tokens_details"].(map[string]interface{})
	if !ok {
		t.Fatalf("usage 缺少 completion_tokens_details: %v", usage)
	}
	assertNumber(t, completionDetails, "reasoning_tokens", 7)
}

// TestResponsesUsage_CacheReachesAnthropicClient 断言 cached_tokens 映射为 Anthropic 的
// cache_read_input_tokens；reasoning 在 Anthropic 协议无位置，按协议能力裁剪后丢弃。
func TestResponsesUsage_CacheReachesAnthropicClient(t *testing.T) {
	events := runRealtimeTransform(t, responsesSSEWithTokenDetails, "openai-res", "anthropic")

	delta := findEvent(t, events, "message_delta")
	usage, ok := delta.data["usage"].(map[string]interface{})
	if !ok {
		t.Fatalf("message_delta 缺少 usage: %v", delta.data)
	}

	assertNumber(t, usage, "input_tokens", 100)
	assertNumber(t, usage, "output_tokens", 42)
	assertNumber(t, usage, "cache_read_input_tokens", 80)
}
