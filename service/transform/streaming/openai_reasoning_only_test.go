package streaming

import (
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 本文件覆盖「只给 reasoning_content、不给 role/content」的流。
//
// 这是推理模型 + 工具调用前先思考的常见形态（先思考、随后直接调工具或结束）。
// 该形态下 message 项从未开出，而 reasoning 的收尾事件曾写在 message 分支内部——
// 被 `!messageStreamStarted` 的早退挡在后面，表现为：reasoning 项开了不关、
// 且 response.completed.output 里完全没有 reasoning（内容整段消失）。

const (
	// fixtureOpenAIReasoningOnlyWithFinish：只有 reasoning，随后 finish_reason=stop。
	fixtureOpenAIReasoningOnlyWithFinish = `data: {"id":"chatcmpl-ro","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"reasoning_content":"先想一想"},"finish_reason":null}]}

data: {"id":"chatcmpl-ro","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"reasoning_content":"再想一想"},"finish_reason":null}]}

data: {"id":"chatcmpl-ro","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}

data: [DONE]

`

	// fixtureOpenAIReasoningOnlyWithToolCall：只有 reasoning，随后直接调工具。
	fixtureOpenAIReasoningOnlyWithToolCall = `data: {"id":"chatcmpl-rot","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"reasoning_content":"需要跑代码"},"finish_reason":null}]}

data: {"id":"chatcmpl-rot","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_r","type":"function","function":{"name":"run_code","arguments":"{\"code\":\"1\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}

data: [DONE]

`
)

var openAIReasoningOnlyFixtures = []struct {
	name string
	in   string
}{
	{name: "reasoning_only_with_finish", in: fixtureOpenAIReasoningOnlyWithFinish},
	{name: "reasoning_only_with_tool_call", in: fixtureOpenAIReasoningOnlyWithToolCall},
}

// TestOpenAIResponses_ReasoningOnlyClosesItsItem 断言只有 reasoning 的流里，
// reasoning 项仍被正常关闭（每个 added 都有配对的 done）。
func TestOpenAIResponses_ReasoningOnlyClosesItsItem(t *testing.T) {
	t.Parallel()

	for _, c := range openAIReasoningOnlyFixtures {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

			added := map[int]string{}
			doneCount := map[int]int{}
			for _, ev := range events {
				switch ev.typ() {
				case "response.output_item.added":
					item, _ := ev.data["item"].(map[string]interface{})
					itemType, _ := item["type"].(string)
					added[ev.outputIndex(t)] = itemType
				case "response.output_item.done":
					doneCount[ev.outputIndex(t)]++
				}
			}

			sawReasoning := false
			for idx, itemType := range added {
				if itemType == "reasoning" {
					sawReasoning = true
				}
				if doneCount[idx] == 0 {
					t.Errorf("output_index %d（%s）开了但没关；事件序列: %v",
						idx, itemType, eventTypes(events))
				}
			}
			if !sawReasoning {
				t.Fatalf("未产出 reasoning 项；事件序列: %v", eventTypes(events))
			}
		})
	}
}

// TestOpenAIResponses_ReasoningOnlySurvivesInCompletedOutput 断言 reasoning 内容
// 出现在 response.completed.output 里——不需要正文也不能丢思考过程。
func TestOpenAIResponses_ReasoningOnlySurvivesInCompletedOutput(t *testing.T) {
	t.Parallel()

	for _, c := range openAIReasoningOnlyFixtures {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			events := runRealtimeTransform(t, c.in, consts.FormatOpenAIChat, consts.FormatOpenAIResponses)

			completed := findEvent(t, events, "response.completed")
			output, _ := nestedMap(t, completed.data, "response")["output"].([]interface{})

			found := false
			for _, raw := range output {
				item, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if typ, _ := item["type"].(string); typ == "reasoning" {
					found = true
					summary, _ := item["summary"].([]interface{})
					if len(summary) == 0 {
						t.Errorf("reasoning 项的 summary 为空: %#v", item)
					}
				}
			}
			if !found {
				t.Errorf("response.completed.output 里没有 reasoning 项，思考内容丢失: %v", output)
			}
		})
	}
}
