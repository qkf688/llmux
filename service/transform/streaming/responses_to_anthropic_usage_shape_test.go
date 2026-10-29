package streaming

import (
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 同一条流里 message_start 与 message_delta 的 usage 必须同一口径。
// 早先 message_start 直写「含缓存总量」且不带 cache_read_input_tokens，而
// message_delta 写「未命中量 + cache_read」——上游在 response.created 就带 usage 时，
// 客户端先看到的那份会把缓存段按全价算一遍（正是本次要修的症状换了个出口）。
func TestResponsesToAnthropicUsage_MessageStartMatchesDelta(t *testing.T) {
	events := runRealtimeTransform(t, responsesSSEUsageInCreated, consts.FormatOpenAIResponses, consts.FormatAnthropic)

	start := findEvent(t, events, "message_start")
	startUsage := nestedMap(t, start.data, "message", "usage")
	assertNumber(t, startUsage, "input_tokens", 20)
	assertNumber(t, startUsage, "output_tokens", 5)
	assertNumber(t, startUsage, "cache_read_input_tokens", 80)

	delta := findEvent(t, events, "message_delta")
	deltaUsage := nestedMap(t, delta.data, "usage")
	assertNumber(t, deltaUsage, "input_tokens", 20)
	assertNumber(t, deltaUsage, "output_tokens", 42)
	assertNumber(t, deltaUsage, "cache_read_input_tokens", 80)
}

// response.created 不带 usage 时仍走 1/1 占位（上游 usage 在末包，首帧没有真值；
// 写 0 会让只读首帧的客户端把用量算成 0）。这条锁住「同口径改造没有把占位吃掉」。
func TestResponsesToAnthropicUsage_NoUsageInCreatedFallsBackToPlaceholder(t *testing.T) {
	events := runRealtimeTransform(t, responsesSSEUsageWithoutCreated, consts.FormatOpenAIResponses, consts.FormatAnthropic)

	start := findEvent(t, events, "message_start")
	startUsage := nestedMap(t, start.data, "message", "usage")
	assertNumber(t, startUsage, "input_tokens", 1)
	assertNumber(t, startUsage, "output_tokens", 1)
	if _, ok := startUsage["cache_read_input_tokens"]; ok {
		t.Fatalf("占位 usage 不应带 cache_read_input_tokens: %v", startUsage)
	}
}

// responsesSSEUsageInCreated 让 response.created 就带上完整 usage（部分兼容网关会这样），
// 用来验证 message_start 与 message_delta 的口径一致。上游 input_tokens=100 已含
// cached 80，故客户端两处都应看到 20 + cache_read 80。
const responsesSSEUsageInCreated = `event: response.created
data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_usage","object":"response","model":"res-test","status":"in_progress","output":[],"usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":80},"output_tokens":5,"total_tokens":105}}}

event: response.output_item.added
data: {"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"in_progress","content":[]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","sequence_number":2,"output_index":0,"content_index":0,"item_id":"msg_1","delta":"hi"}

event: response.output_item.done
data: {"type":"response.output_item.done","sequence_number":3,"output_index":0,"item":{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hi"}]}}

event: response.completed
data: {"type":"response.completed","sequence_number":4,"response":{"id":"resp_usage","object":"response","model":"res-test","status":"completed","output":[],"usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":80},"output_tokens":42,"total_tokens":142}}}

data: [DONE]
`

// responsesSSEUsageWithoutCreated 的 response.created 不带 usage，走 1/1 占位。
const responsesSSEUsageWithoutCreated = `event: response.created
data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_nousage","object":"response","model":"res-test","status":"in_progress","output":[]}}

event: response.output_item.added
data: {"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"type":"message","id":"msg_2","role":"assistant","status":"in_progress","content":[]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","sequence_number":2,"output_index":0,"content_index":0,"item_id":"msg_2","delta":"hi"}

event: response.output_item.done
data: {"type":"response.output_item.done","sequence_number":3,"output_index":0,"item":{"type":"message","id":"msg_2","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hi"}]}}

event: response.completed
data: {"type":"response.completed","sequence_number":4,"response":{"id":"resp_nousage","object":"response","model":"res-test","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":3,"total_tokens":13}}}

data: [DONE]
`
