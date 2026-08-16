package streaming

import (
	"encoding/json"
	"fmt"

	"github.com/qkf688/llmux/common/maputil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/shared"
)

func init() {
	RegisterRealtimeRoute("anthropic", "openai-res", handleRealtimeAnthropicToResponses)
}

func handleRealtimeAnthropicToResponses(state *realtimeStreamState, data string) error {
	// Anthropic 不发送 [DONE]，忽略
	if data == "[DONE]" {
		return nil
	}

	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		logRealtimeChunkParseError(state, data, err)
		return nil
	}

	eventType := state.currentEvent
	if eventType == "" {
		eventType = maputil.String(chunk, "type")
	}

	switch eventType {
	case "message_start":
		return handleAnthropicToResponsesMessageStart(state, chunk)
	case "content_block_start":
		return handleAnthropicToResponsesContentBlockStart(state, chunk)
	case "content_block_delta":
		return handleAnthropicToResponsesContentBlockDelta(state, chunk)
	case "message_delta":
		return handleAnthropicToResponsesMessageDelta(state, chunk)
	case "message_stop":
		// Responses API 不需要单独的 stop 事件
		return nil
	default:
		return nil
	}
}

func handleAnthropicToResponsesMessageStart(state *realtimeStreamState, chunk map[string]interface{}) error {
	// 获取 message ID 并生成 item ID
	state.responseID = shared.GetNestedString(chunk, "message.id")
	state.itemID = fmt.Sprintf("msg_%s", state.responseID)
	createdAt := int(shared.GetNestedFloat(chunk, "message.created_at"))

	// input 侧 token 只在 message_start 出现（旧版上游的 message_delta 不重复带），
	// 缓存下来供 message_delta 构建 response.completed.usage 时合并。
	if message, ok := chunk["message"].(map[string]interface{}); ok {
		if usage, ok := message["usage"].(map[string]interface{}); ok {
			state.anthropicStartUsage = usage
			// 同时立刻交一份给侧信道：上游若在 message_start 之后异常结束
			// （连接中断 / error 事件 / 无 delta 的空回复），message_delta 分支
			// 不会执行，input 侧真值（缓存命中数是计费大头）就永久丢了。
			// 后续 message_delta 交的是含 input 侧的合并快照，是本次的超集，
			// 「最后观测胜出」不会造成降级。
			captureUpstreamUsageMap(state, usage)
		}
	}

	// 发送 response.created 事件
	responseCreated := map[string]interface{}{
		"type":            "response.created",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"object":     "response",
			"id":         state.responseID,
			"model":      shared.GetNestedString(chunk, "message.model"),
			"created_at": createdAt,
			"output":     []interface{}{},
			"status":     "in_progress",
		},
	}
	if err := writeRealtimeOrderedData(state, responseCreated); err != nil {
		return err
	}

	// 发送 response.in_progress 事件
	responseInProgress := map[string]interface{}{
		"type":            "response.in_progress",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"object":     "response",
			"id":         state.responseID,
			"model":      shared.GetNestedString(chunk, "message.model"),
			"created_at": createdAt,
			"output":     []interface{}{},
			"status":     "in_progress",
		},
	}
	return writeRealtimeOrderedData(state, responseInProgress)
}

func handleAnthropicToResponsesContentBlockStart(state *realtimeStreamState, chunk map[string]interface{}) error {
	contentBlock, ok := chunk["content_block"].(map[string]interface{})
	if !ok {
		return nil
	}
	blockType := maputil.String(contentBlock, "type")
	blockIndex := int(maputil.Float64(chunk, "index"))

	if blockType == "thinking" {
		// 处理 thinking 类型的 content block（Extended Thinking）
		if state.hasReasoningItem {
			return nil
		}
		state.hasReasoningItem = true
		state.reasoningItemID = fmt.Sprintf("reasoning_%s", state.responseID)
		state.reasoningOutputIndex = blockIndex

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

		state.accumulatedReasoning = ""
		return nil
	}

	if blockType == "tool_use" {
		// 工具调用块：必须映射成 function_call item 并透传 id/name，
		// 落进下面的通用分支会被包成 type="message"，id 与 name 一起丢掉。
		toolID := maputil.String(contentBlock, "id")
		item := map[string]interface{}{
			"type":      "function_call",
			"id":        toolID,
			"call_id":   toolID,
			"name":      maputil.String(contentBlock, "name"),
			"arguments": "",
		}
		state.responsesOutput.set(blockIndex, item)

		toolItemAdded := map[string]interface{}{
			"type":            "response.output_item.added",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    blockIndex,
			"item":            item,
		}
		return writeRealtimeOrderedData(state, toolItemAdded)
	}

	// 处理普通文本类型的 content block
	if !state.hasMessageOutputIndex {
		state.messageOutputIndex = blockIndex
		state.hasMessageOutputIndex = true
	}

	itemAdded := map[string]interface{}{
		"type":            "response.output_item.added",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    blockIndex,
		"item": map[string]interface{}{
			"id":      state.itemID,
			"type":    "message",
			"role":    "assistant",
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
		"output_index":    blockIndex,
		"item_id":         state.itemID,
		"content_index":   0,
		"part": map[string]interface{}{
			"type": "output_text",
			"text": "",
		},
	}
	if err := writeRealtimeOrderedData(state, contentPartAdded); err != nil {
		return err
	}

	state.accumulatedText = ""
	return nil
}

func handleAnthropicToResponsesContentBlockDelta(state *realtimeStreamState, chunk map[string]interface{}) error {
	delta, ok := chunk["delta"].(map[string]interface{})
	if !ok {
		return nil
	}
	deltaType := maputil.String(delta, "type")

	if deltaType == "thinking_delta" {
		text := maputil.String(delta, "thinking")
		if text == "" {
			return nil
		}
		state.accumulatedReasoning += text
		reasoningDelta := map[string]interface{}{
			"type":            "response.reasoning_summary_text.delta",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item_id":         state.reasoningItemID,
			"summary_index":   0,
			"delta":           text,
		}
		return writeRealtimeOrderedData(state, reasoningDelta)
	}

	if deltaType == "text_delta" {
		text := maputil.String(delta, "text")
		if text == "" {
			return nil
		}
		state.accumulatedText += text
		textDelta := map[string]interface{}{
			"type":            "response.output_text.delta",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    int(maputil.Float64(chunk, "index")),
			"item_id":         state.itemID,
			"content_index":   0,
			"delta":           text,
		}
		return writeRealtimeOrderedData(state, textDelta)
	}

	if deltaType == "input_json_delta" {
		partialJSON := maputil.String(delta, "partial_json")
		if partialJSON == "" {
			return nil
		}
		blockIndex := int(maputil.Float64(chunk, "index"))
		state.responsesOutput.appendArguments(blockIndex, partialJSON)

		argsDelta := map[string]interface{}{
			"type":            "response.function_call_arguments.delta",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    blockIndex,
			"delta":           partialJSON,
		}
		return writeRealtimeOrderedData(state, argsDelta)
	}

	return nil
}

func handleAnthropicToResponsesMessageDelta(state *realtimeStreamState, chunk map[string]interface{}) error {
	// 如果有 reasoning 内容，先发送 reasoning 完成事件
	if state.hasReasoningItem && state.accumulatedReasoning != "" {
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

		reasoningItemDone := map[string]interface{}{
			"type":            "response.output_item.done",
			"sequence_number": nextRealtimeSequence(state),
			"output_index":    state.reasoningOutputIndex,
			"item": map[string]interface{}{
				"id":   state.reasoningItemID,
				"type": "reasoning",
				"summary": []map[string]interface{}{
					{
						"type": "summary_text",
						"text": state.accumulatedReasoning,
					},
				},
				"status": "completed",
			},
		}
		if err := writeRealtimeOrderedData(state, reasoningItemDone); err != nil {
			return err
		}

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
	}

	// 只有真的出现过文本块才发 message 类型 done 事件：纯 tool_use 流不应再带
	// output_text.done / content_part.done / output_item.done(message)，否则
	// messageOutputIndex=0 会与 blockIndex=0 的 function_call 撞车，
	// 且事件流本身冗余。reasoning 路径在上面已独立发完，与这里互不影响。
	if err := emitMessageStreamFinalization(state); err != nil {
		return err
	}

	status := "completed"
	if delta, ok := chunk["delta"].(map[string]interface{}); ok {
		if reason := maputil.String(delta, "stop_reason"); reason != "" && reason == "max_tokens" {
			status = "incomplete"
		}
	}

	responseCompleted := map[string]interface{}{
		"type":            "response.completed",
		"sequence_number": nextRealtimeSequence(state),
		"response": map[string]interface{}{
			"object":     "response",
			"id":         state.responseID,
			"model":      shared.GetNestedString(chunk, "message.model"),
			"created_at": 0,
			// 必须是真实产出：下游靠扫这个数组里有没有 function_call 判定 finish_reason。
			"output": state.responsesOutput.snapshot(),
			"status": status,
		},
	}

	// 合并 usage：input 侧在 message_start，output 侧在 message_delta，两处各带一部分，
	// 且各家上游键名不一（anthropic 原生用 input_tokens/cache_read_input_tokens，
	// kimi 一类只回 openai 兼容的 completion_tokens/cached_tokens）。合并成完整快照后
	// 整体交候选表识别——**不在此白名单键名**，否则只回兼容字段的上游会被过滤成全零
	// 并被侧信道丢弃（正是非流 ParseResponse 修掉的那个分叉）。
	deltaUsage, _ := chunk["usage"].(map[string]interface{})
	if deltaUsage != nil || state.anthropicStartUsage != nil {
		mergedUsage := mergeAnthropicUsage(state.anthropicStartUsage, deltaUsage)

		// 完整快照交侧信道：它是 message_start 那次的超集，覆盖不会降级。
		captureUpstreamUsageMap(state, mergedUsage)

		// 出站 usage 走同一归一入口 models.UsageFromMap，与落库口径同源
		// （含 total 采用上游值的回退），避免「客户端看 X、DB 记 Y」的分叉；
		// 装配交 responsesUsageFromModel，零值明细判据与另两个写出点共用一份实现。
		//
		// 两条 anthropic 专属结论（判据本身在被复用的实现里，此处只记为什么成立）：
		// ① cache_creation 不参与 cached_tokens——上游首次写缓存时 cache_creation>0
		// 而 cache_read=0，认它就会写出 cached_tokens:0；models 的候选表只收
		// cache_read_input_tokens，故不会误取。
		// ② 原生 Anthropic 协议无 reasoning 槽位（thinking 计入 output_tokens），
		// 归一结果 reasoning 通常为 0，装配器不写出 output_tokens_details（写
		// reasoning_tokens:0 会让下游误以为上游明确报告了 0）。若混合形状上游
		// （走 anthropic 协议但附带 openai 兼容键）经候选表带出 reasoning 真值，
		// 装配器会按与落库同口径写出——这正是本系列要的「客户端与 DB 同源」。
		responseCompleted["response"].(map[string]interface{})["usage"] =
			responsesUsageFromModel(models.UsageFromMap(mergedUsage))
	}

	return writeRealtimeOrderedData(state, responseCompleted)
}

// mergeAnthropicUsage 合并 message_start 与 message_delta 两处 usage 为一份完整快照。
//
// Anthropic 把 usage 拆两处：input 侧（含缓存命中，计费大头）在 message_start，
// output 侧在 message_delta。合并后整体交 models.UsageFromMap 的候选表识别键名，
// 而非在调用点白名单——否则只回 openai 兼容字段的上游会漏统计。
//
// delta 是更晚的观测，其非零值覆盖 start；delta 缺失或显式带 0 的键保留 start 真值
// （旧版上游的 message_delta 不重复带 input 侧，不能用 0 抹掉 start 的 input）。
func mergeAnthropicUsage(start, delta map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(start)+len(delta))
	for k, v := range start {
		merged[k] = v
	}
	for k, v := range delta {
		if f, ok := v.(float64); ok && f <= 0 {
			continue
		}
		merged[k] = v
	}
	return merged
}

// emitMessageStreamFinalization 发出 message 类型 content 块的收尾事件集：
// output_text.done → content_part.done → output_item.done(message)。
// 仅在 hasMessageOutputIndex 为真时执行，纯 tool_use 流不会进入此分支。
// 抽出的理由：原逻辑嵌在 handleAnthropicToResponsesMessageDelta 中导致该函数 80→130 行膨胀；
// 独立函数后 reasoning 路径与 message 路径边界更清晰（SRP），便于复用与单测。
func emitMessageStreamFinalization(state *realtimeStreamState) error {
	if !state.hasMessageOutputIndex {
		return nil
	}
	msgIndex := state.messageOutputIndex

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

	contentItems := []map[string]interface{}{
		{
			"type": "output_text",
			"text": state.accumulatedText,
		},
	}
	// 如果有 reasoning，添加到 content 数组前面
	if state.hasReasoningItem && state.accumulatedReasoning != "" {
		contentItems = append([]map[string]interface{}{
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
		}, contentItems...)
	}

	messageItem := map[string]interface{}{
		"id":      state.itemID,
		"type":    "message",
		"role":    "assistant",
		"content": contentItems,
		"status":  "completed",
	}
	// 只有真的出现过文本块才记入 output：否则纯 tool_use 的流会把
	// index 0 上的 function_call 覆盖掉。
	state.responsesOutput.set(msgIndex, messageItem)

	outputItemDone := map[string]interface{}{
		"type":            "response.output_item.done",
		"sequence_number": nextRealtimeSequence(state),
		"output_index":    msgIndex,
		"item":            messageItem,
	}
	return writeRealtimeOrderedData(state, outputItemDone)
}
