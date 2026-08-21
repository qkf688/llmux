package streaming

import (
	"io"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

// 上游省略 total_tokens 时，「发给客户端的 total」与「侧信道交给落库侧的 total」
// 必须同口径（review-03 #6）：侧信道那侧有 total==0 → prompt+completion 的回退，
// 下游写出点却直写上游原值，结果客户端看到 0 而 DB 记的是回退值。
const (
	// openai-res 上游只报 input/output 不报 total——真实上游（Anthropic 兼容门面、
	// 部分 OpenAI 兼容网关）会这样。
	fixtureResponsesUsageNoTotal = `data: {"type":"response.completed","response":{"id":"r_nototal","status":"completed","output":[{"type":"output_text","text":"hi"}],"usage":{"input_tokens":30026,"output_tokens":162}}}` + "\n\n"

	// openai 上游尾包只报 prompt/completion 不报 total。
	fixtureOpenAIUsageNoTotal = `data: {"id":"c_nototal","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"hi"}}]}` + "\n\n" +
		`data: {"id":"c_nototal","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
		`data: {"id":"c_nototal","object":"chat.completion.chunk","created":1,"model":"m","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}` + "\n\n" +
		"data: [DONE]\n\n"
)

// TestResponsesToOpenAI_TotalTokensFallback 覆盖 openai-res → openai：上游缺
// total_tokens 时客户端不应看到 0，且与侧信道落库口径一致。
func TestResponsesToOpenAI_TotalTokensFallback(t *testing.T) {
	events := runRealtimeTransform(t, fixtureResponsesUsageNoTotal, "openai-res", "openai")

	var usage map[string]interface{}
	for _, ev := range events {
		if u, ok := ev.data["usage"].(map[string]interface{}); ok {
			usage = u
		}
	}
	if usage == nil {
		t.Fatalf("输出流中未出现带 usage 的 chunk，事件序列: %v", eventTypes(events))
	}

	assertNumber(t, usage, "prompt_tokens", 30026)
	assertNumber(t, usage, "completion_tokens", 162)
	assertNumber(t, usage, "total_tokens", 30188)

	if got := sideChannelTotalTokens(t, fixtureResponsesUsageNoTotal, "openai-res", "openai"); got != 30188 {
		t.Errorf("侧信道 TotalTokens = %d，期望 30188（须与客户端同口径）", got)
	}
}

// TestOpenAIToResponses_TotalTokensFallback 覆盖 openai → openai-res：同一条回退
// 口径在这条线上也必须成立（装配与 responses→openai 共用 usageWireFromModel）。
func TestOpenAIToResponses_TotalTokensFallback(t *testing.T) {
	events := runRealtimeTransform(t, fixtureOpenAIUsageNoTotal, "openai", "openai-res")

	completed := findEvent(t, events, "response.completed")
	usage := nestedMap(t, completed.data, "response", "usage")

	assertNumber(t, usage, "input_tokens", 10)
	assertNumber(t, usage, "output_tokens", 5)
	assertNumber(t, usage, "total_tokens", 15)

	if got := sideChannelTotalTokens(t, fixtureOpenAIUsageNoTotal, "openai", "openai-res"); got != 15 {
		t.Errorf("侧信道 TotalTokens = %d，期望 15（须与客户端同口径）", got)
	}
}

// sideChannelTotalTokens 跑一遍带侧信道的转换，返回落库侧拿到的 TotalTokens。
func sideChannelTotalTokens(t *testing.T, sse string, from, to consts.WireFormat) int64 {
	t.Helper()

	sc := models.NewTransformSideChannel(false)
	out, err := TransformResponseRealtime(newSSEResponse(sse), from, to, sc)
	if err != nil {
		t.Fatalf("TransformResponseRealtime(%s->%s): %v", from, to, err)
	}
	defer out.Body.Close()
	_, _ = io.Copy(io.Discard, out.Body)

	u, ok := sc.UpstreamUsage()
	if !ok {
		t.Fatal("侧信道未记录上游 usage")
	}
	return u.TotalTokens
}
