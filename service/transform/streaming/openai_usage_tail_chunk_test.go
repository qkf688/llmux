package streaming

import (
	"testing"
)

// 本文件覆盖待办 #18：OpenAI 流式规范里 stream_options.include_usage 的 usage
// 是在 finish_reason **之后**单独发一个 choices:[] 的尾包，而非与 finish_reason 同包。
// 转换器过去假设两者同包，导致尾包被整条丢弃、跨协议后 token 统计全为 0。
//
// 既有 fixture 全把 usage 和 finish 放在同一 chunk，测不出这个缺陷，故单独立文件。
// 公共断言辅助在 test_helpers_test.go。

const (
	// fixtureOpenAIUsageInTailChunk 模仿真实上游（复现日志 #106）：
	// finish_reason 那包 usage 显式为 null，真实 usage 在随后的 choices:[] 尾包里，
	// 且尾包带 prompt_tokens_details / completion_tokens_details 明细。
	fixtureOpenAIUsageInTailChunk = `data: {"id":"chatcmpl-tail","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}

data: {"id":"chatcmpl-tail","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}

data: {"id":"chatcmpl-tail","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[],"usage":{"prompt_tokens":37914,"completion_tokens":135,"total_tokens":38049,"prompt_tokens_details":{"cached_tokens":37000},"completion_tokens_details":{"reasoning_tokens":105}}}

data: [DONE]

`

	// fixtureOpenAIUsageInTailChunkNoDone 同上，但上游直接 EOF 不发 [DONE]。
	// 覆盖流末 finalize 兜底路径：没有 [DONE] 也必须把延后的 completed 发出去。
	fixtureOpenAIUsageInTailChunkNoDone = `data: {"id":"chatcmpl-tail","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}

data: {"id":"chatcmpl-tail","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}

data: {"id":"chatcmpl-tail","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[],"usage":{"prompt_tokens":37914,"completion_tokens":135,"total_tokens":38049,"prompt_tokens_details":{"cached_tokens":37000},"completion_tokens_details":{"reasoning_tokens":105}}}

`

	// fixtureOpenAIUsageWithFinish 是「usage 与 finish_reason 同包」的老行为，
	// 用于确认延后 completed 的改动没有把这条既有路径改坏。
	fixtureOpenAIUsageWithFinish = `data: {"id":"chatcmpl-same","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}

data: {"id":"chatcmpl-same","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":2,"total_tokens":10}}

data: [DONE]

`
)

// 逐帧驱动 handler 用的裸 data 帧（与 fixtureOpenAIUsageInTailChunk 同形）。
const (
	frameOpenAIRoleDelta  = `{"id":"chatcmpl-tail","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}`
	frameOpenAIFinishOnly = `{"id":"chatcmpl-tail","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":null}`
	frameOpenAIUsageTail  = `{"id":"chatcmpl-tail","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[],"usage":{"prompt_tokens":37914,"completion_tokens":135,"total_tokens":38049}}`
)

var usageTailFixtures = []struct {
	name string
	in   string
}{
	{name: "with_done", in: fixtureOpenAIUsageInTailChunk},
	{name: "without_done", in: fixtureOpenAIUsageInTailChunkNoDone},
}

// TestOpenAIUsageTailChunk_ResponsesOut 断言 openai → openai-res 的
// response.completed 带上尾包里的真实 usage。
func TestOpenAIUsageTailChunk_ResponsesOut(t *testing.T) {
	t.Parallel()

	for _, c := range usageTailFixtures {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, "openai", "openai-res")
			completed := findEvent(t, events, "response.completed")
			usage := nestedMap(t, completed.data, "response", "usage")

			assertNumber(t, usage, "input_tokens", 37914)
			assertNumber(t, usage, "output_tokens", 135)
			assertNumber(t, usage, "total_tokens", 38049)
		})
	}
}

// TestOpenAIUsageTailChunk_TokenDetailsArePassedThrough 断言 usage 明细不再被写死 0。
// 尾包里的 prompt_tokens_details.cached_tokens / completion_tokens_details.reasoning_tokens
// 是上游给出的真值，抹成 0 与 #18 同源——openai-res processer 会把 cached_tokens 落库。
func TestOpenAIUsageTailChunk_TokenDetailsArePassedThrough(t *testing.T) {
	t.Parallel()

	for _, c := range usageTailFixtures {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, "openai", "openai-res")
			completed := findEvent(t, events, "response.completed")

			inputDetails := nestedMap(t, completed.data, "response", "usage", "input_tokens_details")
			assertNumber(t, inputDetails, "cached_tokens", 37000)

			outputDetails := nestedMap(t, completed.data, "response", "usage", "output_tokens_details")
			assertNumber(t, outputDetails, "reasoning_tokens", 105)
		})
	}
}

// TestOpenAIUsageTailChunk_AnthropicOut 断言 openai → anthropic 的
// message_delta 带上真实 usage（Anthropic 侧的权威 token 来源，
// 也是 chat_logs 的 usage 提取来源）。
func TestOpenAIUsageTailChunk_AnthropicOut(t *testing.T) {
	t.Parallel()

	for _, c := range usageTailFixtures {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, "openai", "anthropic")
			delta := findEvent(t, events, "message_delta")
			usage, ok := delta.data["usage"].(map[string]interface{})
			if !ok {
				t.Fatalf("message_delta 缺少 usage 字段: %v", delta.data)
			}

			assertNumber(t, usage, "input_tokens", 37914)
			assertNumber(t, usage, "output_tokens", 135)
		})
	}
}

// TestOpenAIUsageWithFinish_StillWorks 确认「usage 与 finish 同包」的既有上游行为未回归。
func TestOpenAIUsageWithFinish_StillWorks(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureOpenAIUsageWithFinish, "openai", "openai-res")
	completed := findEvent(t, events, "response.completed")
	usage := nestedMap(t, completed.data, "response", "usage")

	assertNumber(t, usage, "input_tokens", 8)
	assertNumber(t, usage, "output_tokens", 2)
	assertNumber(t, usage, "total_tokens", 10)
}

// TestOpenAIUsageTailChunk_CompletedIsLast 断言延后 response.completed 后，
// 它仍是流里最后一个事件且只出现一次——否则下游按「completed 即终态」处理会错序。
func TestOpenAIUsageTailChunk_CompletedIsLast(t *testing.T) {
	t.Parallel()

	for _, c := range usageTailFixtures {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, "openai", "openai-res")

			last := events[len(events)-1]
			if last.typ() != "response.completed" {
				t.Fatalf("最后一个事件是 %q，期望 response.completed；实际序列: %v", last.typ(), eventTypes(events))
			}

			if n := countEvents(events, "response.completed"); n != 1 {
				t.Fatalf("response.completed 出现 %d 次，期望恰好 1 次", n)
			}
		})
	}
}

// TestOpenAIUsageTailChunk_FlushesOnUsageArrival 断言终态在 usage 尾包到达时就已交付，
// 不再依赖上游主动收尾（[DONE] / 关连接）。
//
// 为什么必须保证这一点：延后 completed 引入了活性风险——若上游发完 usage 后既不发
// [DONE] 也不关连接（keep-alive 挂住、代理层半开），客户端要等到读超时才拿到终态；
// 而第二跳 responses_to_anthropic 的收尾**只**由 response.completed 驱动、无 EOF 兜底，
// 届时 anthropic 客户端会拿到未关闭的 content_block 且并行 tool_call 参数静默丢失。
//
// 白盒断言（检查 pendingCompleted 已被交付）而非黑盒：输出内容上「usage 到达即发」与
// 「[DONE] 时才发」完全一致，只有 state 能区分二者。
func TestOpenAIUsageTailChunk_FlushesOnUsageArrival(t *testing.T) {
	t.Parallel()

	state, cleanup := newTestRealtimeState(t, "openai", "openai-res")
	defer cleanup()

	for _, frame := range []string{frameOpenAIRoleDelta, frameOpenAIFinishOnly} {
		if err := handleRealtimeOpenAIToResponses(state, frame); err != nil {
			t.Fatalf("处理帧失败: %v", err)
		}
	}
	if state.pendingCompleted == nil {
		t.Fatal("finish_reason 之后 completed 应处于延后状态，等待 usage 尾包")
	}

	if err := handleRealtimeOpenAIToResponses(state, frameOpenAIUsageTail); err != nil {
		t.Fatalf("处理 usage 尾包失败: %v", err)
	}
	if state.pendingCompleted != nil {
		t.Fatal("usage 尾包到达后 completed 应已交付，不应继续等待 [DONE]")
	}

	// [DONE] 退化为幂等兜底：已交付则什么都不做。
	if err := handleRealtimeOpenAIToResponses(state, "[DONE]"); err != nil {
		t.Fatalf("[DONE] 幂等兜底失败: %v", err)
	}
}
