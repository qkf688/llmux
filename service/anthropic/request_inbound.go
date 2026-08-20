package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/shared"
)

// TransformToUnified 将 Anthropic 请求格式转换为统一格式。
//
// 顶层字段与 system / tools / thinking / output_config / tool_choice 走 anthropicRequest
// 及其嵌套 DTO（见 request_dto.go）；messages 及其 content 块也走 anthropicMessage +
// envelope-peek + 块子 struct（见 request_dto.go），请求入站已无 map 逐键取值。
// DTO 的字段容器全部宽容（类型不匹配 = 当作未传，不报错），故本函数只在 body 不是
// JSON 对象时返回错误。
//
// 与 map 时代的对外差异只有一条：键匹配由精确查找变成 encoding/json 的「先精确、
// 未命中再 EqualFold」，即 `{"Max_Tokens":100}`、`{"thinking":{"Budget_Tokens":30000}}`
// 现在能解析（旧版静默丢失）。这是刻意接受的改善——openai 入站早已是 DTO（同样大小写
// 不敏感），`shared.UnknownTopLevelKeys` 的 isClaimedKey 也做 EqualFold 兜底；旧版
// anthropic 的「检测认为 Temperature 已认领、解析却读不到它」才是两侧不一致。
func TransformToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	var req anthropicRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, err
	}

	unified := &models.UnifiedRequest{
		Model:  req.Model.Value,
		Stream: req.Stream.Value,
	}
	unified.System, unified.SystemParts = parseSystem(req.System)

	if req.MaxTokens.Set {
		unified.MaxTokens = req.MaxTokens.Value
	}
	if req.Temperature.Set {
		unified.Temperature = &req.Temperature.Value
	}
	if req.TopP.Set {
		unified.TopP = &req.TopP.Value
	}

	unified.Messages = parseMessages(req.Messages)
	unified.Tools = parseTools(req.Tools)

	if req.StopSequences.Set && len(req.StopSequences.Value) > 0 {
		unified.Stop = &models.UnifiedStop{Multiple: req.StopSequences.Value}
	}

	if req.Metadata.Set && len(req.Metadata.Value) > 0 {
		unified.Metadata = req.Metadata.Value
	}

	var thinking anthropicThinking
	if shared.DecodeJSONObject(req.Thinking, &thinking) {
		budgetTokens := thinking.BudgetTokens.Value
		if thinking.Type.Value == "enabled" && budgetTokens > 0 {
			effort := ThinkingBudgetToReasoningEffort(budgetTokens)
			if effort != "" {
				unified.ReasoningEffort = &effort
			}
			unified.ReasoningBudget = &budgetTokens
		}
	}

	// output_config.effort 是 Claude 4.6 adaptive thinking 的显式档位字段，
	// 优先级高于 thinking.budget_tokens 反推（显式意图优先）。
	// 归一化开关与 OpenAI/Responses 入站一致：开启时走 NormalizeReasoningEffort，
	// 关闭时原值透传（用户显式关闭映射设置后，跨协议行为对称）；
	// budget 仍取 thinking.budget_tokens（两字段并存，出站 budget 优先已有实现不变）。
	var outputConfig anthropicOutputConfig
	if shared.DecodeJSONObject(req.OutputConfig, &outputConfig) {
		if effortStr := outputConfig.Effort.Value; effortStr != "" {
			effort := effortStr
			if shared.GetReasoningEffortMappingEnabled(ctx) {
				effort = shared.NormalizeReasoningEffort(ctx, effortStr)
			}
			unified.ReasoningEffort = &effort
		}
	}

	// tool_choice (best-effort): keep unified semantics as OpenAI-style tool_choice.
	unified.ToolChoice = parseToolChoice(req.ToolChoice)

	return unified, nil
}

// parseToolChoice 解析 Anthropic 的 tool_choice：裸字符串 ｜ 对象两种形态。
//
// 非 "tool" 的 type（auto / any / none）作为 StringValue 透传——统一模型不设
// Anthropic 专属枚举，透传把「落地成什么」留给出站适配器。两个值都没拿到时返回
// nil 而不是空壳：空壳会让出站 emit 出 `"tool_choice":{}`，上游据此判 400。
func parseToolChoice(raw json.RawMessage) *models.UnifiedToolChoice {
	if len(raw) == 0 || shared.IsJSONNull(raw) {
		return nil
	}

	choice := &models.UnifiedToolChoice{}
	if value, ok := shared.RawString(raw); ok {
		if value != "" {
			choice.StringValue = &value
		}
	} else {
		var obj anthropicToolChoice
		if shared.DecodeJSONObject(raw, &obj) {
			switch tcType := obj.Type.Value; tcType {
			case "tool":
				if name := obj.Name.Value; name != "" {
					choice.ObjectValue = &models.UnifiedToolChoiceObject{
						Type: "function",
						Function: &models.UnifiedToolChoiceFunction{
							Name: name,
						},
					}
				}
			default:
				if tcType != "" {
					choice.StringValue = &tcType
				}
			}
		}
	}

	if choice.StringValue == nil && choice.ObjectValue == nil {
		return nil
	}
	return choice
}

// decodeRawArray 把一段 JSON 解成数组的原始元素切片，是本包所有「数组形字段」
// （messages / content / tools / system 数组分支）的唯一入口。
//
// 用 err 判定区分「不是数组」（返回 false）与「空数组」（返回 true + 空切片），
// 与 map 时代 asSlice 的两种结果一致。**不用 shared.RawArray**——它吞掉「不是
// 数组」这个错误，会把标量值（如 `"system":123`）与空数组 `[]` 混为一谈，从而把
// 「非数组 → 返回 nil」误改成「返回空切片」。
func decodeRawArray(raw json.RawMessage) ([]json.RawMessage, bool) {
	if len(raw) == 0 || shared.IsJSONNull(raw) {
		return nil, false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, false
	}
	return items, true
}

func parseMessages(raw json.RawMessage) []models.UnifiedMessage {
	items, ok := decodeRawArray(raw)
	if !ok {
		return nil
	}

	messages := make([]models.UnifiedMessage, 0, len(items))
	for _, item := range items {
		var msgDTO anthropicMessage
		if !shared.DecodeJSONObject(item, &msgDTO) {
			continue
		}

		// 单次遍历同一份 content，一并产出 content parts / tool_result 消息 /
		// tool_calls / reasoning（#28：此前对同一份 content 连扫三遍）。
		parsed := parseContentBlocks(msgDTO.Content)
		msg := models.UnifiedMessage{
			Role:      msgDTO.Role.Value,
			Content:   parsed.content,
			ToolCalls: parsed.toolCalls,
		}
		msg.CacheControl = parseCacheControl(msgDTO.CacheControl)

		if parsed.reasoning != "" {
			reasoning := parsed.reasoning
			msg.ReasoningContent = &reasoning
			if parsed.signature != "" {
				signature := parsed.signature
				msg.ReasoningSignature = &signature
			}
		}
		if parsed.redactedData != "" {
			redactedData := parsed.redactedData
			msg.RedactedThinkingData = &redactedData
		}

		// tool_result blocks are mapped to OpenAI-style tool messages to enable cross-format conversion.
		if len(parsed.toolResultMessages) > 0 {
			messages = append(messages, parsed.toolResultMessages...)
		}

		// If a user message only contains tool_result blocks, skip the original message.
		if msg.Role == "user" && parsed.content == nil && len(parsed.toolResultMessages) > 0 {
			continue
		}

		messages = append(messages, msg)
	}
	return messages
}

func parseTools(raw json.RawMessage) []models.UnifiedTool {
	items, ok := decodeRawArray(raw)
	if !ok {
		return nil
	}

	tools := make([]models.UnifiedTool, 0, len(items))
	for _, item := range items {
		var dto anthropicTool
		if !shared.DecodeJSONObject(item, &dto) {
			continue
		}

		tool := models.UnifiedTool{
			Type: "function",
			Function: models.UnifiedFunc{
				Name:        dto.Name.Value,
				Description: dto.Description.Value,
				Parameters:  shared.RawJSONValue(dto.InputSchema),
			},
		}
		tool.CacheControl = parseCacheControl(dto.CacheControl)
		tools = append(tools, tool)
	}
	return tools
}

func parseToolCallFromBlock(item json.RawMessage, index int) (models.UnifiedToolCall, bool) {
	var block anthropicToolUseBlock
	if !shared.DecodeJSONObject(item, &block) {
		return models.UnifiedToolCall{}, false
	}

	// input 必须经 map 往返再 Marshal，**不能**把 RawMessage 原文透传：
	// json.Marshal 对 map 按键字典序输出并去掉空格，这是 map 时代确立的
	// arguments 对外形态（arguments 直接进上游请求体）。原文透传会保留客户端
	// 键序，属对外行为变更。非对象形态（数组 / 标量 / null / 缺失）一律塌成 "{}"。
	argsStr := "{}"
	var inputMap map[string]interface{}
	if shared.DecodeJSONObject(block.Input, &inputMap) {
		if argsBytes, err := json.Marshal(inputMap); err == nil {
			argsStr = string(argsBytes)
		}
	}

	return models.UnifiedToolCall{
		ID:    block.ID.Value,
		Type:  "function",
		Index: index,
		Function: models.UnifiedToolCallFunction{
			Name:      block.Name.Value,
			Arguments: argsStr,
		},
		CacheControl: parseCacheControl(block.CacheControl),
	}, true
}

// contentBlocks 是单次遍历一份 content 后的全部产出。
//
// 六种块 type 的结果彼此正交，但都来自同一个数组，所以按「一次遍历、多路产出」
// 组织，而不是每种产出各扫一遍（#28）。
type contentBlocks struct {
	// content 是收敛后的统一模型 Content：裸 string（content 本身是字符串，或
	// collapseContentParts 把单个纯文本块降级）｜[]UnifiedMessageContentPart ｜
	// 兜底透传的 json.RawMessage ｜ nil（无有效块）。
	content            interface{}
	toolResultMessages []models.UnifiedMessage
	toolCalls          []models.UnifiedToolCall

	// reasoning 是所有 thinking 块文本的顺序拼接；signature 取**最后**一个非空值
	// （后者覆盖前者）；redactedData 取**第一个**非空值（先到先得）。两者方向相反，
	// 是既有行为，回归测试 TestTransformToUnified_MultipleRedactedThinkingKeepsFirstNonEmpty
	// 锁死了 redacted 侧。
	reasoning    string
	signature    string
	redactedData string
}

// parseContentBlocks 单次遍历一份 content（messages[].content 或响应体 content），
// 一并产出 content parts、tool_result 转出的 tool 消息、tool_calls、reasoning 三元组。
//
// 三种非数组形态的处理与拆分成三个函数的时代逐一等价：
//   - 缺失 / null → 全空
//   - 裸字符串 → 只有 content，其余为空
//   - 既非字符串也非数组（对象 / 数字等）→ content 兜底透传原始 RawMessage。
//     透传 RawMessage 而非解成 map：它满足 json.Marshaler，出站会原样输出字节，
//     比 map 重序列化更保真（保留键序与数字精度）。
func parseContentBlocks(raw json.RawMessage) contentBlocks {
	var out contentBlocks
	if len(raw) == 0 || shared.IsJSONNull(raw) {
		return out
	}

	if str, ok := shared.RawString(raw); ok {
		out.content = str
		return out
	}

	items, ok := decodeRawArray(raw)
	if !ok {
		// Keep behavior: passthrough unknown payloads (but this may reduce conversion quality).
		out.content = raw
		return out
	}

	parts := make([]models.UnifiedMessageContentPart, 0, len(items))
	out.toolCalls = make([]models.UnifiedToolCall, 0)
	var thinkingText strings.Builder

	for _, item := range items {
		var envelope anthropicContentBlockEnvelope
		if !shared.DecodeJSONObject(item, &envelope) {
			continue
		}

		switch envelope.Type.Value {
		case "tool_result":
			if toolMsg, ok := parseToolResultBlock(item); ok {
				out.toolResultMessages = append(out.toolResultMessages, toolMsg)
			}

		case "tool_use":
			if toolCall, ok := parseToolCallFromBlock(item, len(out.toolCalls)); ok {
				out.toolCalls = append(out.toolCalls, toolCall)
			}

		case "thinking":
			var block anthropicThinkingBlock
			if !shared.DecodeJSONObject(item, &block) {
				continue
			}
			thinkingText.WriteString(block.Thinking.Value)
			if sig := block.Signature.Value; sig != "" {
				out.signature = sig
			}

		case "redacted_thinking":
			// thinking 的可读文本与 signature 使用既有 reasoning 字段；
			// redacted_thinking.data 是不透明密文，必须放入独立字段，禁止与文本拼接
			// 或尝试解析。统一模型只保留一个块，判据是「当前累积值仍为空」——
			// 领头的空 data 块会被跳过，继续用后面的块填充。
			if out.redactedData == "" {
				var block anthropicRedactedThinkingBlock
				if !shared.DecodeJSONObject(item, &block) {
					continue
				}
				out.redactedData = block.Data.Value
			}

		default:
			if part, ok := parseTextOrImageBlock(item, envelope.Type.Value); ok {
				parts = append(parts, part)
			}
		}
	}

	out.content = collapseContentParts(parts)
	out.reasoning = thinkingText.String()
	return out
}

// parseToolResultBlock 把一个 tool_result 块转成 OpenAI 风格的 tool 消息。
// tool_use_id 为空时整块 skip（返回 false），既不产出 tool 消息也不进 content parts。
func parseToolResultBlock(item json.RawMessage) (models.UnifiedMessage, bool) {
	var block anthropicToolResultBlock
	if !shared.DecodeJSONObject(item, &block) {
		return models.UnifiedMessage{}, false
	}

	toolUseID := block.ToolUseID.Value
	if toolUseID == "" {
		return models.UnifiedMessage{}, false
	}

	toolMsg := models.UnifiedMessage{
		Role:         "tool",
		ToolCallID:   toolUseID,
		Content:      parseToolResultContent(block.Content),
		CacheControl: parseCacheControl(block.CacheControl),
	}
	// Optional[bool] 只在 is_error 确为 JSON 布尔时 Set，与 map 时代的
	// itemMap["is_error"].(bool) 断言等价。
	if block.IsError.Set {
		isErr := block.IsError.Value
		toolMsg.ToolCallIsError = &isErr
	}
	return toolMsg, true
}

// parseToolResultContent 解析 tool_result 的 content。
//
// 曾经这里只留 text 块，而 computer-use / 截图类工具的结果整条都是图片块，
// 于是 content 变成空串，部分上游据此判 400。现在图片块一并保留，
// 由各出站适配器决定是原样透传（Anthropic 原生支持块数组）还是降级为占位文本。
func parseToolResultContent(raw json.RawMessage) interface{} {
	if len(raw) == 0 || shared.IsJSONNull(raw) {
		return ""
	}
	if str, ok := shared.RawString(raw); ok {
		return str
	}

	items, ok := decodeRawArray(raw)
	if !ok {
		return ""
	}

	parts := make([]models.UnifiedMessageContentPart, 0, len(items))
	for _, item := range items {
		var envelope anthropicContentBlockEnvelope
		if !shared.DecodeJSONObject(item, &envelope) {
			continue
		}
		if part, ok := parseTextOrImageBlock(item, envelope.Type.Value); ok {
			parts = append(parts, part)
		}
	}

	if content := collapseContentParts(parts); content != nil {
		return content
	}
	return ""
}

// collapseContentParts 把内容块收敛成统一格式的 Content 形态：
// 单个无缓存标记的文本块降级为 string，其余保持块数组，空则为 nil。
func collapseContentParts(parts []models.UnifiedMessageContentPart) interface{} {
	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 && parts[0].Type == "text" && parts[0].Text != nil && parts[0].CacheControl == nil {
		return *parts[0].Text
	}
	return parts
}

// parseTextOrImageBlock 解析单个 text / image 块，供消息内容与 tool_result 共用。
//
// blockType 由调用方经 envelope-peek 得出并传入，避免同一份 raw 重复解判别字段。
func parseTextOrImageBlock(raw json.RawMessage, blockType string) (models.UnifiedMessageContentPart, bool) {
	switch blockType {
	case "text":
		var block anthropicTextBlock
		if !shared.DecodeJSONObject(raw, &block) {
			return models.UnifiedMessageContentPart{}, false
		}

		text := block.Text.Value
		if text == "" {
			// Some callers use "content" for text blocks.
			text = block.Content.Value
		}
		if text == "" {
			return models.UnifiedMessageContentPart{}, false
		}

		part := models.UnifiedMessageContentPart{
			Type: "text",
			Text: &text,
		}
		part.CacheControl = parseCacheControl(block.CacheControl)
		return part, true

	case "image":
		var block anthropicImageBlock
		if !shared.DecodeJSONObject(raw, &block) {
			return models.UnifiedMessageContentPart{}, false
		}

		var source anthropicImageSource
		if !shared.DecodeJSONObject(block.Source, &source) {
			return models.UnifiedMessageContentPart{}, false
		}

		var url string
		switch source.Type.Value {
		case "base64":
			mediaType := source.MediaType.Value
			data := source.Data.Value
			if mediaType == "" || data == "" {
				return models.UnifiedMessageContentPart{}, false
			}
			url = fmt.Sprintf("data:%s;base64,%s", mediaType, data)
		case "url":
			url = source.URL.Value
		}
		if url == "" {
			return models.UnifiedMessageContentPart{}, false
		}

		part := models.UnifiedMessageContentPart{
			Type: "image_url",
			ImageURL: &models.UnifiedImageURL{
				URL: url,
			},
		}
		part.CacheControl = parseCacheControl(block.CacheControl)
		return part, true

	default:
		return models.UnifiedMessageContentPart{}, false
	}
}
