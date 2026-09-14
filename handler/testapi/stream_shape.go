package testapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/service/credprobe"
	"github.com/tidwall/gjson"
)

// 本文件是「流式模式」的协议语义层：把 credprobe 交回的 SSE 帧折叠成**非流式响应体形状**，
// 使 extractThinking / extractVisionText / extractStructuredOutputJSON 原封复用。
//
// 为什么折叠而不是另写一套流式判定：通过标准只能有一份。两套判定各跑一次意味着
// 「非流式说支持、流式说不支持」这类结论永远查不清是哪一套错。
//
// 折叠与判定函数（foldStream / validateStreamFold 及其分支）均为纯函数：输入帧 → 输出结构，
// 无 IO、不碰全局状态，故可直接表驱动测试。文件末尾的模式参数解析与回包形状 helpers 读
// gin 请求态，不在此列。

// 终止标记值域（回包 stream.terminated_by 的取值，前端按它给结论）。
const (
	streamTerminalDone               = "[DONE]"
	streamTerminalMessageStop        = "message_stop"
	streamTerminalResponseCompleted  = "response.completed"
	streamTerminalResponseFailed     = "response.failed"
	streamTerminalResponseIncomplete = "response.incomplete"
	streamTerminalNone               = "none"
)

// 严格门的失败事实各自成哨兵错误：管理员要能分清「流被切断」「上游不肯吐字」与
// 「上游在流里报了故障」，合并成一条文案就等于把三类不同的渠道缺陷混成一锅；
// error_type 归类也按这三类分（见 streamFailureType）。
var (
	errStreamNotTerminated  = errors.New("流式响应未收到终止事件，可能被上游提前切断")
	errStreamNoContentDelta = errors.New("流式响应未包含任何内容增量")
	errStreamUpstream       = errors.New("流内返回上游错误")
)

// 测试执行模式的回包值域（mode 键恒在，不给「键缺失」留语义——同 respond.go 对 group_id 的立场）。
const (
	testModeStream    = "stream"
	testModeNonStream = "non_stream"
)

func testModeOf(stream bool) string {
	if stream {
		return testModeStream
	}
	return testModeNonStream
}

// modeSuffix 成功文案的模式后缀：「通过」二字不能含糊——流式通过与非流式通过是两件事，
// 而弹窗里只会看到这一行结论文本。
func modeSuffix(stream bool) string {
	if stream {
		return "（流式）"
	}
	return ""
}

// streamModeParam 模式开关的查询参数名（同一 GET 承载两种模式，不新增路由）。
const streamModeParam = "stream"

// streamFold 一次流式折叠的产物：判定所需的全部事实都在这里面（只解析一遍帧）。
type streamFold struct {
	// canonical 是非流式形状的响应体，直接喂给既有 extractor。未收到可判定的终止帧时为空。
	canonical []byte
	// terminatedBy 见上面值域；streamTerminalNone 表示读到 EOF 也没见到终止标记。
	terminatedBy string
	// hasContentDelta 是否出现过非空内容增量（文本 / 推理 / tool 参数）。
	// 刻意不把 ping、usage 尾包、空 delta 算作内容：否则心跳能把「上游啥也没答」洗成有内容。
	hasContentDelta bool
	// streamError 流内上游错误：HTTP 200 + event-stream，但中途回了 error 帧 /
	// response.failed。不单独记下来，就会被 extractor 报成「模型未返回内容」，
	// 把上游故障伪装成模型能力问题。
	streamError string
}

// foldStream 按上游响应体形状折叠 SSE 帧。判定用 wire format 而非 provider type，
// 与三个 extractor 同口径（按「哪一家」分支会让每接一家新上游都要来这里加 case）。
func foldStream(upstreamFormat consts.WireFormat, frames []credprobe.SSEFrame) streamFold {
	switch upstreamFormat {
	case consts.FormatAnthropic:
		return foldAnthropicStream(frames)
	case consts.FormatOpenAIResponses:
		return foldResponsesStream(frames)
	default:
		return foldOpenAIChatStream(frames)
	}
}

// validateStreamFold 严格门：流内错误优先（它是根因），其次终止、最后内容。
// 「协议合规」（Content-Type）这一门在 credprobe.RunStreamProbe 里已经拦掉了。
func validateStreamFold(fold streamFold) error {
	if fold.streamError != "" {
		return fmt.Errorf("%w: %s", errStreamUpstream, fold.streamError)
	}
	if fold.terminatedBy == streamTerminalNone {
		return errStreamNotTerminated
	}
	if !fold.hasContentDelta {
		return errStreamNoContentDelta
	}
	return nil
}

// streamFailureType 把严格门的三类失败映到 error_type：流内上游错误是**上游故障**
// （与非流式路径下同一故障的状态码归类同为 provider），缺终止事件/无内容才是渠道的
// 协议与能力问题（validation）。不区分会让同一个过载故障在非流式弹窗归 provider、
// 流式弹窗归 validation——用户会以为换了模式就换了病因。
func streamFailureType(err error) string {
	if errors.Is(err, errStreamUpstream) {
		return common.ErrorTypeProvider
	}
	return common.ErrorTypeValidation
}

// streamStatsPayload 回包里的流式诊断块。键名是前后端契约（webui ModelProviderTestResult.stream）。
func streamStatsPayload(fold streamFold, probe credprobe.StreamProbeResult) map[string]any {
	return map[string]any{
		"frames":         len(probe.Frames),
		"first_frame_ms": probe.FirstFrameMS,
		"duration_ms":    probe.DurationMS,
		"terminated_by":  fold.terminatedBy,
	}
}

// parseStreamMode 纯函数：?stream 参数值 → 是否流式。解析不动按非流式（默认安全，
// 老前端不带参数时行为与改造前逐字节一致）。
func parseStreamMode(raw string) bool {
	stream, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return stream
}

// queryStreamMode 请求侧的薄封装（四个端点各自解析必漂移，故收敛在此）。
func queryStreamMode(c *gin.Context) bool {
	return parseStreamMode(c.Query(streamModeParam))
}

// eventNameOf 帧的事件名：event: 行优先，缺失时回落到载荷的 type 字段——
// 部分中转平台不发 event: 行，回落之后这类上游才可能被判出终止事件。
func eventNameOf(frame credprobe.SSEFrame) string {
	if frame.Event != "" {
		return frame.Event
	}
	return gjson.Get(frame.Data, "type").String()
}

// --- OpenAI Chat Completions ---

type openAIChatToolCallAccumulator struct {
	id          string
	callType    string
	name        string
	arguments   strings.Builder
	argumentsOK bool
}

type openAIChatStreamAccumulator struct {
	role         string
	content      strings.Builder
	reasoning    strings.Builder
	reasoningCtn strings.Builder
	// 两个推理键各自记「上游到底发没发」：写回时只写出现过的键，不互相归一。
	reasoningSeen    bool
	reasoningCtnSeen bool
	finishReason     string
	toolCalls        map[int]*openAIChatToolCallAccumulator
}

// foldOpenAIChatStream 累加 choices[0] 的 delta（探针体是单轮单 choice，多 choice 无判定价值）。
func foldOpenAIChatStream(frames []credprobe.SSEFrame) streamFold {
	fold := streamFold{terminatedBy: streamTerminalNone}
	acc := &openAIChatStreamAccumulator{toolCalls: map[int]*openAIChatToolCallAccumulator{}}

	for _, frame := range frames {
		if strings.TrimSpace(frame.Data) == streamTerminalDone {
			fold.terminatedBy = streamTerminalDone
			continue
		}
		payload := gjson.Parse(frame.Data)
		if !payload.IsObject() {
			continue // 畸形/裸行帧：不算内容，判定文本靠 raw_output 排查
		}
		if errText := streamErrorFromPayload(payload); errText != "" {
			fold.streamError = errText
			continue
		}

		choice := payload.Get("choices.0")
		if !choice.Exists() {
			continue // usage 尾包等
		}
		delta := choice.Get("delta")
		// content 正常是字符串，个别多模态网关回成 [{type:"text",text:...}] 数组：
		// extractVisionText 两种形状都收，折叠层必须同样收，否则这类网关的 vision
		// 流式测试会稳定判「无内容增量」——假阴性恰好落在本功能要检出的渠道上。
		switch content := delta.Get("content"); content.Type {
		case gjson.String:
			if text := content.String(); text != "" {
				acc.content.WriteString(text)
				fold.hasContentDelta = true
			}
		case gjson.JSON:
			for _, part := range appendTextParts(nil, content.Array(), "text") {
				if part != "" {
					acc.content.WriteString(part)
					fold.hasContentDelta = true
				}
			}
		}
		// reasoning 在不同网关形状不一（字符串 / 对象 / reasoning_details 数组）：
		// 只接受字符串，与 extractThinking 现有口径一致，不另立标准。
		if r := delta.Get("reasoning"); r.Type == gjson.String {
			acc.reasoningSeen = true
			if r.String() != "" {
				acc.reasoning.WriteString(r.String())
				fold.hasContentDelta = true
			}
		}
		if r := delta.Get("reasoning_content"); r.Type == gjson.String {
			acc.reasoningCtnSeen = true
			if r.String() != "" {
				acc.reasoningCtn.WriteString(r.String())
				fold.hasContentDelta = true
			}
		}
		if role := delta.Get("role"); role.Type == gjson.String && acc.role == "" {
			acc.role = role.String()
		}
		foldOpenAIChatToolCalls(acc, delta.Get("tool_calls"), &fold)

		if reason := choice.Get("finish_reason"); reason.Type == gjson.String && reason.String() != "" {
			acc.finishReason = reason.String()
		}
	}

	fold.canonical = acc.canonicalBody()
	return fold
}

// foldOpenAIChatToolCalls 按 tool_call 的 index 分片累加 arguments（覆盖式赋值会把 JSON
// 拼成只剩最后一片）。
//
// 如实说明本折叠的边界：部分 OpenAI 兼容网关把结构化输出放在 tool_calls 里，但
// extractStructuredOutputJSON 只看 message.content（非流式同样如此），所以这里的分片
// **只服务于「有没有内容增量」判定与回显**，不宣称能让 structured 测试走 tool 通道。
// 想支持那条路径要在 extractor 里加兜底，那会同时改变非流式的通过标准，属另一件事。
func foldOpenAIChatToolCalls(acc *openAIChatStreamAccumulator, toolCalls gjson.Result, fold *streamFold) {
	if !toolCalls.IsArray() {
		return
	}
	for _, item := range toolCalls.Array() {
		index := int(item.Get("index").Int())
		slot, ok := acc.toolCalls[index]
		if !ok {
			slot = &openAIChatToolCallAccumulator{}
			acc.toolCalls[index] = slot
		}
		if id := item.Get("id"); id.Type == gjson.String && id.String() != "" {
			slot.id = id.String()
		}
		if callType := item.Get("type"); callType.Type == gjson.String && callType.String() != "" {
			slot.callType = callType.String()
		}
		if name := item.Get("function.name"); name.Type == gjson.String && name.String() != "" {
			slot.name = name.String()
		}
		if args := item.Get("function.arguments"); args.Type == gjson.String && args.String() != "" {
			slot.arguments.WriteString(args.String())
			slot.argumentsOK = true
			fold.hasContentDelta = true
		}
	}
}

func (acc *openAIChatStreamAccumulator) canonicalBody() []byte {
	message := map[string]any{"content": acc.content.String()}
	if acc.role != "" {
		message["role"] = acc.role
	}
	if acc.reasoningCtnSeen {
		message["reasoning_content"] = acc.reasoningCtn.String()
	}
	if acc.reasoningSeen {
		message["reasoning"] = acc.reasoning.String()
	}
	if indexes := sortedToolCallIndexes(acc.toolCalls); len(indexes) > 0 {
		toolCalls := make([]any, 0, len(indexes))
		for _, index := range indexes {
			slot := acc.toolCalls[index]
			function := map[string]any{"name": slot.name}
			if slot.argumentsOK {
				function["arguments"] = slot.arguments.String()
			}
			callType := slot.callType
			if callType == "" {
				callType = "function"
			}
			toolCalls = append(toolCalls, map[string]any{
				"index":    index,
				"id":       slot.id,
				"type":     callType,
				"function": function,
			})
		}
		message["tool_calls"] = toolCalls
	}

	choice := map[string]any{"index": 0, "message": message, "finish_reason": acc.finishReason}
	body, err := json.Marshal(map[string]any{"choices": []any{choice}})
	if err != nil {
		// 手拼 map 不可能 marshal 失败；真失败即代码缺陷，panic 比静默交出空判定体更可诊断。
		panic(fmt.Sprintf("testapi: OpenAI Chat 折叠体序列化失败: %v", err))
	}
	return body
}

// --- Anthropic Messages ---

type anthropicBlockAccumulator struct {
	blockType   string
	id          string
	name        string
	text        strings.Builder
	thinking    strings.Builder
	signature   strings.Builder
	partialJSON strings.Builder
}

type anthropicStreamAccumulator struct {
	blocks     map[int]*anthropicBlockAccumulator
	stopReason string
}

// foldAnthropicStream 按 content_block index 建块并保序（thinking 块在前、text 块在后，
// extractor 用 `content.#(type=="thinking")` 取值，块序错乱不影响判定但会影响回显可读性）。
func foldAnthropicStream(frames []credprobe.SSEFrame) streamFold {
	fold := streamFold{terminatedBy: streamTerminalNone}
	acc := &anthropicStreamAccumulator{blocks: map[int]*anthropicBlockAccumulator{}}

	for _, frame := range frames {
		payload := gjson.Parse(frame.Data)
		if !payload.IsObject() {
			continue
		}
		eventName := eventNameOf(frame)
		switch eventName {
		case "content_block_start":
			block := payload.Get("content_block")
			acc.startBlock(int(payload.Get("index").Int()), block, &fold)
		case "content_block_delta":
			acc.foldDelta(payload, &fold)
		case "message_delta":
			if reason := payload.Get("delta.stop_reason"); reason.Type == gjson.String && reason.String() != "" {
				acc.stopReason = reason.String()
			}
		case "message_stop":
			fold.terminatedBy = streamTerminalMessageStop
		case "error":
			fold.streamError = streamErrorFromPayload(payload)
		default:
			// message_start / ping / content_block_stop 等对判定无新增事实。
		}
	}

	fold.canonical = acc.canonicalBody()
	return fold
}

func (acc *anthropicStreamAccumulator) block(index int) *anthropicBlockAccumulator {
	block, ok := acc.blocks[index]
	if !ok {
		block = &anthropicBlockAccumulator{}
		acc.blocks[index] = block
	}
	return block
}

// startBlock 收块类型、tool_use 的 id/name，以及**块内初始内容**。
// id/name 在 start 之后不再出现，不在此收就永久丢失；初始内容同理——官方流是
// `text:""` / `input:{}` 的空占位，但部分中转把整段答案或完整 input 直接放进 start 块
// 且不再发 delta，漏收会把「上游其实答了」判成「无内容增量」（与 thinking 假阴性同型）。
//
// 若上游同时给了 start 初始 input 与后续 partial_json，两者会被拼成一段而不再合法——
// 那种自相矛盾的流交给 schema 校验报真错，折叠层不猜。
func (acc *anthropicStreamAccumulator) startBlock(index int, block gjson.Result, fold *streamFold) {
	slot := acc.block(index)
	if t := block.Get("type"); t.Type == gjson.String && t.String() != "" {
		slot.blockType = t.String()
	}
	if id := block.Get("id"); id.Type == gjson.String {
		slot.id = id.String()
	}
	if name := block.Get("name"); name.Type == gjson.String {
		slot.name = name.String()
	}
	if text := block.Get("text"); text.Type == gjson.String && text.String() != "" {
		slot.text.WriteString(text.String())
		fold.hasContentDelta = true
	}
	if thinking := block.Get("thinking"); thinking.Type == gjson.String && thinking.String() != "" {
		slot.thinking.WriteString(thinking.String())
		fold.hasContentDelta = true
	}
	if input := block.Get("input"); input.Exists() && input.Type != gjson.Null {
		if raw := strings.TrimSpace(input.Raw); raw != "" && raw != "{}" {
			slot.partialJSON.WriteString(raw)
			fold.hasContentDelta = true
		}
	}
}

func (acc *anthropicStreamAccumulator) foldDelta(payload gjson.Result, fold *streamFold) {
	block := acc.block(int(payload.Get("index").Int()))
	delta := payload.Get("delta")
	switch delta.Get("type").String() {
	case "text_delta":
		if text := delta.Get("text"); text.Type == gjson.String && text.String() != "" {
			block.text.WriteString(text.String())
			fold.hasContentDelta = true
		}
	case "thinking_delta":
		if thinking := delta.Get("thinking"); thinking.Type == gjson.String && thinking.String() != "" {
			block.thinking.WriteString(thinking.String())
			fold.hasContentDelta = true
		}
	case "signature_delta":
		// signature 属于 thinking 块的一部分（块内容与其签名必须同源），不能丢。
		if signature := delta.Get("signature"); signature.Type == gjson.String {
			block.signature.WriteString(signature.String())
		}
	case "input_json_delta":
		// tool 入参在流里是 JSON 碎片，只能整块拼完再判（单片不是合法值）。
		if partial := delta.Get("partial_json"); partial.Type == gjson.String && partial.String() != "" {
			block.partialJSON.WriteString(partial.String())
			fold.hasContentDelta = true
		}
	default:
		// 未知 delta 类型（上游扩字段）：忽略，不当成内容增量。
	}
}

func (acc *anthropicStreamAccumulator) canonicalBody() []byte {
	indexes := make([]int, 0, len(acc.blocks))
	for index := range acc.blocks {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	blocks := make([]any, 0, len(indexes))
	for _, index := range indexes {
		blocks = append(blocks, acc.blocks[index].canonicalBlock())
	}

	body, err := json.Marshal(map[string]any{
		"type":        "message",
		"role":        "assistant",
		"content":     blocks,
		"stop_reason": acc.stopReason,
	})
	if err != nil {
		panic(fmt.Sprintf("testapi: Anthropic 折叠体序列化失败: %v", err))
	}
	return body
}

func (b *anthropicBlockAccumulator) canonicalBlock() map[string]any {
	blockType := b.blockType
	if blockType == "" {
		// 没收到 content_block_start（上游漏发或被截断）：按块内已有的最强信号推断。
		switch {
		case b.partialJSON.Len() > 0:
			blockType = "tool_use"
		case b.thinking.Len() > 0 || b.signature.Len() > 0:
			blockType = "thinking"
		default:
			blockType = "text"
		}
	}

	block := map[string]any{"type": blockType}
	switch blockType {
	case "tool_use":
		block["id"] = b.id
		block["name"] = b.name
		// input 必须是 JSON 值：extractStructuredOutputJSON 取 `.input` 的 Raw 当负载用，
		// 塞字符串会让它拿到带引号的原文。
		block["input"] = b.canonicalToolInput()
	case "thinking":
		block["thinking"] = b.thinking.String()
		if b.signature.Len() > 0 {
			block["signature"] = b.signature.String()
		}
		block["id"] = b.id
	default:
		block["text"] = b.text.String()
	}
	return block
}

// canonicalToolInput 拼完的 tool 入参碎片**原样保留字节**：先 unmarshal 再 marshal 会让
// Go 把 map 键按字母序重排，判定文本与上游原文就对不上、也白白扩大 diff 面。
// 拼不回合法 JSON（多半是流被切断）时退回片段字符串，交给既有的 schema 校验报出真实原因，
// 不在折叠层另发明一套错误。
func (b *anthropicBlockAccumulator) canonicalToolInput() any {
	fragment := strings.TrimSpace(b.partialJSON.String())
	if fragment == "" {
		return map[string]any{}
	}
	if json.Valid([]byte(fragment)) {
		return json.RawMessage(fragment)
	}
	return fragment
}

// --- OpenAI Responses ---

// foldResponsesStream 直接取终止帧 response.completed 的 response 原文——它本身就是完整
// 非流式响应体形状。
//
// 刻意不做 delta 兜底折叠：严格门要求终止帧存在，没有终止帧的流无论如何都判失败，
// 多一条「从 delta 猜完整响应」的分支只会与官方 schema 漂移，是纯负债。
func foldResponsesStream(frames []credprobe.SSEFrame) streamFold {
	fold := streamFold{terminatedBy: streamTerminalNone}

	for _, frame := range frames {
		payload := gjson.Parse(frame.Data)
		eventName := eventNameOf(frame)
		switch {
		case eventName == streamTerminalResponseCompleted:
			fold.terminatedBy = streamTerminalResponseCompleted
			fold.canonical, _ = foldResponsesBody(payload)
		case eventName == streamTerminalResponseFailed, eventName == streamTerminalResponseIncomplete:
			fold.terminatedBy = eventName
			fold.canonical, _ = foldResponsesBody(payload)
			if reason := payload.Get("response.error.message"); reason.Type == gjson.String && reason.String() != "" {
				fold.streamError = reason.String()
			} else if detail := payload.Get("response.incomplete_details.reason"); detail.Type == gjson.String && detail.String() != "" {
				fold.streamError = "响应未完成: " + detail.String()
			} else {
				fold.streamError = "上游以 " + eventName + " 结束流"
			}
		case strings.HasSuffix(eventName, ".delta"):
			// response.output_text.delta / reasoning_summary_text.delta /
			// function_call_arguments.delta / refusal.delta 等：带非空 delta 即内容增量。
			if delta := payload.Get("delta"); delta.Type == gjson.String && delta.String() != "" {
				fold.hasContentDelta = true
			}
		case eventName == "error":
			fold.streamError = streamErrorFromPayload(payload)
		}
	}

	// 上游可以一条 delta 都不发但 completed 里带完整 output（部分网关确实如此）：
	// 终止帧才是权威事实，delta 缺席不该被判成「无内容」。
	if !fold.hasContentDelta && fold.terminatedBy == streamTerminalResponseCompleted && len(fold.canonical) > 0 {
		output := gjson.GetBytes(fold.canonical, "output")
		fold.hasContentDelta = output.IsArray() && len(output.Array()) > 0
	}
	return fold
}

// foldResponsesBody 取事件载荷里的 response 对象原文（逐字节保留，不重新序列化）。
func foldResponsesBody(payload gjson.Result) ([]byte, bool) {
	response := payload.Get("response")
	if !response.Exists() || response.Type != gjson.JSON {
		return nil, false
	}
	return []byte(response.Raw), true
}

// streamErrorFromPayload 从帧载荷里取人读错误文本：兼容网关常在中途塞
// {"error":{...}}（HTTP 仍 200）。取不到 message 就退回该字段原文，绝不静默吞掉。
func streamErrorFromPayload(payload gjson.Result) string {
	errNode := payload.Get("error")
	if !errNode.Exists() || errNode.Type == gjson.Null {
		return ""
	}
	if message := errNode.Get("message"); message.Type == gjson.String && message.String() != "" {
		return message.String()
	}
	if errNode.Type == gjson.String && errNode.String() != "" {
		return errNode.String()
	}
	return errNode.Raw
}

func sortedToolCallIndexes(toolCalls map[int]*openAIChatToolCallAccumulator) []int {
	indexes := make([]int, 0, len(toolCalls))
	for index := range toolCalls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	return indexes
}
