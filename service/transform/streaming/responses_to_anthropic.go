package streaming

import (
	"encoding/json"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/service/responses"
)

// anthropicItemMeta 记录 Responses output item 的元数据，供并行顺序化时
// 在 output_item.done 驱动 block 切换后构造对应类型的 content_block_start。
type anthropicItemMeta struct {
	itemType string // "message" / "function_call" / "reasoning" / ...
	itemID   string // function_call 的 call_id
	itemName string // function_call 的 name
}

// anthropicBufferedDelta 缓冲一条未发出的 content_block_delta 的 inner delta
// （即 content_block_delta.delta 字段的内容，如 {"type":"text_delta","text":"x"}）。
// flush 时连同 block index 一起发出。
type anthropicBufferedDelta struct {
	inner map[string]interface{}
}

func init() {
	RegisterRealtimeRoute("openai-res", "anthropic", handleRealtimeResponsesToAnthropic)
}

func handleRealtimeResponsesToAnthropic(state *realtimeStreamState, data string) error {
	// [DONE] 被忽略，因为 response.completed 已发送 message_stop
	if data == "[DONE]" {
		return nil
	}

	var ev responses.ResponsesStreamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		logRealtimeChunkParseError(state, data, err)
		return nil
	}

	eventType := state.currentEvent
	if eventType == "" {
		eventType = ev.Type
	}

	switch eventType {
	case "response.created":
		return handleResponsesToAnthropicCreatedEvent(state, &ev)
	case "response.output_item.added":
		return handleResponsesToAnthropicOutputItemAddedEvent(state, &ev)
	case "response.output_item.done":
		return handleResponsesToAnthropicOutputItemDoneEvent(state, &ev)
	case "response.output_text.delta":
		return handleResponsesToAnthropicOutputTextDeltaEvent(state, &ev)
	case "response.reasoning_summary_text.delta":
		return handleResponsesToAnthropicReasoningDeltaEvent(state, &ev)
	case "response.function_call_arguments.delta":
		return handleResponsesToAnthropicFunctionArgsDeltaEvent(state, &ev)
	case "response.content_part.added":
		return handleResponsesToAnthropicContentPartAddedEvent(state, &ev)
	case "response.completed":
		return handleResponsesToAnthropicCompletedEvent(state, &ev)
	default:
		return nil
	}
}

func handleResponsesToAnthropicCreatedEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	responseID := ""
	if ev.Response != nil {
		responseID = ev.Response.ID
	}
	if responseID == "" {
		responseID = ev.ResponseID
	}

	// model 透传上游返回值，没有时用空字符串（Anthropic 客户端可容忍缺省）。
	modelName := ""
	if ev.Response != nil {
		modelName = ev.Response.Model
	}

	// usage：首个 chunk 自带 usage 时用真实值，否则兜底 1/1。
	// 写死 0/0 会让依赖 message_start.usage.input_tokens 做早期计费的客户端拿到 0 误判。
	// 真实 token 统计仍以 message_delta.usage 为准（completed 事件带最终值）。
	inputTokens, outputTokens := 1, 1
	if ev.Response != nil && ev.Response.Usage != nil {
		inputTokens = int(ev.Response.Usage.InputTokens)
		outputTokens = int(ev.Response.Usage.OutputTokens)
	}

	messageStart := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":      responseID,
			"type":    "message",
			"role":    "assistant",
			"content": []interface{}{},
			"model":   modelName,
			"usage": map[string]interface{}{
				"input_tokens":  inputTokens,
				"output_tokens": outputTokens,
			},
		},
	}
	return writeRealtimeEventJSONData(state, "message_start", messageStart)
}

func handleResponsesToAnthropicOutputItemAddedEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	if ev.Item == nil {
		return nil
	}
	itemType := ev.Item.Type

	// 记录 item 元数据，供并行顺序化时在 output_item.done 驱动 block 切换后
	// 构造对应类型的 content_block_start（见 openAnthropicBlockForItem）。
	if state.anthropicItemMeta == nil {
		state.anthropicItemMeta = map[int]anthropicItemMeta{}
	}
	state.anthropicItemMeta[ev.OutputIndex] = anthropicItemMeta{
		itemType: itemType,
		itemID:   ev.Item.ID,
		itemName: derefString(ev.Item.Name),
	}

	// message 是容器型 item（content 数组为空，正文由后续 output_text.delta 逐条送达）。
	// 若在此处急切开 text content_block，遇到推理模型 reasoning 先于 content 到达时，
	// reasoning 的 output_item.added 会把这个尚未收到任何文本的空 text block 提前 stop，
	// 导致后续正文 text_delta 全发到已关闭的 block 上（log-33 bug）。
	// 因此 message 只预留 output_index → block_index 映射，text block 改由第一个
	// output_text.delta 惰性开启（见 emitOrBufferAnthropicDelta 的 active 接管分支）。
	if itemType == "message" {
		getOrAllocAnthropicBlockIndex(state, ev.OutputIndex)
		return nil
	}

	// 并行顺序化：Anthropic 一次只允许一个 active content_block。
	// 若当前无 active block，立即开本 item 的 block；若已有 active（另一个 item 正在接收 delta），
	// 只记录 meta 排队，不关不开——本 item 的 block 由前一个 item 的 output_item.done
	// 驱动 openNextPendingBlock 时开启，期间的 delta 由 emitOrBufferAnthropicDelta 缓冲。
	if state.anthropicActiveOutputIndex < 0 {
		blockIndex := getOrAllocAnthropicBlockIndex(state, ev.OutputIndex)
		if err := openAnthropicBlockForItem(state, blockIndex, ev.OutputIndex, state.anthropicItemMeta[ev.OutputIndex]); err != nil {
			return err
		}
		state.anthropicActiveOutputIndex = ev.OutputIndex
	}
	return nil
}

// handleResponsesToAnthropicOutputItemDoneEvent 处理 output_item.done：
// 标记该 output_index 已完成。若它是当前 active，关掉它的 block，然后从缓冲队列里
// 取下一个有待发 delta 的 output_index，开 block 并 flush 缓冲。
func handleResponsesToAnthropicOutputItemDoneEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	if state.anthropicDoneOutputIndices == nil {
		state.anthropicDoneOutputIndices = map[int]bool{}
	}
	state.anthropicDoneOutputIndices[ev.OutputIndex] = true

	// 非 active item 的 done：无需切换 block，只标记 done（防止迟到的 delta 被缓冲）。
	if state.anthropicActiveOutputIndex != ev.OutputIndex {
		return nil
	}

	// active item 完成：关 block，清 active，开下一个待 flush 的 block。
	if state.anthropicActiveBlockIndex >= 0 {
		blockStop := map[string]interface{}{
			"type":  "content_block_stop",
			"index": state.anthropicActiveBlockIndex,
		}
		if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
			return err
		}
		state.anthropicActiveBlockIndex = -1
	}
	state.anthropicActiveOutputIndex = -1

	return openNextPendingAnthropicBlock(state)
}

// openNextPendingAnthropicBlock 从候选列表（有 meta、未 started、需补发）中找最小
// output_index，开 block 并 flush 其全部缓冲 delta。
// 补发后行为按 done 状态分：
//   - 已 done → 立即 stop + 清 active + 继续找下一个（循环补发空 block）
//   - 未 done → 设为 active + 返回（不 stop，等 done 或后续 delta 驱动）
//
// 循环不变量：每次迭代至少标记一个 started（openAnthropicBlockForItem 内部标记），
// 候选集合有限（≤ item 数），必然终止。
func openNextPendingAnthropicBlock(state *realtimeStreamState) error {
	for {
		indices := collectPendingAnthropicBlockIndices(state)
		if len(indices) == 0 {
			return nil
		}
		nextIdx := indices[0]

		meta, ok := state.anthropicItemMeta[nextIdx]
		if !ok {
			meta = anthropicItemMeta{itemType: "message"}
		}
		blockIndex := getOrAllocAnthropicBlockIndex(state, nextIdx)
		if err := openAnthropicBlockForItem(state, blockIndex, nextIdx, meta); err != nil {
			return err
		}

		// flush 缓冲 delta（可能为空——空 args 的 tool_call 无 delta）。
		for _, bd := range state.anthropicDeltaBuffer[nextIdx] {
			contentDelta := map[string]interface{}{
				"type":  "content_block_delta",
				"index": blockIndex,
				"delta": bd.inner,
			}
			if err := writeRealtimeEventJSONData(state, "content_block_delta", contentDelta); err != nil {
				return err
			}
		}
		delete(state.anthropicDeltaBuffer, nextIdx)

		// 已 done：立即 stop + 清 active + 继续找下一个（空 args 的 tool_call 走此分支）。
		if state.anthropicDoneOutputIndices != nil && state.anthropicDoneOutputIndices[nextIdx] {
			blockStop := map[string]interface{}{
				"type":  "content_block_stop",
				"index": blockIndex,
			}
			if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
				return err
			}
			state.anthropicActiveBlockIndex = -1
			continue
		}

		// 未 done：设为 active + 返回（等 done 或后续 delta 驱动，不提前 stop）。
		state.anthropicActiveOutputIndex = nextIdx
		return nil
	}
}

// flushRemainingAnthropicBuffers 在 response.completed 兜底阶段，按 output_index 升序
// 逐个处理「有 meta、未 started」的 item：开 block → flush 缓冲 delta（可能为空）→ 关 block。
// 用于上游不发 output_item.done 的场景（如 openai→responses 对 reasoning/tool_call），
// 以及空 args 的并行 tool_call（无 delta 触发接管、done 兜底也跳过时）补发空 block。
// 候选筛选复用 collectPendingAnthropicBlockIndices，与 openNextPendingAnthropicBlock 一致。
func flushRemainingAnthropicBuffers(state *realtimeStreamState) error {
	indices := collectPendingAnthropicBlockIndices(state)
	if len(indices) == 0 {
		return nil
	}

	for _, idx := range indices {
		meta, ok := state.anthropicItemMeta[idx]
		if !ok {
			meta = anthropicItemMeta{itemType: "message"}
		}
		blockIndex := getOrAllocAnthropicBlockIndex(state, idx)
		if err := openAnthropicBlockForItem(state, blockIndex, idx, meta); err != nil {
			return err
		}
		for _, bd := range state.anthropicDeltaBuffer[idx] {
			contentDelta := map[string]interface{}{
				"type":  "content_block_delta",
				"index": blockIndex,
				"delta": bd.inner,
			}
			if err := writeRealtimeEventJSONData(state, "content_block_delta", contentDelta); err != nil {
				return err
			}
		}
		blockStop := map[string]interface{}{
			"type":  "content_block_stop",
			"index": blockIndex,
		}
		if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
			return err
		}
		delete(state.anthropicDeltaBuffer, idx)
	}
	state.anthropicActiveBlockIndex = -1
	state.anthropicActiveOutputIndex = -1
	return nil
}

// openAnthropicBlockForItem 根据 item 元数据构造对应类型的 Anthropic content_block
// 并发送 content_block_start，然后标记该 block 为 active，并记录 outputIndex 已 started。
// 调用方负责在调用前关闭前一个 active block（如需要）——本函数只「开 block」不「关 active」，
// 关 active 由 output_item.done 驱动，避免并行 item.added 提前关掉仍需接收 delta 的 block。
//
// outputIndex 用于标记 anthropicStartedOutputIndices（开 block 与标记 started 天然原子，
// 调用方无需额外标记），让 done/completed 兜底能区分「已开 block」与「未开 block 待补」
// （空 args 的并行 tool_call 场景）。
//
// 映射 Responses item 类型 → Anthropic content_block 类型：
// 未知类型一律降级为 text（Anthropic 只认 text/thinking/tool_use/image 等固定几种，
// 把 Responses 的 item 类型原样透出会产出非法 content_block）。
func openAnthropicBlockForItem(state *realtimeStreamState, blockIndex int, outputIndex int, meta anthropicItemMeta) error {
	contentBlock := map[string]interface{}{
		"type": "text",
		"text": "",
	}
	switch meta.itemType {
	case "function_call":
		contentBlock = map[string]interface{}{
			"type": "tool_use",
			"id":   meta.itemID,
			"name": meta.itemName,
		}
	case "reasoning":
		contentBlock = map[string]interface{}{
			"type": "thinking",
		}
	}

	blockStart := map[string]interface{}{
		"type":          "content_block_start",
		"index":         blockIndex,
		"content_block": contentBlock,
	}
	if err := writeRealtimeEventJSONData(state, "content_block_start", blockStart); err != nil {
		return err
	}
	state.anthropicActiveBlockIndex = blockIndex
	if state.anthropicStartedOutputIndices == nil {
		state.anthropicStartedOutputIndices = map[int]bool{}
	}
	state.anthropicStartedOutputIndices[outputIndex] = true
	return nil
}

// collectPendingAnthropicBlockIndices 收集需要补发 block 的 output_index（升序），
// 供 openNextPendingAnthropicBlock（done 驱动）和 flushRemainingAnthropicBuffers（completed 兜底）共用。
// 候选条件：有 meta、未 started、且（有缓冲 delta 或 类型需要补发空 block）。
// message 类型无 delta 时不补发空 text block（保持 NoDelta 不变量——空 text block 无意义）。
// function_call/reasoning 无 delta 仍补发空 block（tool_use/thinking block 对客户端有语义意义）。
func collectPendingAnthropicBlockIndices(state *realtimeStreamState) []int {
	var indices []int
	for idx, meta := range state.anthropicItemMeta {
		if state.anthropicStartedOutputIndices != nil && state.anthropicStartedOutputIndices[idx] {
			continue
		}
		hasBuf := len(state.anthropicDeltaBuffer[idx]) > 0
		if !hasBuf && meta.itemType == "message" {
			continue
		}
		indices = append(indices, idx)
	}
	// 升序排序，保证 block 顺序与 output_index 顺序一致。
	for i := 1; i < len(indices); i++ {
		for j := i; j > 0 && indices[j-1] > indices[j]; j-- {
			indices[j-1], indices[j] = indices[j], indices[j-1]
		}
	}
	return indices
}

// emitOrBufferAnthropicDelta 处理 reasoning/args delta 的并行顺序化：
//   - outputIndex 已 done（output_item.done 已到）→ 丢弃，返回 nil（迟到的 delta 不再发）
//   - outputIndex == activeOutputIndex，或 activeOutputIndex < 0（无 active）→
//     接管为 active：若 block 未开则取 meta 调 openAnthropicBlockForItem 开 block，设 activeOutputIndex，发 delta
//   - 否则（有 active 但不是本 outputIndex）→ 缓冲到 anthropicDeltaBuffer，
//     由 output_item.done 或 completed 兜底 flush
//
// 注意：reasoning/args delta 不主动关别的 active block（与 text delta 不同），
// 因为并行 tool_call 的 args 是交错的，主动关会丢数据。text delta 走
// switchToAnthropicTextBlock 主动切换（reasoning 先于 content 的顺序场景，content 到达时 reasoning 已发完）。
func emitOrBufferAnthropicDelta(state *realtimeStreamState, outputIndex int, inner map[string]interface{}) error {
	if state.anthropicDoneOutputIndices != nil && state.anthropicDoneOutputIndices[outputIndex] {
		return nil
	}

	// 无 active：接管为 active（item.added 未开块的防御性兜底，或并行场景中首个 delta）。
	if state.anthropicActiveOutputIndex < 0 {
		blockIndex := getOrAllocAnthropicBlockIndex(state, outputIndex)
		meta, ok := state.anthropicItemMeta[outputIndex]
		if !ok {
			meta = anthropicItemMeta{itemType: "message"}
		}
		if err := openAnthropicBlockForItem(state, blockIndex, outputIndex, meta); err != nil {
			return err
		}
		state.anthropicActiveOutputIndex = outputIndex
	}

	if state.anthropicActiveOutputIndex == outputIndex {
		contentDelta := map[string]interface{}{
			"type":  "content_block_delta",
			"index": getAnthropicBlockIndex(state, outputIndex),
			"delta": inner,
		}
		return writeRealtimeEventJSONData(state, "content_block_delta", contentDelta)
	}

	// 非 active：缓冲，等 output_item.done 或 completed 兜底 flush。
	if state.anthropicDeltaBuffer == nil {
		state.anthropicDeltaBuffer = map[int][]anthropicBufferedDelta{}
	}
	state.anthropicDeltaBuffer[outputIndex] = append(state.anthropicDeltaBuffer[outputIndex], anthropicBufferedDelta{inner: inner})
	return nil
}

// switchToAnthropicTextBlock 确保 outputIndex 对应的 text block 是 active：
// 若已是 active 直接返回；若 active 是别的 output_index（如 reasoning），先关掉它再开 text block。
// 用于 text delta 的「主动切换」语义——reasoning 先于 content 的顺序场景中，
// content 到达时 reasoning 阶段已结束，主动关 thinking 开 text 是正确的。
// （并行 tool_call + text 的场景极少，且 Anthropic 也不支持 text 与 tool_use 并行，主动切换可接受。）
func switchToAnthropicTextBlock(state *realtimeStreamState, outputIndex int) error {
	if state.anthropicActiveOutputIndex == outputIndex && state.anthropicActiveBlockIndex >= 0 {
		return nil
	}
	blockIndex := getOrAllocAnthropicBlockIndex(state, outputIndex)
	if state.anthropicActiveBlockIndex >= 0 {
		if err := closeAnthropicActiveBlockIfNeeded(state, blockIndex); err != nil {
			return err
		}
		state.anthropicActiveOutputIndex = -1
	}
	if err := openAnthropicBlockForItem(state, blockIndex, outputIndex, anthropicItemMeta{itemType: "message"}); err != nil {
		return err
	}
	state.anthropicActiveOutputIndex = outputIndex
	return nil
}

func handleResponsesToAnthropicOutputTextDeltaEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	delta := ev.Delta
	if delta == "" {
		return nil
	}
	// text block 惰性开启 + 主动切换：message item 的 output_item.added 只预留映射，
	// 第一个 output_text.delta 到达时开 text block；若当前 active 是 reasoning，
	// 主动关 reasoning 开 text（reasoning 先于 content 的顺序场景）。
	if err := switchToAnthropicTextBlock(state, ev.OutputIndex); err != nil {
		return err
	}
	contentDelta := map[string]interface{}{
		"type":  "content_block_delta",
		"index": getAnthropicBlockIndex(state, ev.OutputIndex),
		"delta": map[string]interface{}{
			"type": "text_delta",
			"text": delta,
		},
	}
	return writeRealtimeEventJSONData(state, "content_block_delta", contentDelta)
}

func handleResponsesToAnthropicReasoningDeltaEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	delta := ev.Delta
	if delta == "" {
		return nil
	}
	// reasoning delta 不缓冲：thinking block 一旦被 text delta 切走（switchToAnthropicTextBlock），
	// 重开会分裂成多个 thinking block（同 index 重复 start 违反 Anthropic 不变量，分配新 index
	// 则语义分裂）。reasoning 被 text 切走后迟到的 delta 属上游协议异常（正常推理模型 reasoning
	// 先于 content），丢弃比分裂更安全——优先保 text（最终输出）完整。
	// 与 args delta（多 tool_call 并行是真实场景，需缓冲）不同。
	if state.anthropicActiveOutputIndex != ev.OutputIndex {
		return nil
	}
	contentDelta := map[string]interface{}{
		"type":  "content_block_delta",
		"index": getAnthropicBlockIndex(state, ev.OutputIndex),
		"delta": map[string]interface{}{
			"type":     "thinking_delta",
			"thinking": delta,
		},
	}
	return writeRealtimeEventJSONData(state, "content_block_delta", contentDelta)
}

func handleResponsesToAnthropicFunctionArgsDeltaEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	delta := ev.Delta
	if delta == "" {
		return nil
	}
	return emitOrBufferAnthropicDelta(state, ev.OutputIndex, map[string]interface{}{
		"type":         "input_json_delta",
		"partial_json": delta,
	})
}

func handleResponsesToAnthropicCompletedEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	stopReason := "end_turn"
	if ev.Response != nil && ev.Response.Status != nil && *ev.Response.Status == "incomplete" {
		stopReason = "max_tokens"
	}
	if ev.Response != nil {
		for _, item := range ev.Response.Output {
			if item.Type == "function_call" {
				stopReason = "tool_use"
				break
			}
		}
	}

	if state.anthropicActiveBlockIndex >= 0 {
		blockStop := map[string]interface{}{
			"type":  "content_block_stop",
			"index": state.anthropicActiveBlockIndex,
		}
		if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
			return err
		}
		state.anthropicActiveBlockIndex = -1
		state.anthropicActiveOutputIndex = -1
	}

	// 兜底 flush：上游（如 openai→responses）可能不发 reasoning/tool_call 的 output_item.done，
	// 导致这些 item 的 delta 仍缓冲在 anthropicDeltaBuffer 里。在 message_delta 前按 output_index
	// 升序逐个开 block → flush delta → 关 block，保证参数完整不丢。
	// 注意：active item 的 delta 永远直发不缓冲，其缓冲必为空，flushRemainingAnthropicBuffers
	// 不会重复处理它。若未来 emitOrBufferAnthropicDelta 逻辑变更导致 active item 也缓冲，
	// 需在此前 delete(state.anthropicDeltaBuffer, state.anthropicActiveOutputIndex) 防御。
	if err := flushRemainingAnthropicBuffers(state); err != nil {
		return err
	}

	messageDelta := map[string]interface{}{
		"type": "message_delta",
		"delta": map[string]interface{}{
			"stop_reason": stopReason,
		},
	}

	// 添加 usage 信息
	if ev.Response != nil && ev.Response.Usage != nil {
		messageDelta["usage"] = map[string]interface{}{
			"input_tokens":  int(ev.Response.Usage.InputTokens),
			"output_tokens": int(ev.Response.Usage.OutputTokens),
		}
	}

	if err := writeRealtimeEventJSONData(state, "message_delta", messageDelta); err != nil {
		return err
	}

	messageStop := map[string]interface{}{"type": "message_stop"}
	return writeRealtimeEventJSONData(state, "message_stop", messageStop)
}

func handleResponsesToAnthropicContentPartAddedEvent(state *realtimeStreamState, ev *responses.ResponsesStreamEvent) error {
	if ev.Part == nil {
		return nil
	}
	if ev.Part.Type != "image_url" && ev.Part.Type != "output_image" && ev.Part.Type != "input_image" && ev.Part.Type != "image" {
		return nil
	}

	imageURL, detail := extractResponsesContentPartImageURL(ev.Part)
	if imageURL == "" {
		return nil
	}

	// 记录被 image 打断的 active output_index。
	interruptedOutputIndex := state.anthropicActiveOutputIndex

	blockIndex := allocAnthropicBlockIndex(state)
	if err := closeAnthropicActiveBlockIfNeeded(state, blockIndex); err != nil {
		return err
	}

	block := map[string]interface{}{
		"type": "image",
	}
	if mediaType, data, ok := common.ParseBase64DataURL(imageURL); ok {
		block["source"] = map[string]interface{}{
			"type":       "base64",
			"media_type": mediaType,
			"data":       data,
		}
	} else {
		block["source"] = map[string]interface{}{
			"type": "url",
			"url":  imageURL,
		}
	}
	if detail != nil && *detail != "" {
		block["detail"] = *detail
	}

	blockStart := map[string]interface{}{
		"type":          "content_block_start",
		"index":         blockIndex,
		"content_block": block,
	}
	if err := writeRealtimeEventJSONData(state, "content_block_start", blockStart); err != nil {
		return err
	}

	blockStop := map[string]interface{}{
		"type":  "content_block_stop",
		"index": blockIndex,
	}
	if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
		return err
	}

	state.anthropicActiveBlockIndex = -1
	// 同步清 activeOutputIndex：image 是单发完即关，关 block 后无 active，
	// 后续 delta 应走 emitOrBufferAnthropicDelta 的「无 active 接管」或缓冲分支，
	// 而非误判前一个 item 仍 active（会导致 delta 发到已 stop 的 block）。
	state.anthropicActiveOutputIndex = -1

	// 清掉被打断的 output_index 的 block index 映射，让后续 delta 分配新 block index
	// （避免重复 start 同一个 block——Anthropic 不允许同 index 重复 start）。
	// 不清 started 标记：该 block 确实 start 过（被 image 打断），flushRemainingAnthropicBuffers
	// 据此跳过它不会误补发空 block。后续 delta 走「无 active 接管」分支，getOrAllocAnthropicBlockIndex
	// 分配新 index 后 openAnthropicBlockForItem 标记 started=true（已是 true，无影响）。
	// 职责边界：映射管「本次 block index 分配」，started 管「start 事件是否发放」。
	// image 打断后形成「映射=无、started=true」中间态是有意为之。
	// 极端场景下同一 tool_call 的 args 分裂到两个 block，但比重复 start（协议违规）
	// 和 args 丢失（功能 bug）要好——image 穿插并行 tool_call 本身是上游协议异常。
	if interruptedOutputIndex >= 0 {
		if state.responsesOutputIndexToAnthropicBlockIndex != nil {
			delete(state.responsesOutputIndexToAnthropicBlockIndex, interruptedOutputIndex)
		}
	}
	return nil
}

func closeAnthropicActiveBlockIfNeeded(state *realtimeStreamState, nextIndex int) error {
	if state.anthropicActiveBlockIndex < 0 || state.anthropicActiveBlockIndex == nextIndex {
		return nil
	}
	blockStop := map[string]interface{}{
		"type":  "content_block_stop",
		"index": state.anthropicActiveBlockIndex,
	}
	if err := writeRealtimeEventJSONData(state, "content_block_stop", blockStop); err != nil {
		return err
	}
	state.anthropicActiveBlockIndex = -1
	return nil
}

func getOrAllocAnthropicBlockIndex(state *realtimeStreamState, outputIndex int) int {
	if state.responsesOutputIndexToAnthropicBlockIndex == nil {
		state.responsesOutputIndexToAnthropicBlockIndex = map[int]int{}
	}
	if idx, ok := state.responsesOutputIndexToAnthropicBlockIndex[outputIndex]; ok {
		return idx
	}
	idx := allocAnthropicBlockIndex(state)
	state.responsesOutputIndexToAnthropicBlockIndex[outputIndex] = idx
	return idx
}

func getAnthropicBlockIndex(state *realtimeStreamState, outputIndex int) int {
	if state.responsesOutputIndexToAnthropicBlockIndex == nil {
		return outputIndex
	}
	if idx, ok := state.responsesOutputIndexToAnthropicBlockIndex[outputIndex]; ok {
		return idx
	}
	return outputIndex
}

func allocAnthropicBlockIndex(state *realtimeStreamState) int {
	if state.anthropicNextBlockIndex < 0 {
		state.anthropicNextBlockIndex = 0
	}
	idx := state.anthropicNextBlockIndex
	state.anthropicNextBlockIndex++
	return idx
}

func extractResponsesContentPartImageURL(part *responses.ResponsesContentPart) (string, *string) {
	if part == nil {
		return "", nil
	}
	if part.ImageURL != nil && len(*part.ImageURL) > 0 {
		var urlStr string
		if err := json.Unmarshal(*part.ImageURL, &urlStr); err == nil && urlStr != "" {
			return urlStr, part.Detail
		}
		var obj struct {
			URL    string  `json:"url"`
			Detail *string `json:"detail,omitempty"`
		}
		if err := json.Unmarshal(*part.ImageURL, &obj); err == nil && obj.URL != "" {
			if obj.Detail != nil && *obj.Detail != "" {
				return obj.URL, obj.Detail
			}
			return obj.URL, part.Detail
		}
	}
	if part.URL != nil && *part.URL != "" {
		return *part.URL, part.Detail
	}
	return "", part.Detail
}
