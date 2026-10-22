package streaming

import (
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 本文件覆盖「末帧同时带 delta 与 finish_reason」这一形态。
//
// 修复「末包不带 delta 键」时，finish 判定被提到 delta 断言之前并直接 return——
// 这修好了「无 delta 键」的形态，却把「delta 与 finish 同帧」的增量整段吃掉了：
// 上游把最后一截正文/工具参数与结束标记合在同一个 chunk 是常见写法（本仓
// handler/testapi 与 service/credprobe 的测试服务端都是这种形状）。
//
// 正确语义：finish 判定不能挡在 delta 断言之后（否则无 delta 键的帧被丢），
// 但也不能拦在 delta 处理之前（否则同帧增量被丢）。两者不冲突——先处理
// delta（缺失时按空增量），再判 finish。

const (
	// fixtureOpenAIContentWithFinish：末帧同时带 content 与 finish_reason。
	fixtureOpenAIContentWithFinish = `data: {"id":"chatcmpl-cf","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"前半"},"finish_reason":null}]}

data: {"id":"chatcmpl-cf","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"后半"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}

data: [DONE]

`

	// fixtureOpenAIToolCallWithFinish：末帧同时带 tool_calls 参数与 finish_reason。
	fixtureOpenAIToolCallWithFinish = `data: {"id":"chatcmpl-tcf","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_x","type":"function","function":{"name":"run_code","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-tcf","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"code\":\"print(1)\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}

data: [DONE]

`

	// fixtureOpenAISingleFrameContentWithFinish：整条流只有一帧，同时带 content 与 finish。
	fixtureOpenAISingleFrameContentWithFinish = `data: {"id":"chatcmpl-sf","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"只有这一帧"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}

data: [DONE]

`
)

// TestOpenAIResponses_DeltaAndFinishInSameChunk 断言末帧携带的增量不被丢弃。
//
// 判定依据是「最终正文」与「arguments 终值」，不依赖事件条数——无论实现在
// 收尾前还是收尾后处理该帧，只要内容完整就算对。
func TestOpenAIResponses_DeltaAndFinishInSameChunk(t *testing.T) {
	t.Parallel()

	t.Run("content_with_finish", func(t *testing.T) {
		t.Parallel()

		events := runRealtimeTransform(t, fixtureOpenAIContentWithFinish,
			consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

		done := findEvent(t, events, "response.output_text.done")
		if text, _ := done.data["text"].(string); text != "前半后半" {
			t.Errorf("output_text.done.text = %q，期望「前半后半」（末帧的「后半」被丢弃）", text)
		}

		completed := findEvent(t, events, "response.completed")
		output, _ := nestedMap(t, completed.data, "response")["output"].([]interface{})
		if len(output) == 0 {
			t.Fatal("response.completed.output 为空，正文整段丢失")
		}
	})

	t.Run("single_frame_content_with_finish", func(t *testing.T) {
		t.Parallel()

		events := runRealtimeTransform(t, fixtureOpenAISingleFrameContentWithFinish,
			consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

		done := findEvent(t, events, "response.output_text.done")
		if text, _ := done.data["text"].(string); text != "只有这一帧" {
			t.Errorf("单帧流的 output_text.done.text = %q，期望「只有这一帧」", text)
		}
	})

	t.Run("tool_call_with_finish", func(t *testing.T) {
		t.Parallel()

		events := runRealtimeTransform(t, fixtureOpenAIToolCallWithFinish,
			consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

		argsDone := findEvent(t, events, "response.function_call_arguments.done")
		args, _ := argsDone.data["arguments"].(string)
		if args != `{"code":"print(1)"}` {
			t.Errorf("arguments.done = %q，期望完整参数（末帧参数被丢弃）", args)
		}

		completed := findEvent(t, events, "response.completed")
		output, _ := nestedMap(t, completed.data, "response")["output"].([]interface{})
		found := false
		for _, raw := range output {
			if item, ok := raw.(map[string]interface{}); ok {
				if typ, _ := item["type"].(string); typ == "function_call" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("response.completed.output 里没有 function_call 项，工具调用整段丢失: %v", output)
		}
	})
}
