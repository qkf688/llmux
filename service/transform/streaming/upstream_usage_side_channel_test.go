package streaming

import (
	"io"
	"strings"
	"testing"

	"github.com/qkf688/llmux/models"
)

// #19 的核心回归：客户端用 anthropic style 时，Anthropic Messages 协议没有
// reasoning_tokens 槽位，从转换后的下游流反解必然拿不到它。侧信道必须在读上游
// 那一跳就把原始 usage 交出来，使落库侧仍能拿到真实的 reasoning_tokens。
func TestSideChannel_UpstreamReasoningSurvivesAnthropicClient(t *testing.T) {
	// 上游 openai：finish 之后单独发 usage 尾包（stream_options.include_usage 规范行为）。
	sse := `data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"hi"}}]}` + "\n\n" +
		`data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
		`data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[],"usage":{"prompt_tokens":120,"completion_tokens":88,"total_tokens":208,"prompt_tokens_details":{"cached_tokens":64},"completion_tokens_details":{"reasoning_tokens":55}}}` + "\n\n" +
		"data: [DONE]\n\n"

	sc := models.NewTransformSideChannel(false)
	// openai -> anthropic 是双跳（经 openai-res 中枢）；侧信道只挂第一跳。
	out, err := TransformResponseRealtime(newSSEResponse(sse), "openai", "anthropic", sc)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	body, _ := io.ReadAll(out.Body)

	// 前提确认：下游 anthropic 流确实没有 reasoning_tokens——否则这个测试没在测真问题。
	if strings.Contains(string(body), "reasoning_tokens") {
		t.Fatalf("anthropic downstream unexpectedly carries reasoning_tokens; test premise broken:\n%s", body)
	}

	u, ok := sc.UpstreamUsage()
	if !ok {
		t.Fatal("side channel captured no upstream usage")
	}
	if u.PromptTokens != 120 || u.CompletionTokens != 88 || u.TotalTokens != 208 {
		t.Errorf("tokens = %d/%d/%d, want 120/88/208", u.PromptTokens, u.CompletionTokens, u.TotalTokens)
	}
	if u.CompletionTokensDetails.ReasoningTokens != 55 {
		t.Errorf("ReasoningTokens = %d, want 55 (lost across anthropic conversion)", u.CompletionTokensDetails.ReasoningTokens)
	}
	if u.PromptTokensDetails.CachedTokens != 64 {
		t.Errorf("CachedTokens = %d, want 64", u.PromptTokensDetails.CachedTokens)
	}
}

// 上游本身是 openai-res 的单跳转换：usage 已是结构化的，同样要进侧信道。
func TestSideChannel_UpstreamResponsesUsageCaptured(t *testing.T) {
	sse := `data: {"type":"response.completed","response":{"id":"r1","usage":{"input_tokens":300,"output_tokens":70,"total_tokens":370,"input_tokens_details":{"cached_tokens":128},"output_tokens_details":{"reasoning_tokens":33}}}}` + "\n\n"

	sc := models.NewTransformSideChannel(false)
	out, err := TransformResponseRealtime(newSSEResponse(sse), "openai-res", "anthropic", sc)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	io.Copy(io.Discard, out.Body)

	u, ok := sc.UpstreamUsage()
	if !ok {
		t.Fatal("side channel captured no upstream usage")
	}
	if u.TotalTokens != 370 {
		t.Errorf("TotalTokens = %d, want 370", u.TotalTokens)
	}
	if u.CompletionTokensDetails.ReasoningTokens != 33 {
		t.Errorf("ReasoningTokens = %d, want 33", u.CompletionTokensDetails.ReasoningTokens)
	}
	if u.PromptTokensDetails.CachedTokens != 128 {
		t.Errorf("CachedTokens = %d, want 128", u.PromptTokensDetails.CachedTokens)
	}
}

// Anthropic 上游把 usage 拆在 message_start（input 侧）和 message_delta（output 侧），
// 侧信道是「最后观测胜出」，必须写入合并后的完整快照而非分两次覆盖。
func TestSideChannel_AnthropicUpstreamUsageIsMergedNotOverwritten(t *testing.T) {
	sse := "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"m1","model":"claude","usage":{"input_tokens":500,"cache_read_input_tokens":256}}}` + "\n\n" +
		"event: message_delta\n" +
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":90}}` + "\n\n"

	sc := models.NewTransformSideChannel(false)
	out, err := TransformResponseRealtime(newSSEResponse(sse), "anthropic", "openai", sc)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	io.Copy(io.Discard, out.Body)

	u, ok := sc.UpstreamUsage()
	if !ok {
		t.Fatal("side channel captured no upstream usage")
	}
	// input 侧只出现在 message_start：若被 message_delta 的快照覆盖就会变 0。
	if u.PromptTokens != 500 {
		t.Errorf("PromptTokens = %d, want 500 (message_start value must not be overwritten)", u.PromptTokens)
	}
	if u.CompletionTokens != 90 {
		t.Errorf("CompletionTokens = %d, want 90", u.CompletionTokens)
	}
	if u.PromptTokensDetails.CachedTokens != 256 {
		t.Errorf("CachedTokens = %d, want 256", u.PromptTokensDetails.CachedTokens)
	}
	// 落库口径与 process.go 一致：total 只算 input+output，不额外加 cache。
	if u.TotalTokens != 590 {
		t.Errorf("TotalTokens = %d, want 590", u.TotalTokens)
	}
}

// 直通/无旁路场景：sideChannel 为 nil 时捕获逻辑必须是 no-op，不能 panic。
func TestSideChannel_NilIsNoOp(t *testing.T) {
	sse := `data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"m","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}` + "\n\n"
	out, err := TransformResponseRealtime(newSSEResponse(sse), "openai", "anthropic", nil)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	io.Copy(io.Discard, out.Body)
}
