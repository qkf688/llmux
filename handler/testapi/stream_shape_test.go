package testapi

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/service/credprobe"
	"github.com/tidwall/gjson"
)

func dataFrames(payloads ...string) []credprobe.SSEFrame {
	frames := make([]credprobe.SSEFrame, 0, len(payloads))
	for _, payload := range payloads {
		frames = append(frames, credprobe.SSEFrame{Data: payload})
	}
	return frames
}

func eventFrames(pairs ...[2]string) []credprobe.SSEFrame {
	frames := make([]credprobe.SSEFrame, 0, len(pairs))
	for _, pair := range pairs {
		frames = append(frames, credprobe.SSEFrame{Event: pair[0], Data: pair[1]})
	}
	return frames
}

func openAIDeltaFrame(deltaJSON, finishReason string) string {
	if finishReason == "" {
		return `{"choices":[{"index":0,"delta":` + deltaJSON + `}]}`
	}
	return `{"choices":[{"index":0,"delta":` + deltaJSON + `,"finish_reason":"` + finishReason + `"}]}`
}

// TestFoldStream_OpenAIChatContentAndReasoning 折叠必须把 delta 拼回非流式形状，
// 且**流里出现哪个推理键就写回哪个键**：把 reasoning_content 与 reasoning 互相归一，
// 等于替网关掩盖了真实差异，extractThinking 的双键判定就失去了意义。
func TestFoldStream_OpenAIChatContentAndReasoning(t *testing.T) {
	fold := foldStream(consts.FormatOpenAIChat, dataFrames(
		openAIDeltaFrame(`{"role":"assistant","content":"The "}`, ""),
		openAIDeltaFrame(`{"reasoning_content":"先算 2^10=1024"}`, ""),
		openAIDeltaFrame(`{"content":"image is red."}`, ""),
		openAIDeltaFrame(`{"reasoning_content":"，再减 8 得 1016"}`, ""),
		openAIDeltaFrame(`{}`, "stop"),
		"[DONE]",
	))

	if got := gjson.GetBytes(fold.canonical, "choices.0.message.content").String(); got != "The image is red." {
		t.Fatalf("content = %q, want 拼接后的完整文本", got)
	}
	if got := gjson.GetBytes(fold.canonical, "choices.0.message.reasoning_content").String(); got != "先算 2^10=1024，再减 8 得 1016" {
		t.Fatalf("reasoning_content = %q", got)
	}
	if gjson.GetBytes(fold.canonical, "choices.0.message.reasoning").Exists() {
		t.Fatal("上游没发 reasoning 键，折叠不得凭空造出来")
	}
	if got := gjson.GetBytes(fold.canonical, "choices.0.finish_reason").String(); got != "stop" {
		t.Fatalf("finish_reason = %q, want stop", got)
	}
	if fold.terminatedBy != streamTerminalDone || !fold.hasContentDelta {
		t.Fatalf("fold = %+v, want [DONE] + 有内容", fold)
	}
}

func TestFoldStream_OpenAIChatReasoningKeyPassthrough(t *testing.T) {
	fold := foldStream(consts.FormatOpenAIChat, dataFrames(
		openAIDeltaFrame(`{"reasoning":"step by step"}`, ""),
		openAIDeltaFrame(`{"content":"1016"}`, "stop"),
		"[DONE]",
	))
	if got := gjson.GetBytes(fold.canonical, "choices.0.message.reasoning").String(); got != "step by step" {
		t.Fatalf("reasoning = %q", got)
	}
	if gjson.GetBytes(fold.canonical, "choices.0.message.reasoning_content").Exists() {
		t.Fatal("不得把 reasoning 改名成 reasoning_content")
	}
}

// TestFoldStream_OpenAIChatToolCallArguments 结构化输出在部分兼容网关上走 tool_calls，
// arguments 是分片 delta，必须按 tool_call index 分片拼接而不是互相覆盖。
func TestFoldStream_OpenAIChatToolCallArguments(t *testing.T) {
	fold := foldStream(consts.FormatOpenAIChat, dataFrames(
		openAIDeltaFrame(`{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"structured_output","arguments":""}}]}`, ""),
		openAIDeltaFrame(`{"tool_calls":[{"index":0,"function":{"arguments":"{\"language\":\"zh\""}}]}`, ""),
		openAIDeltaFrame(`{"tool_calls":[{"index":0,"function":{"arguments":",\"version\":1}"}}]}`, ""),
		openAIDeltaFrame(`{}`, "tool_calls"),
		"[DONE]",
	))

	if got := gjson.GetBytes(fold.canonical, "choices.0.message.tool_calls.0.function.name").String(); got != "structured_output" {
		t.Fatalf("name = %q", got)
	}
	if got := gjson.GetBytes(fold.canonical, "choices.0.message.tool_calls.0.id").String(); got != "call_1" {
		t.Fatalf("id = %q", got)
	}
	arguments := gjson.GetBytes(fold.canonical, "choices.0.message.tool_calls.0.function.arguments").String()
	if !json.Valid([]byte(arguments)) {
		t.Fatalf("arguments 拼不回合法 JSON: %q", arguments)
	}
	if !fold.hasContentDelta {
		t.Fatal("tool 参数分片也是内容增量（否则纯工具调用的流会被判「无内容」）")
	}
}

// TestFoldStream_AnthropicThinkingAndTextBlocks Anthropic 的块靠 index 定序：thinking 在前、
// text 在后，且 signature_delta 是 thinking 块的一部分，丢了就不等价。
func TestFoldStream_AnthropicThinkingAndTextBlocks(t *testing.T) {
	fold := foldStream(consts.FormatAnthropic, eventFrames(
		[2]string{"message_start", `{"type":"message_start","message":{"id":"msg_1","content":[]}}`},
		[2]string{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"2^10=1024"}}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig_abc"}}`},
		[2]string{"content_block_stop", `{"type":"content_block_stop","index":0}`},
		[2]string{"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"答案是 1016"}}`},
		[2]string{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`},
		[2]string{"message_stop", `{"type":"message_stop"}`},
	))

	blocks := gjson.GetBytes(fold.canonical, "content").Array()
	if len(blocks) != 2 {
		t.Fatalf("content 块数 = %d, want 2: %s", len(blocks), fold.canonical)
	}
	if blocks[0].Get("type").String() != "thinking" || blocks[0].Get("thinking").String() != "2^10=1024" {
		t.Fatalf("thinking 块折叠错: %s", blocks[0].Raw)
	}
	if blocks[0].Get("signature").String() != "sig_abc" {
		t.Fatalf("signature 丢了: %s", blocks[0].Raw)
	}
	if blocks[1].Get("type").String() != "text" || blocks[1].Get("text").String() != "答案是 1016" {
		t.Fatalf("text 块折叠错: %s", blocks[1].Raw)
	}
	if got := gjson.GetBytes(fold.canonical, "stop_reason").String(); got != "end_turn" {
		t.Fatalf("stop_reason = %q", got)
	}
	if fold.terminatedBy != streamTerminalMessageStop || !fold.hasContentDelta {
		t.Fatalf("fold = %+v", fold)
	}

	// 折叠产物必须能直接喂给既有 extractor——这是「通过标准只有一份」的机械证明。
	thinkingText, err := extractThinking(consts.FormatAnthropic, fold.canonical)
	if err != nil || thinkingText != "2^10=1024" {
		t.Fatalf("extractThinking(折叠产物) = (%q, %v)", thinkingText, err)
	}
}

// TestFoldStream_AnthropicToolUsePartialJSON 结构化输出的 Anthropic 形状是 tool_use：
// input 在流里是 partial_json 碎片，折叠后必须是**对象**而非字符串，
// 否则 extractStructuredOutputJSON 取到的 .Raw 不是合法 JSON 值。
func TestFoldStream_AnthropicToolUsePartialJSON(t *testing.T) {
	payload := `{"language":"zh","version":1,"features":["a"],"score":1,"passed":true}`
	fold := foldStream(consts.FormatAnthropic, eventFrames(
		[2]string{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tool_1","name":"structured_output","input":{}}}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"language\":\"zh\","}}`},
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"version\":1,\"features\":[\"a\"],\"score\":1,\"passed\":true}"}}`},
		[2]string{"message_stop", `{"type":"message_stop"}`},
	))

	gotInput := gjson.GetBytes(fold.canonical, `content.0.input`)
	if gotInput.Type != gjson.JSON {
		t.Fatalf("tool_use.input 不是对象: %s", gotInput.Raw)
	}

	gotPayload, _, err := extractStructuredOutputJSON(consts.FormatAnthropic, fold.canonical)
	if err != nil {
		t.Fatalf("extractStructuredOutputJSON(折叠产物) 失败: %v", err)
	}
	if string(gotPayload) != payload {
		t.Fatalf("结构化输出 = %s, want %s", gotPayload, payload)
	}
}

// TestFoldStream_EventNameFallback 部分中转不发 event: 行，只在 data 里带 type：
// 事件名回落到载荷的 type 字段，否则这类上游的流式测试恒判「缺终止事件」。
func TestFoldStream_EventNameFallback(t *testing.T) {
	fold := foldStream(consts.FormatAnthropic, dataFrames(
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"红"}}`,
		`{"type":"message_stop"}`,
	))
	if fold.terminatedBy != streamTerminalMessageStop {
		t.Fatalf("terminatedBy = %q, want message_stop", fold.terminatedBy)
	}
	if got := gjson.GetBytes(fold.canonical, "content.0.text").String(); got != "红" {
		t.Fatalf("text = %q", got)
	}
	// 与既有 extractor 同口径：折叠产物喂给 vision 的文本提取应拿到答案。
	visionText, err := extractVisionText(consts.FormatAnthropic, fold.canonical)
	if err != nil || visionText != "红" {
		t.Fatalf("extractVisionText(折叠产物) = (%q, %v)", visionText, err)
	}
}

// TestFoldStream_OpenAIResponsesUsesCompletedPayloadVerbatim Responses 的终止帧本身就带
// 完整非流式响应体，直接取它——省掉一整条 delta 映射分支（方案的实质性简化，别再改回去）。
func TestFoldStream_OpenAIResponsesUsesCompletedPayloadVerbatim(t *testing.T) {
	completedResponse := `{"id":"resp_1","status":"completed","model":"gpt-5-nano","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"1024-8=1016"}]},{"type":"message","content":[{"type":"output_text","text":"1016"}]}]}`
	fold := foldStream(consts.FormatOpenAIResponses, eventFrames(
		[2]string{"response.created", `{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`},
		[2]string{"response.output_text.delta", `{"type":"response.output_text.delta","delta":"1016"}`},
		[2]string{"response.completed", `{"type":"response.completed","response":` + completedResponse + `}`},
	))

	var completed, canonical any
	if err := json.Unmarshal([]byte(completedResponse), &completed); err != nil {
		t.Fatalf("测试数据本身不是合法 JSON: %v", err)
	}
	if err := json.Unmarshal(fold.canonical, &canonical); err != nil {
		t.Fatalf("canonical 不是合法 JSON: %s (%v)", fold.canonical, err)
	}
	if !reflect.DeepEqual(completed, canonical) {
		t.Fatalf("canonical 应与 response.completed.response 逐字段相等:\n got %s\nwant %s", fold.canonical, completedResponse)
	}
	if fold.terminatedBy != streamTerminalResponseCompleted || !fold.hasContentDelta {
		t.Fatalf("fold = %+v", fold)
	}

	thinkingText, err := extractThinking(consts.FormatOpenAIResponses, fold.canonical)
	if err != nil || thinkingText != "1024-8=1016" {
		t.Fatalf("extractThinking(折叠产物) = (%q, %v)", thinkingText, err)
	}
}

// TestFoldStream_OpenAIResponsesWithoutCompletedFrame 只收到 created/delta 就 EOF：
// 没有可判定的完整响应体，canonical 必须是空数组形状而不是「半条流装成完整响应」。
func TestFoldStream_OpenAIResponsesWithoutCompletedFrame(t *testing.T) {
	fold := foldStream(consts.FormatOpenAIResponses, eventFrames(
		[2]string{"response.output_text.delta", `{"type":"response.output_text.delta","delta":"1016"}`},
	))
	if fold.terminatedBy != streamTerminalNone {
		t.Fatalf("terminatedBy = %q, want none", fold.terminatedBy)
	}
	if err := validateStreamFold(fold); !errors.Is(err, errStreamNotTerminated) {
		t.Fatalf("validateStreamFold = %v, want errStreamNotTerminated", err)
	}
}

func TestFoldStream_ResponseFailedAndIncompleteAreTerminals(t *testing.T) {
	failed := foldStream(consts.FormatOpenAIResponses, eventFrames(
		[2]string{"response.failed", `{"type":"response.failed","response":{"status":"failed","error":{"message":"upstream blew up"}}}`},
	))
	if failed.terminatedBy != streamTerminalResponseFailed {
		t.Fatalf("terminatedBy = %q", failed.terminatedBy)
	}
	if failed.streamError == "" || !strings.Contains(failed.streamError, "upstream blew up") {
		t.Fatalf("streamError = %q, want 带上游 error.message", failed.streamError)
	}
	if err := validateStreamFold(failed); err == nil || !strings.Contains(err.Error(), "upstream blew up") {
		t.Fatalf("validateStreamFold = %v, want 报出流内错误", err)
	}

	incomplete := foldStream(consts.FormatOpenAIResponses, eventFrames(
		[2]string{"response.incomplete", `{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`},
	))
	if err := validateStreamFold(incomplete); err == nil || !strings.Contains(err.Error(), "max_output_tokens") {
		t.Fatalf("validateStreamFold = %v, want 报出不完整原因", err)
	}
}

// TestValidateStreamFold_Gates 严格门的三条失败路径各自独立可辨：文案不同，
// 管理员才能分清「上游不支持流」「流被切断」「上游不肯吐字」三件事。
func TestValidateStreamFold_Gates(t *testing.T) {
	cases := []struct {
		name    string
		fold    streamFold
		wantErr error
	}{
		{"全部满足", streamFold{terminatedBy: streamTerminalDone, hasContentDelta: true}, nil},
		{"缺终止事件", streamFold{terminatedBy: streamTerminalNone, hasContentDelta: true}, errStreamNotTerminated},
		{"有终止但无内容", streamFold{terminatedBy: streamTerminalDone, hasContentDelta: false}, errStreamNoContentDelta},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStreamFold(tc.fold)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("应通过; got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// TestFoldStream_OpenAIChatInStreamErrorFrame 兼容网关常见套路：HTTP 200 + event-stream，
// 中途塞一条 {"error":{...}} 再 [DONE]。若不当成错误，extractThinking 只会报
// 「未返回推理内容」，把上游故障伪装成模型能力问题。
func TestFoldStream_OpenAIChatInStreamErrorFrame(t *testing.T) {
	fold := foldStream(consts.FormatOpenAIChat, dataFrames(
		openAIDeltaFrame(`{"content":"par"}`, ""),
		`{"error":{"message":"engine_overloaded"}}`,
		"[DONE]",
	))
	if fold.streamError == "" || !strings.Contains(fold.streamError, "engine_overloaded") {
		t.Fatalf("streamError = %q, want 捕获流内 error 帧", fold.streamError)
	}
	err := validateStreamFold(fold)
	if err == nil || !strings.Contains(err.Error(), "engine_overloaded") {
		t.Fatalf("validateStreamFold = %v, want 优先报流内错误", err)
	}
}

// TestFoldStream_AnthropicErrorEvent Anthropic 的 error 事件形状与 OpenAI 帧不同，单独锁。
func TestFoldStream_AnthropicErrorEvent(t *testing.T) {
	fold := foldStream(consts.FormatAnthropic, eventFrames(
		[2]string{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"x"}}`},
		[2]string{"error", `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`},
	))
	if err := validateStreamFold(fold); err == nil || !strings.Contains(err.Error(), "Overloaded") {
		t.Fatalf("validateStreamFold = %v", err)
	}
}

// TestFoldStream_IgnoresNoiseFrames 心跳、畸形 JSON、未知事件都不能让折叠崩掉，
// 也不能被当成内容增量（否则 ping 会把「上游啥也没答」洗成有内容）。
func TestFoldStream_IgnoresNoiseFrames(t *testing.T) {
	fold := foldStream(consts.FormatOpenAIChat, dataFrames(
		``,
		`not json at all`,
		`{"choices":[{"index":0,"delta":{"content":""}}]}`,
		`{"usage":{"prompt_tokens":1}}`,
		"[DONE]",
	))
	if fold.hasContentDelta {
		t.Fatal("空 delta / 畸形帧 / 纯 usage 帧都不算内容增量")
	}
	if fold.terminatedBy != streamTerminalDone {
		t.Fatalf("terminatedBy = %q", fold.terminatedBy)
	}
	if err := validateStreamFold(fold); !errors.Is(err, errStreamNoContentDelta) {
		t.Fatalf("validateStreamFold = %v, want 无内容增量", err)
	}
}

// TestFoldStream_OpenAIChatNoTerminalKeepsPartialContent 缺终止事件时折叠产物仍要保留
// 已收到的内容（raw_output 之外，判定文本也有价值），但严格门必须判失败。
func TestFoldStream_OpenAIChatNoTerminalKeepsPartialContent(t *testing.T) {
	fold := foldStream(consts.FormatOpenAIChat, dataFrames(
		openAIDeltaFrame(`{"content":"断了半条流"}`, ""),
	))
	if fold.terminatedBy != streamTerminalNone {
		t.Fatalf("terminatedBy = %q", fold.terminatedBy)
	}
	if got := gjson.GetBytes(fold.canonical, "choices.0.message.content").String(); got != "断了半条流" {
		t.Fatalf("已收到的内容被丢了: %q", got)
	}
	if err := validateStreamFold(fold); !errors.Is(err, errStreamNotTerminated) {
		t.Fatalf("validateStreamFold = %v", err)
	}
}

// TestStreamStatsPayload 回包诊断块的数据源：键名是前端契约，逐键锁死。
func TestStreamStatsPayload(t *testing.T) {
	fold := streamFold{terminatedBy: streamTerminalDone, hasContentDelta: true}
	probe := credprobe.StreamProbeResult{Frames: dataFrames("a", "b", "c"), FirstFrameMS: 120, DurationMS: 3400, Framed: true}

	stats := streamStatsPayload(fold, probe)
	if stats["frames"] != 3 || stats["first_frame_ms"] != int64(120) || stats["duration_ms"] != int64(3400) {
		t.Fatalf("stats = %+v", stats)
	}
	if stats["terminated_by"] != streamTerminalDone {
		t.Fatalf("terminated_by = %v", stats["terminated_by"])
	}

	encoded, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("stats 必须可序列化: %v", err)
	}
	for _, key := range []string{"frames", "first_frame_ms", "duration_ms", "terminated_by"} {
		if !gjson.GetBytes(encoded, key).Exists() {
			t.Fatalf("回包缺键 %s: %s", key, encoded)
		}
	}
}

// TestFoldStream_OpenAIChatArrayContent 个别多模态网关把 delta.content 回成
// [{type:"text",text:...}] 数组；extractVisionText 两种形状都收，折叠层只收字符串
// 就会让这类网关的流式测试稳定假失败（正是本功能要抓的渠道）。
func TestFoldStream_OpenAIChatArrayContent(t *testing.T) {
	fold := foldStream(consts.FormatOpenAIChat, dataFrames(
		`{"choices":[{"index":0,"delta":{"content":[{"type":"text","text":"The image is "}]}]}`,
		`{"choices":[{"index":0,"delta":{"content":[{"type":"text","text":"red."}]},"finish_reason":"stop"}]}`,
		"[DONE]",
	))
	if got := gjson.GetBytes(fold.canonical, "choices.0.message.content").String(); got != "The image is red." {
		t.Fatalf("数组形态 delta.content 未被折叠: %q (canonical=%s)", got, fold.canonical)
	}
	if !fold.hasContentDelta {
		t.Fatal("数组形态正文也是内容增量")
	}
	if text, err := extractVisionText(consts.FormatOpenAIChat, fold.canonical); err != nil || !visionAnswerMatches(text) {
		t.Fatalf("extractVisionText(折叠产物) = (%q, %v)", text, err)
	}
}

// TestFoldStream_AnthropicContentBlockStartCarriesInitialContent 部分中转把整段答案
// 直接放进 content_block_start 且不再发 delta：漏收会把「上游其实答了」判成无内容增量。
func TestFoldStream_AnthropicContentBlockStartCarriesInitialContent(t *testing.T) {
	fold := foldStream(consts.FormatAnthropic, eventFrames(
		[2]string{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"2^10=1024，减 8 得 1016"}}`},
		[2]string{"content_block_stop", `{"type":"content_block_stop","index":0}`},
		[2]string{"message_stop", `{"type":"message_stop"}`},
	))
	if !fold.hasContentDelta {
		t.Fatalf("start 块里的初始内容没算增量: %+v", fold)
	}
	text, err := extractThinking(consts.FormatAnthropic, fold.canonical)
	if err != nil || text != "2^10=1024，减 8 得 1016" {
		t.Fatalf("extractThinking(折叠产物) = (%q, %v); canonical=%s", text, err, fold.canonical)
	}
}

// TestStreamFailureType 三类严格门失败的 error_type 必须与非流式路径同型：
// 流内上游错误是渠道故障（provider），缺终止/无内容是协议与能力问题（validation）。
// 混成一个 validation 会让同一个过载故障换个模式就换个病因。
func TestStreamFailureType(t *testing.T) {
	cases := []struct {
		name        string
		fold        streamFold
		wantType    string
		wantMessage string
	}{
		{"流内错误帧", streamFold{terminatedBy: streamTerminalDone, hasContentDelta: true, streamError: "Overloaded"}, common.ErrorTypeProvider, "Overloaded"},
		{"缺终止事件", streamFold{terminatedBy: streamTerminalNone, hasContentDelta: true}, common.ErrorTypeValidation, "未收到终止事件"},
		{"无内容增量", streamFold{terminatedBy: streamTerminalDone}, common.ErrorTypeValidation, "内容增量"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStreamFold(tc.fold)
			if err == nil {
				t.Fatal("应返回失败")
			}
			if !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("err = %v, want 含 %q", err, tc.wantMessage)
			}
			if got := streamFailureType(err); got != tc.wantType {
				t.Fatalf("streamFailureType = %q, want %q", got, tc.wantType)
			}
		})
	}
}

// TestQueryStreamMode 「缺省即非流式」是向后兼容的唯一保证：老前端不带参数时
// 行为必须与改造前逐字节一致。
func TestQueryStreamMode(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"1", "true", "TRUE", "t"} {
		if !parseStreamMode(raw) {
			t.Fatalf("stream=%s 应判流式", raw)
		}
	}
	for _, raw := range []string{"", "0", "false", "banana"} {
		if parseStreamMode(raw) {
			t.Fatalf("stream=%q 应判非流式", raw)
		}
	}
}
