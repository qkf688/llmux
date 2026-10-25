package streaming

import (
	"encoding/json"
	"log/slog"
	"sort"

	"github.com/qkf688/llmux/common/maputil"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/models/unified"
	"github.com/qkf688/llmux/service/transform/shared"
)

func init() {
	RegisterRealtimeRoute(consts.FormatOpenAIChat, consts.FormatOpenAIResponses, handleRealtimeOpenAIToResponses)
}

func handleRealtimeOpenAIToResponses(state *realtimeStreamState, data string) error {
	// OpenAI 的 [DONE] 不需要转换，但它是「上游已结束」的信号：
	// 此时把延后的 response.completed（携带尾包 usage）flush 出去；
	// 上游没给 finish_reason 时，还在这里做流末兜底收尾（补齐终态）。
	// 正常路径下 usage 尾包到达时已交付，这里退化为幂等兜底。
	if data == "[DONE]" {
		return finalizeOpenAIToResponsesStream(state)
	}

	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		logRealtimeChunkParseError(state, data, err)
		return nil
	}

	// 缓存上游的 id/model/created：流末兜底收尾拿不到具体 chunk（上游可能整条流
	// 都不给 finish_reason），response.completed 的这三项只能取缓存值。
	captureOpenAIStreamMeta(state, chunk)

	// usage 可能出现在任意 chunk（含 finish 之后的 choices:[] 尾包）。
	// 先无条件抓取，供延后的 response.completed 使用（待办 #18）。
	captureOpenAIUsage(state, chunk)

	// usage 一到即可交付终态，不必再等 [DONE]。
	// 这一步是活性保障而非优化：若上游发完 usage 后既不发 [DONE] 也不关连接
	// （keep-alive 挂住、代理层半开），而终态只在 [DONE]/EOF 交付，客户端要等到
	// 读超时；且第二跳 responses_to_anthropic 的收尾只由 response.completed 驱动、
	// 无 EOF 兜底，届时 anthropic 客户端会拿到未关闭的 content_block。
	// 对「usage 与 finish 同包」的上游不触发：那时 pendingCompleted 尚为 nil。
	if state.pendingCompleted != nil && len(state.pendingUsage) > 0 {
		if err := flushPendingOpenAIToResponsesCompleted(state); err != nil {
			return err
		}
	}

	choices, ok := chunk["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		// choices:[] 的尾包（stream_options.include_usage 的 usage 载体）到此为止：
		// usage 已被 captureOpenAIUsage 收走，无 delta/finish 可处理。
		return nil
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil
	}

	// response.created / response.in_progress 是流首两个事件，与 role / content 无关：
	// 上游可能先给 reasoning_content 或 tool_calls（此时还没有 role），
	// 若把 created 挂在 role 上，这些项的 output_item.added 会先于 created 出现——
	// 违反 Responses 协议「response.created 是流首事件」。
	if err := ensureOpenAIResponsesStarted(state, chunk); err != nil {
		return err
	}

	// finish_reason 只依赖 choice、不依赖 delta，但**不能在这里提前 return**：
	// 上游常把最后一截正文/工具参数与结束标记合在同一个 chunk
	//（`{"delta":{"content":"末段"},"finish_reason":"stop"}`，本仓 testapi 与
	// credprobe 的测试服务端都是此形状）。提前 return 会把该帧的 delta 整段吃掉，
	// 表现为正文少最后一截、工具参数变空串、finish_reason=tool_calls 被降级成
	// 普通结束。故先处理 delta（缺失时按空增量），再走收尾。
	finishReason := maputil.String(choice, "finish_reason")

	// delta 缺失或为 null 的帧不带任何增量内容，按空增量处理后继续。
	// 这也是「末包不带 delta 键」能走到收尾的前提：不因 delta 断言失败而丢帧。
	delta, _ := choice["delta"].(map[string]interface{})

	role := maputil.String(delta, "role")
	content := maputil.String(delta, "content")

	// message 项在「首帧带 role 或带 content」时开启：
	//   - 只看 role：整条流不带 role 的上游（部分兼容网关）永远开不出 message 项，
	//     客户端拿到「有正文、无 message 项」的裸 delta 流；
	//   - 只看 content：纯 tool_call 流会凭空多出一个空 message 项。
	// role 只决定 message 项的 role 字段，不决定它何时开。
	if role != "" || content != "" {
		if err := ensureOpenAIMessageItem(state, role); err != nil {
			return err
		}
	}

	if reasoningContent := maputil.String(delta, "reasoning_content"); reasoningContent != "" {
		armOpenAIToResponsesFinalize(state)
		if err := emitOpenAIToResponsesReasoningDelta(state, reasoningContent); err != nil {
			return err
		}
	}

	if content != "" {
		armOpenAIToResponsesFinalize(state)
		state.accumulatedText += content
		textDelta := map[string]interface{}{
			"type":            "response.output_text.delta",
			"sequence_number": nextRealtimeSequence(state),
			// 必须取 message 项自己的 index，不能写死 0：reasoning / function_call
			// 先于正文到达时会占走 0，message 项拿 1——写死 0 会把正文挂到别人的项上。
			"output_index":  ensureMessageOutputIndex(state),
			"item_id":       state.itemID,
			"content_index": 0,
			"delta":         content,
		}
		if err := writeRealtimeOrderedData(state, textDelta); err != nil {
			return err
		}
	}

	if toolCalls, ok := delta["tool_calls"].([]interface{}); ok {
		if len(toolCalls) > 0 {
			armOpenAIToResponsesFinalize(state)
		}
		if err := emitOpenAIToResponsesToolCalls(state, toolCalls); err != nil {
			return err
		}
	}

	// 增量处理完再收尾：该帧的 delta（若有）已经写出，finish 只决定终态。
	if finishReason != "" {
		return finishOpenAIToResponsesStream(state, chunk, finishReason)
	}

	return nil
}

// ensureMessageOutputIndex 惰性给 message 项分配 output_index。
// 分配而非写死 0，是为了让 reasoning / function_call 能拿到互不重叠的 index。
func ensureMessageOutputIndex(state *realtimeStreamState) int {
	if !state.hasMessageOutputIndex {
		state.messageOutputIndex = state.responsesOutput.alloc()
		state.hasMessageOutputIndex = true
	}
	return state.messageOutputIndex
}

// armOpenAIToResponsesFinalize 挂上流末收尾出口。
//
// **为什么要在「首次产出内容」就挂，而不是等 finish_reason**：
// state.finalize 是流末（EOF / 上游读错误）唯一的收尾出口，realtime.go 的兜底
// 逻辑是 `if state.finalize != nil`。旧实现只在 finish 分支里赋值 finalize，
// 于是「上游全程不给 finish_reason」时 finalize 恒为 nil，流末兜底整段跳过——
// 注释声称的「上游不发 [DONE] 也要收尾」实际只覆盖了「已经 finish 过」的流。
//
// 挂上之后，finish 分支仍会覆盖它（同一个函数），两条路径共用一份收尾实现。
func armOpenAIToResponsesFinalize(state *realtimeStreamState) {
	if state.finalize == nil {
		state.finalize = finalizeOpenAIToResponsesStream
	}
}

// ensureOpenAIResponsesStarted 发出流首两个事件（response.created / response.in_progress）。
//
// 幂等：一条流只发一次。部分 OpenAI 兼容上游会在**末包**（带 finish_reason 的那一帧）
// 再带一次 role；若不拦，客户端会看到「同一个 response id 建了两次」，
// 严格 Responses 客户端判协议异常。
//
// 与 role 无关：上游可能先给 reasoning_content / tool_calls 再给 role，
// 届时 created 必须已经发出（它是流首事件），不能等 role。
func ensureOpenAIResponsesStarted(state *realtimeStreamState, chunk map[string]interface{}) error {
	if state.responseStreamStarted {
		return nil
	}
	state.responseStreamStarted = true

	// id 按目标协议（Responses）重铸：上游 OpenAI Chat 的 `chatcmpl-...` 不能直接当
	// Responses 的响应级 id 用（客户端会按 `resp_` 前缀认响应）。
	// message 项 id 前缀固定 `msg_`（Responses 的 item 级标识），与响应级不是同一个东西。
	state.responseID = shared.ResponseIDFor(consts.FormatOpenAIResponses, maputil.String(chunk, "id"))
	state.itemID = shared.ResponseItemIDFor(state.responseID)

	responseCreated := map[string]interface{}{
		"type":            "response.created",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"id":         state.responseID,
			"object":     "response",
			"model":      maputil.String(chunk, "model"),
			"created_at": int(maputil.Float64(chunk, "created")),
			"status":     "in_progress",
			"output":     []interface{}{},
		},
	}
	if err := writeRealtimeOrderedData(state, responseCreated); err != nil {
		return err
	}

	responseInProgress := map[string]interface{}{
		"type":            "response.in_progress",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"id":         state.responseID,
			"object":     "response",
			"model":      maputil.String(chunk, "model"),
			"created_at": int(maputil.Float64(chunk, "created")),
			"status":     "in_progress",
			"output":     []interface{}{},
		},
	}
	return writeRealtimeOrderedData(state, responseInProgress)
}

// ensureOpenAIMessageItem 开启 message 项（output_item.added + content_part.added）。
//
// 幂等：一条流只开一次。开启条件是「首帧带 role 或带 content」——
// 只看 role 会漏掉整条流不带 role 的上游（永远开不出 message 项），
// 只看 content 会给纯 tool_call 流凭空造一个空 message 项。
// role 只决定 message 项的 role 字段，缺省用 "assistant"。
func ensureOpenAIMessageItem(state *realtimeStreamState, role string) error {
	if state.messageStreamStarted {
		return nil
	}
	state.messageStreamStarted = true

	if role == "" {
		role = "assistant"
	}
	msgIndex := ensureMessageOutputIndex(state)

	itemAdded := map[string]interface{}{
		"type":            "response.output_item.added",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item": map[string]interface{}{
			"id":      state.itemID,
			"type":    "message",
			"role":    role,
			"content": []interface{}{},
			"status":  "in_progress",
		},
	}
	if err := writeRealtimeOrderedData(state, itemAdded); err != nil {
		return err
	}

	contentPartAdded := map[string]interface{}{
		"type":            "response.content_part.added",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item_id":         state.itemID,
		"content_index":   0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": "",
		},
	}
	return writeRealtimeOrderedData(state, contentPartAdded)
}

func emitOpenAIToResponsesReasoningDelta(state *realtimeStreamState, reasoningContent string) error {
	// 如果还没有创建 reasoning item，先创建
	if !state.hasReasoningItem {
		state.hasReasoningItem = true
		state.reasoningItemID = "reasoning_" + state.responseID
		// 独立分配：写死 0 会与 message 项的 output_index 撞车。
		state.reasoningOutputIndex = state.responsesOutput.alloc()

		reasoningItemAdded := map[string]interface{}{
			"type":            "response.output_item.added",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item": map[string]interface{}{
				"id":      state.reasoningItemID,
				"type":    "reasoning",
				"status":  "in_progress",
				"summary": []interface{}{},
			},
		}
		if err := writeRealtimeOrderedData(state, reasoningItemAdded); err != nil {
			return err
		}

		summaryPartAdded := map[string]interface{}{
			"type":            "response.reasoning_summary_part.added",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item_id":         state.reasoningItemID,
			"summary_index":   0,
			"part": map[string]interface{}{
				"type": "summary_text",
			},
		}
		if err := writeRealtimeOrderedData(state, summaryPartAdded); err != nil {
			return err
		}
	}

	state.accumulatedReasoning += reasoningContent
	reasoningDelta := map[string]interface{}{
		"type":            "response.reasoning_summary_text.delta",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    state.reasoningOutputIndex,
		"item_id":         state.reasoningItemID,
		"summary_index":   0,
		"delta":           reasoningContent,
	}
	return writeRealtimeOrderedData(state, reasoningDelta)
}

func emitOpenAIToResponsesToolCalls(state *realtimeStreamState, toolCalls []interface{}) error {
	// 终态之后不得再开新的 output item：Responses 协议里 response.completed 不可撤回，
	// 此后开的项既进不了 response.completed 的 output 快照（已构建），也不会被
	// emitResponsesOutputItemFinalization 收尾（收尾已跑过）——产出「开了却没关」的
	// item，严格客户端判本轮运行失败。
	//
	// 触发形状：上游把 finish_reason 帧发在工具名之前（部分兼容上游会补发后续帧）。
	// 流末兜底已按「有 id 无 name」丢弃该工具调用并发出终态，后续帧若再把它当
	// 待开的工具调用处理，就会重开一个永不关闭的 item。
	if state.openAIOutputClosed {
		return nil
	}

	for _, tc := range toolCalls {
		toolCall, ok := tc.(map[string]interface{})
		if !ok {
			continue
		}
		function, ok := toolCall["function"].(map[string]interface{})
		if !ok {
			continue
		}

		toolCallIndex := int(maputil.Float64(toolCall, "index"))
		buf := getOrAllocOpenAIToolCallBuffer(state, toolCallIndex)

		// 收集本帧的 id/name：上游可能分帧给（甚至只给其一），取首个非空值，
		// 后续帧不得覆盖。
		if id := maputil.String(toolCall, "id"); id != "" && buf.id == "" {
			buf.id = id
		}
		if name := maputil.String(function, "name"); name != "" && buf.name == "" {
			buf.name = name
		}

		// 参数增量：added 之前先进缓冲（协议要求 delta 挂在已开的 item 上，
		// 修复前参数 delta 先于 added 发出属协议违规），added 之后直接透传。
		if args := maputil.String(function, "arguments"); args != "" {
			if buf.opened {
				state.responsesOutput.appendArguments(buf.outputIndex, args)

				argsDelta := map[string]interface{}{
					"type":            "response.function_call_arguments.delta",
					"sequence_number": nextRealtimeSequence(state),
					"output_index":    buf.outputIndex,
					"delta":           args,
				}
				if err := writeRealtimeOrderedData(state, argsDelta); err != nil {
					return err
				}
			} else {
				buf.args += args
			}
		}

		// id+name 齐备才开 item：修复前 id 帧即发 added 且 name 写死一次，
		// id 帧缺 name 产出空名 tool_use（下游客户端报 No such tool 并把脏块
		// 写回历史，会话被出站校验 400 卡死）。
		if !buf.opened && buf.id != "" && buf.name != "" {
			if err := openOpenAIToResponsesToolCall(state, buf); err != nil {
				return err
			}
		}
	}

	return nil
}

// openAIToolCallBuffer 缓冲一个「id/name 尚未齐备」的工具调用。
type openAIToolCallBuffer struct {
	outputIndex int
	id          string
	name        string
	args        string // added 之前攒的参数增量
	opened      bool   // output_item.added 是否已发
}

func getOrAllocOpenAIToolCallBuffer(state *realtimeStreamState, toolCallIndex int) *openAIToolCallBuffer {
	if state.openAIToolCallBuffers == nil {
		state.openAIToolCallBuffers = map[int]*openAIToolCallBuffer{}
	}
	if buf, ok := state.openAIToolCallBuffers[toolCallIndex]; ok {
		return buf
	}
	// output_index 直接分配：缓冲项本身就持有它，且缓冲 map 与 tool_call index
	// 同键域同生命周期，另建一张 index→output_index 映射表是同义重复。
	buf := &openAIToolCallBuffer{outputIndex: state.responsesOutput.alloc()}
	state.openAIToolCallBuffers[toolCallIndex] = buf
	return buf
}

// openOpenAIToResponsesToolCall 发出 function_call 的 output_item.added，
// 并把缓冲里攒的参数增量作为首个 delta 补发（added 必须先于该 item 的任何 delta）。
func openOpenAIToResponsesToolCall(state *realtimeStreamState, buf *openAIToolCallBuffer) error {
	item := map[string]interface{}{
		"type":      "function_call",
		"id":        buf.id,
		"call_id":   buf.id,
		"name":      buf.name,
		"arguments": "",
	}
	state.responsesOutput.set(buf.outputIndex, item)

	itemAdded := map[string]interface{}{
		"type":            "response.output_item.added",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    buf.outputIndex,
		"item":            item,
	}
	if err := writeRealtimeOrderedData(state, itemAdded); err != nil {
		return err
	}

	buf.opened = true

	// added 之前攒的参数此刻补发为一个 delta。
	if buf.args != "" {
		state.responsesOutput.appendArguments(buf.outputIndex, buf.args)

		argsDelta := map[string]interface{}{
			"type":            "response.function_call_arguments.delta",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    buf.outputIndex,
			"delta":           buf.args,
		}
		if err := writeRealtimeOrderedData(state, argsDelta); err != nil {
			return err
		}
		buf.args = ""
	}
	return nil
}

// flushOpenAIToResponsesToolCallBuffers 处理流末仍未「id/name 齐备」的工具调用：
//   - 有 name 无 id：补 call_ 前缀兜底 id 发出（修复前整帧静默丢弃，客户端连错误都看不到）；
//   - 有 id 无 name：丢弃整条工具调用（上游没给工具名，发了就是空名 tool_use，
//     下游客户端会把它写进脏历史，会话被出站校验 400 卡死）。
//
// 必须在 emitOpenAIToResponsesOutputFinalization 开头调用：补发的事件要先于收尾事件，
// 且新开的项要能被 emitResponsesOutputItemFinalization 关掉。
func flushOpenAIToResponsesToolCallBuffers(state *realtimeStreamState) error {
	indices := make([]int, 0, len(state.openAIToolCallBuffers))
	for idx := range state.openAIToolCallBuffers {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	for _, idx := range indices {
		buf := state.openAIToolCallBuffers[idx]
		if buf.opened {
			continue
		}
		if buf.name == "" {
			slog.Warn("dropping tool call without function name at stream end",
				"tool_call_index", idx,
				"tool_call_id", buf.id,
				"upstream_format", state.upstreamFormat,
				"client_format", state.clientFormat,
			)
			continue
		}
		if buf.id == "" {
			buf.id = unified.GenerateUniqueToolCallID(nil)
			slog.Warn("synthesized missing tool_call id at stream end",
				"tool_call_index", idx,
				"tool_call_name", buf.name,
				"upstream_format", state.upstreamFormat,
				"client_format", state.clientFormat,
			)
		}
		if err := openOpenAIToResponsesToolCall(state, buf); err != nil {
			return err
		}
	}
	return nil
}

func finishOpenAIToResponsesStream(state *realtimeStreamState, chunk map[string]interface{}, finishReason string) error {
	// 幂等门：终态不可撤回。上游重复带 finish_reason 时（兼容上游常见），
	// 第二帧不得再发一套 output_item.done / response.completed。
	if state.openAIFinalized {
		return nil
	}

	if err := emitOpenAIToResponsesOutputFinalization(state); err != nil {
		return err
	}

	status := "completed"
	if finishReason == "length" {
		status = "incomplete"
	}

	responseCompleted := map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"object":     "response",
			"id":         state.responseID,
			"model":      maputil.String(chunk, "model"),
			"created_at": int(maputil.Float64(chunk, "created")),
			"status":     status,
			// 必须是真实产出：下游靠扫这个数组里有没有 function_call 判定 finish_reason。
			"output": state.responsesOutput.snapshot(),
		},
	}

	// 不能在此直接写出：OpenAI 把 usage 放在 finish_reason 之后的独立尾包里，
	// 现在发就永远带不上真实 token 数（待办 #18）。缓存起来，等 [DONE] 或流末
	// 由 flushPendingOpenAIToResponsesCompleted 合并 usage 后统一发出。
	state.pendingCompleted = responseCompleted
	state.finalize = finalizeOpenAIToResponsesStream
	return nil
}

// emitOpenAIToResponsesOutputFinalization 发出 message / reasoning / function_call
// 三类 output item 的收尾事件。
//
// 从 finishOpenAIToResponsesStream 抽出来的原因：收尾能力必须与「上游是否给
// finish_reason」解耦。上游不给 finish_reason（兼容上游 / 流被截断）时，
// 兜底路径仍要调用它，否则开了的 output item 永远不关，客户端判「未完成」。
//
// 幂等：openAIOutputClosed 门保证只发一次（终态不可撤回）。
func emitOpenAIToResponsesOutputFinalization(state *realtimeStreamState) error {
	if state.openAIOutputClosed {
		return nil
	}
	state.openAIOutputClosed = true

	// 流末兜底：未齐备的工具调用必须先于收尾事件处理（补发或丢弃），
	// 否则新开的项来不及被 emitResponsesOutputItemFinalization 关掉。
	if err := flushOpenAIToResponsesToolCallBuffers(state); err != nil {
		return err
	}

	// reasoning 的收尾必须独立于 message 项是否存在：上游只给 reasoning_content、
	// 不给 role/content 时（推理模型先思考再调工具/结束的常见形态），message 项
	// 从未开出，若把它写进 message 分支内部就会被下面的早退挡掉——表现为
	// reasoning 项开了不关、且 response.completed.output 里没有 reasoning。
	if err := emitOpenAIReasoningFinalization(state); err != nil {
		return err
	}

	// message 项的收尾只对「开过」的项发：从未 output_item.added 过的项
	// （上游首帧即 finish_reason、或整条流不带 role 且无内容）若发 done，
	// 客户端会看到凭空冒出的 output_index——「关了但没开过」同样是协议违规。
	// 与 Anthropic 线的 `if !state.hasMessageOutputIndex` 守卫同款判据。
	if !state.messageStreamStarted {
		return emitResponsesOutputItemFinalization(state)
	}

	msgIndex := ensureMessageOutputIndex(state)

	outputTextDone := map[string]interface{}{
		"type":            "response.output_text.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item_id":         state.itemID,
		"content_index":   0,
		"text":            state.accumulatedText,
	}
	if err := writeRealtimeOrderedData(state, outputTextDone); err != nil {
		return err
	}

	contentPartDone := map[string]interface{}{
		"type":            "response.content_part.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item_id":         state.itemID,
		"content_index":   0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": state.accumulatedText,
		},
	}
	if err := writeRealtimeOrderedData(state, contentPartDone); err != nil {
		return err
	}

	// 构建 output_item.done 事件的 output 数组，包含 reasoning（如果有）
	outputItems := []map[string]interface{}{
		{
			"type": "output_text",
			"text": state.accumulatedText,
		},
	}
	if state.hasReasoningItem && state.accumulatedReasoning != "" {
		outputItems = append([]map[string]interface{}{
			{
				"type": "reasoning",
				"id":   state.reasoningItemID,
				"summary": []map[string]interface{}{
					{
						"type": "summary_text",
						"text": state.accumulatedReasoning,
					},
				},
			},
		}, outputItems...)
	}

	messageItem := map[string]interface{}{
		"id":      state.itemID,
		"type":    "message",
		"role":    "assistant",
		"content": outputItems,
		"status":  "completed",
	}
	state.responsesOutput.set(msgIndex, messageItem)

	outputItemDone := map[string]interface{}{
		"type":            "response.output_item.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item":            messageItem,
	}
	if err := writeRealtimeOrderedData(state, outputItemDone); err != nil {
		return err
	}
	state.responsesOutput.markClosed(msgIndex)

	// 其余开了还没关的项收尾：function_call 要 arguments.done + output_item.done，
	// reasoning 要 output_item.done。否则严格 Responses 客户端判「项开了没关」→
	// 本轮运行失败（deepseek-harness 实测报 unfinished tool call）。
	return emitResponsesOutputItemFinalization(state)
}

// captureOpenAIUsage 记录 chunk 里的 OpenAI 格式 usage。
// usage 既可能与 finish_reason 同包（部分上游的实现），也可能在其后的
// choices:[] 尾包里（OpenAI 规范行为），故对每个 chunk 都尝试抓取；
// 后到的非空 usage 覆盖先前的，保证拿到最终值。
func captureOpenAIUsage(state *realtimeStreamState, chunk map[string]interface{}) {
	usage, ok := chunk["usage"].(map[string]interface{})
	if !ok || len(usage) == 0 {
		return
	}
	state.pendingUsage = usage
	// 同一份原始 usage 也旁路交给落库侧：转换输出会被目标协议裁剪，
	// 这里是 openai 上游 usage 唯一未失真的观测点。
	captureUpstreamUsageMap(state, usage)
}

// flushPendingOpenAIToResponsesCompleted 把延后的 response.completed 合并 usage 后写出。
// 幂等：写出后清空全部延后状态。三个入口（usage 尾包到达 / [DONE] / 流末 finalize）
// 只有第一个真正写出，其余因 pendingCompleted == nil 直接返回。
func flushPendingOpenAIToResponsesCompleted(state *realtimeStreamState) error {
	completed := state.pendingCompleted
	if completed == nil {
		return nil
	}
	// 三个字段生命周期一致（「已消费」）：一并清空，避免读代码时误判还需收尾。
	usage := state.pendingUsage
	state.pendingCompleted = nil
	state.pendingUsage = nil
	state.finalize = nil

	// sequence_number 在此刻才分配：它必须反映真实发出顺序，而非构建顺序。
	// 依赖 nextRealtimeSequence 的不变量「取号顺序 == 写出顺序」。
	completed["sequence_number"] = nextRealtimeSequence(state)

	if response, ok := completed["response"].(map[string]interface{}); ok {
		if u := buildResponsesUsage(usage); u != nil {
			response["usage"] = u
		}
	}

	state.openAIFinalized = true
	return writeRealtimeOrderedData(state, completed)
}

// emitOpenAIReasoningFinalization 发出 reasoning 项的收尾事件并把它写入 output store。
//
// **为什么必须独立成函数**：reasoning 项的生命周期与 message 项无关——上游可能
// 只给 reasoning_content、不给 role/content（推理模型先思考再调工具/结束）。
// 曾把它写在 message 分支内部，被「message 没开就早退」挡掉，导致 reasoning
// 开了不关、且 response.completed.output 里完全没有 reasoning 项。
//
// 写入 store 才让 emitResponsesOutputItemFinalization 的 pendingItems() 扫得到它
// ——那一层的判据是「进了 store 就算开了」。
func emitOpenAIReasoningFinalization(state *realtimeStreamState) error {
	if !state.hasReasoningItem || state.accumulatedReasoning == "" {
		return nil
	}

	reasoningTextDone := map[string]interface{}{
		"type":            "response.reasoning_summary_text.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    state.reasoningOutputIndex,
		"item_id":         state.reasoningItemID,
		"summary_index":   0,
		"text":            state.accumulatedReasoning,
	}
	if err := writeRealtimeOrderedData(state, reasoningTextDone); err != nil {
		return err
	}

	reasoningPartDone := map[string]interface{}{
		"type":            "response.reasoning_summary_part.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    state.reasoningOutputIndex,
		"item_id":         state.reasoningItemID,
		"summary_index":   0,
		"part": map[string]interface{}{
			"type": "summary_text",
			"text": state.accumulatedReasoning,
		},
	}
	if err := writeRealtimeOrderedData(state, reasoningPartDone); err != nil {
		return err
	}

	// 写入 store：response.completed 的 output 快照从这里取，
	// emitResponsesOutputItemFinalization 的 pendingItems() 也据此判定「这个项开过」。
	state.responsesOutput.set(state.reasoningOutputIndex, map[string]interface{}{
		"id":   state.reasoningItemID,
		"type": "reasoning",
		"summary": []map[string]interface{}{
			{
				"type": "summary_text",
				"text": state.accumulatedReasoning,
			},
		},
		"status": "completed",
	})
	return nil
}

// finalizeOpenAIToResponsesStream 是本流的收尾出口：保证开了的 output item 全部
// 关掉、且终态事件一定发出。
//
// **为什么收尾不能只挂在 finish_reason 上**：上游给不给 finish_reason 不是
// 转换层能控制的——兼容上游常把末包写成 {"choices":[{"index":0,"finish_reason":"stop"}]}
// （不带 delta 键），也有上游整条流都不给 finish_reason。旧实现把收尾完全
// 挂在 finish 分支里，这两种形态下客户端只会收到「有正文、无终态」的悬空流，
// 严格 Responses 客户端报 "stream completed with an unfinished tool call"。
//
// 三层语义：
//  1. 正常路径（上游给了 finish_reason）：收尾事件已在 finishOpenAIToResponsesStream
//     里发出，这里只交付延后的 response.completed（含 usage）。
//  2. 兜底路径（上游没给 finish_reason）：这里补发全部收尾事件 + response.completed，
//     status 取 completed（上游没表达截断，就不能替它断言 incomplete）。
//  3. 幂等：已交付过终态则什么都不做（上游重复 finish / [DONE] 与 EOF 双出口）。
func finalizeOpenAIToResponsesStream(state *realtimeStreamState) error {
	if state.openAIFinalized {
		return nil
	}

	// 兜底路径：上游没给 finish_reason，收尾事件还没发过，这里补发。
	if !state.openAIOutputClosed {
		if err := emitOpenAIToResponsesOutputFinalization(state); err != nil {
			return err
		}
		if state.pendingCompleted == nil {
			state.pendingCompleted = map[string]interface{}{
				"type": "response.completed",
				"response": map[string]interface{}{
					"object":     "response",
					"id":         state.responseID,
					"model":      state.openAIStreamMeta.model,
					"created_at": state.openAIStreamMeta.created,
					// 上游没表达截断就不能断言 incomplete：status 只能是 completed。
					"status": "completed",
					"output": state.responsesOutput.snapshot(),
				},
			}
		}
	}

	return flushPendingOpenAIToResponsesCompleted(state)
}

// buildResponsesUsage 把上游原始 usage map 映射为 Responses 线的 usage 对象。
// 入参为 nil / 空时返回 nil，表示无 usage 可写（不写出零值，避免下游把 0 当真实统计）。
//
// 先过 models.UsageFromMap 归一再交 responsesUsageFromModel 装配：与落库侧、
// anthropic→responses 写出点共用同一份归一 + 装配实现，键名 / total 回退 / 零值
// 明细判据不再各写一遍。归一比旧实现多认几种上游写法（cached 的
// cache_read_input_tokens / prompt_cache_hit_tokens、reasoning 的顶层键等），
// 是修掉 openai→responses 此前漏认这些兼容字段的潜在缺口，方向与落库口径一致。
func buildResponsesUsage(usage map[string]interface{}) map[string]interface{} {
	if len(usage) == 0 {
		return nil
	}
	return responsesUsageFromModel(models.UsageFromMap(usage))
}
