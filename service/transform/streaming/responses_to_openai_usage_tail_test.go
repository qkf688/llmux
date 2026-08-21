package streaming

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 本文件锁定 openai-res → openai 的**出站** usage 形状。
//
// 项目在入站侧已把「usage 在 finish_reason 之后的独立 choices:[] 尾包里」认定为
// OpenAI 规范形状，并为此付出了延后 response.completed 的工程代价（见
// realtime.go 的 pendingCompleted/pendingUsage 注释、openai_usage_tail_chunk_test.go）。
// 出站侧过去却把 usage 与 finish_reason 塞进同一 chunk——正是入站侧标注的「老行为」，
// 同一 codebase 对同一协议事实持两套相反假设。此处把出站形状钉死在规范一侧。
const (
	// 完整 usage + cache/reasoning 明细。
	fixtureResponsesCompletedWithUsageDetails = `data: {"type":"response.completed","response":{"id":"resp_tail","status":"completed","model":"o1","output":[{"type":"output_text","text":"hi"}],"usage":{"input_tokens":100,"output_tokens":42,"total_tokens":142,"input_tokens_details":{"cached_tokens":80},"output_tokens_details":{"reasoning_tokens":7}}}}` + "\n\n" +
		"data: [DONE]\n\n"

	// output 含 function_call → finish_reason 应为 tool_calls，拆包对它同样成立。
	fixtureResponsesCompletedToolCallWithUsage = `data: {"type":"response.completed","response":{"id":"resp_tool","status":"completed","model":"gpt-4","output":[{"type":"function_call","id":"call_0","name":"get_weather","arguments":"{\"loc\":\"BJ\"}"}],"usage":{"input_tokens":10,"output_tokens":8,"total_tokens":18}}}` + "\n\n" +
		"data: [DONE]\n\n"

	// 上游完全不报 usage：不应凭空造出尾包，finish 包就该是末包。
	fixtureResponsesCompletedWithoutUsage = `data: {"type":"response.completed","response":{"id":"resp_nousage","status":"completed","model":"m","output":[{"type":"output_text","text":"hi"}]}}` + "\n\n" +
		"data: [DONE]\n\n"

	// 上游报了 usage 但全零：同样不发尾包——尾包独占一包会被下游读成
	// 「上游明确报告了 token 数」，而全零在仓内既有口径里不是有效观测。
	fixtureResponsesCompletedWithZeroUsage = `data: {"type":"response.completed","response":{"id":"resp_zero","status":"completed","model":"m","output":[{"type":"output_text","text":"hi"}],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}}}` + "\n\n" +
		"data: [DONE]\n\n"

	// anthropic → openai 走 anthropic→responses→openai 双跳，出站形状须与单跳一致。
	fixtureAnthropicStreamForOpenAIOut = `event: message_start
data: {"type":"message_start","message":{"id":"msg_tail","type":"message","role":"assistant","content":[],"model":"claude-test","stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":10,"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`
)

// expectedTailUsage 描述期望出现在尾包里的 usage；nil 表示不应出现尾包。
// cached / reasoning 为 0 表示不应写出对应 details（不产出零值 details）。
type expectedTailUsage struct {
	prompt     int
	completion int
	total      int
	cached     int
	reasoning  int
}

var usageTailCases = []struct {
	name       string
	in         string
	from       consts.WireFormat
	wantFinish string
	wantUsage  *expectedTailUsage
}{
	{
		name:       "openai-res_stop_with_details",
		in:         fixtureResponsesCompletedWithUsageDetails,
		from:       consts.FormatOpenAIResponses,
		wantFinish: "stop",
		wantUsage:  &expectedTailUsage{prompt: 100, completion: 42, total: 142, cached: 80, reasoning: 7},
	},
	{
		name:       "openai-res_tool_calls",
		in:         fixtureResponsesCompletedToolCallWithUsage,
		from:       consts.FormatOpenAIResponses,
		wantFinish: "tool_calls",
		wantUsage:  &expectedTailUsage{prompt: 10, completion: 8, total: 18},
	},
	{
		name:       "anthropic_double_hop",
		in:         fixtureAnthropicStreamForOpenAIOut,
		from:       consts.FormatAnthropic,
		wantFinish: "stop",
		wantUsage:  &expectedTailUsage{prompt: 10, completion: 5, total: 15},
	},
	{
		name:       "no_usage_no_tail_chunk",
		in:         fixtureResponsesCompletedWithoutUsage,
		from:       consts.FormatOpenAIResponses,
		wantFinish: "stop",
		wantUsage:  nil,
	},
	{
		name:       "zero_usage_no_tail_chunk",
		in:         fixtureResponsesCompletedWithZeroUsage,
		from:       consts.FormatOpenAIResponses,
		wantFinish: "stop",
		wantUsage:  nil,
	},
}

// TestResponsesToOpenAI_UsageTailChunk 表驱动锁定出站不变量：
// finish 包不带 usage；有有效 usage 时它独占其后的 choices:[] 末包；无有效 usage 时
// 不凭空造包。覆盖 stop / tool_calls、单跳 / 双跳、无 usage / 全零 usage。
func TestResponsesToOpenAI_UsageTailChunk(t *testing.T) {
	t.Parallel()

	for _, c := range usageTailCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, c.from, "openai")
			finishIdx, usageIdx := locateOpenAIFinishAndUsageChunks(t, events)

			if finishIdx < 0 {
				t.Fatalf("输出流缺少带 finish_reason 的 chunk，共 %d 包", len(events))
			}
			if got := openAIChunkFinishReason(t, events[finishIdx]); got != c.wantFinish {
				t.Errorf("finish_reason = %q，期望 %q", got, c.wantFinish)
			}

			if c.wantUsage == nil {
				if usageIdx >= 0 {
					t.Fatalf("无有效 usage 时不应产出 usage 尾包，却出现在第 %d 包: %v", usageIdx, events[usageIdx].data)
				}
				if finishIdx != len(events)-1 {
					t.Fatalf("无尾包时 finish chunk 应是末包，实际在第 %d 包（共 %d 包）", finishIdx, len(events))
				}
			} else {
				if usageIdx < 0 {
					t.Fatalf("缺少带 usage 的 choices:[] 尾包，共 %d 包", len(events))
				}
				if usageIdx < finishIdx {
					t.Fatalf("usage 尾包（第 %d 包）出现在 finish chunk（第 %d 包）之前", usageIdx, finishIdx)
				}
				if usageIdx != len(events)-1 {
					t.Fatalf("usage 尾包应是流末最后一包，实际在第 %d 包（共 %d 包）", usageIdx, len(events))
				}
				assertTailUsage(t, events[usageIdx].data, c.wantUsage)
			}

			// 拆包不得影响终止语义。
			if n := countSSEDoneMarkers(t, c.in, c.from, "openai"); n != 1 {
				t.Errorf("[DONE] 出现 %d 次，期望恰好 1 次", n)
			}
		})
	}
}

// TestResponsesToOpenAI_UsageTailChunkKeepsStreamMeta 断言尾包仍带 id/object/created/model。
// OpenAI 的 usage 尾包不是裸 usage 对象，而是一个完整 chunk；缺这些字段会让按
// chunk schema 严格解码的客户端 SDK 解析失败。
func TestResponsesToOpenAI_UsageTailChunkKeepsStreamMeta(t *testing.T) {
	t.Parallel()

	events := runRealtimeTransform(t, fixtureResponsesCompletedWithUsageDetails, "openai-res", "openai")
	tail := events[len(events)-1].data

	if _, ok := tail["usage"].(map[string]interface{}); !ok {
		t.Fatalf("流末应为 usage 尾包: %v", tail)
	}
	for _, key := range []string{"id", "object", "created", "model"} {
		if _, ok := tail[key]; !ok {
			t.Errorf("usage 尾包缺少 %q 字段: %v", key, tail)
		}
	}
	if obj, _ := tail["object"].(string); obj != "chat.completion.chunk" {
		t.Errorf("usage 尾包 object = %q，期望 chat.completion.chunk", obj)
	}
}

// TestResponsesToOpenAI_UsageTailChunkSideChannelUnchanged 确认拆包没有动到侧信道：
// 落库侧的 usage 来自 captureUpstreamUsageResponses，与出站 chunk 形状无关，
// 拆包时若漏掉或重复调用它，落库值会丢失或错乱。
func TestResponsesToOpenAI_UsageTailChunkSideChannelUnchanged(t *testing.T) {
	t.Parallel()

	if got := sideChannelTotalTokens(t, fixtureResponsesCompletedWithUsageDetails, "openai-res", "openai"); got != 142 {
		t.Errorf("侧信道 TotalTokens = %d，期望 142", got)
	}
}

// locateOpenAIFinishAndUsageChunks 扫出「带 finish_reason 的包」与「带 usage 的 choices:[] 包」
// 的下标（各取最后一次出现），并顺带断言 finish 包不携带 usage。均返回 -1 表示未出现。
func locateOpenAIFinishAndUsageChunks(t *testing.T, events []sseEvent) (finishIdx, usageIdx int) {
	t.Helper()

	finishIdx, usageIdx = -1, -1
	for i, ev := range events {
		choices := openAIChunkChoices(t, ev)
		_, hasUsage := ev.data["usage"].(map[string]interface{})

		if len(choices) == 0 {
			if hasUsage {
				usageIdx = i
			}
			continue
		}
		if openAIChunkFinishReason(t, ev) != "" {
			finishIdx = i
			if hasUsage {
				t.Fatalf("finish_reason chunk 不应携带 usage（须放在其后的 choices:[] 尾包）: %v", ev.data)
			}
		}
	}
	return finishIdx, usageIdx
}

// assertTailUsage 校验尾包 usage 的三总量与两处 details。
func assertTailUsage(t *testing.T, tail map[string]interface{}, want *expectedTailUsage) {
	t.Helper()

	usage := nestedMap(t, tail, "usage")
	assertNumber(t, usage, "prompt_tokens", want.prompt)
	assertNumber(t, usage, "completion_tokens", want.completion)
	assertNumber(t, usage, "total_tokens", want.total)

	if want.cached > 0 {
		assertNumber(t, nestedMap(t, tail, "usage", "prompt_tokens_details"), "cached_tokens", want.cached)
	} else if _, ok := usage["prompt_tokens_details"]; ok {
		t.Errorf("无 cache 真值时不应写出 prompt_tokens_details: %v", usage)
	}

	if want.reasoning > 0 {
		assertNumber(t, nestedMap(t, tail, "usage", "completion_tokens_details"), "reasoning_tokens", want.reasoning)
	} else if _, ok := usage["completion_tokens_details"]; ok {
		t.Errorf("无 reasoning 真值时不应写出 completion_tokens_details: %v", usage)
	}
}

// openAIChunkChoices 取 OpenAI chat chunk 的 choices 数组。
// 与 sseEvent 上的辅助方法不同：openai 出站流没有 type/event 字段，只能按结构取。
func openAIChunkChoices(t *testing.T, ev sseEvent) []interface{} {
	t.Helper()
	choices, ok := ev.data["choices"].([]interface{})
	if !ok {
		t.Fatalf("chunk 缺少 choices 数组: %v", ev.data)
	}
	return choices
}

// openAIChunkFinishReason 取 choices[0].finish_reason；无值（null）返回空串。
func openAIChunkFinishReason(t *testing.T, ev sseEvent) string {
	t.Helper()

	choices := openAIChunkChoices(t, ev)
	if len(choices) == 0 {
		return ""
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatalf("choices[0] 非对象: %v", ev.data)
	}
	fr, _ := choice["finish_reason"].(string)
	return fr
}

// countSSEDoneMarkers 数出站流里的 [DONE] 标记数量。
// runRealtimeTransform 会跳过 [DONE]，故终止语义只能另跑一遍原始文本来验。
func countSSEDoneMarkers(t *testing.T, sse string, from, to consts.WireFormat) int {
	t.Helper()

	out, err := TransformResponseRealtime(newSSEResponse(sse), from, to, nil)
	if err != nil {
		t.Fatalf("TransformResponseRealtime(%s->%s): %v", from, to, err)
	}
	defer out.Body.Close()

	n := 0
	scanner := bufio.NewScanner(out.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSEEventSize)
	for scanner.Scan() {
		if strings.TrimSpace(strings.TrimPrefix(strings.TrimRight(scanner.Text(), "\r"), "data:")) == "[DONE]" {
			n++
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("读取输出流失败: %v", err)
	}
	_, _ = io.Copy(io.Discard, out.Body)
	return n
}
