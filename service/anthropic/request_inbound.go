package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/common/maputil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/shared"
)

// TransformToUnified 将 Anthropic 请求格式转换为统一格式。
//
// 顶层字段与 system / tools / thinking / output_config / tool_choice 走 anthropicRequest
// 及其嵌套 DTO（见 request_dto.go），messages 的 content 块仍走 map 解析。
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

	unified.Messages = parseMessages(shared.RawJSONValue(req.Messages))
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

func parseMessages(raw interface{}) []models.UnifiedMessage {
	items, ok := asSlice(raw)
	if !ok {
		return nil
	}

	messages := make([]models.UnifiedMessage, 0, len(items))
	for _, item := range items {
		msgMap, ok := asMap(item)
		if !ok {
			continue
		}

		content, toolResultMessages := parseMessageContentAndToolResults(msgMap["content"])
		msg := models.UnifiedMessage{
			Role:      maputil.String(msgMap, "role"),
			Content:   content,
			ToolCalls: parseToolCalls(msgMap["content"]),
		}
		msg.CacheControl = parseCacheControl(msgMap["cache_control"])

		reasoning, signature, redactedData := parseReasoning(msgMap["content"])
		if reasoning != "" {
			msg.ReasoningContent = &reasoning
			if signature != "" {
				msg.ReasoningSignature = &signature
			}
		}
		if redactedData != "" {
			msg.RedactedThinkingData = &redactedData
		}

		// tool_result blocks are mapped to OpenAI-style tool messages to enable cross-format conversion.
		if len(toolResultMessages) > 0 {
			messages = append(messages, toolResultMessages...)
		}

		// If a user message only contains tool_result blocks, skip the original message.
		if msg.Role == "user" && content == nil && len(toolResultMessages) > 0 {
			continue
		}

		messages = append(messages, msg)
	}
	return messages
}

func parseTools(raw json.RawMessage) []models.UnifiedTool {
	if len(raw) == 0 || shared.IsJSONNull(raw) {
		return nil
	}
	// 与 parseSystem 同理：用错误判定区分「不是数组」与「空数组」，前者返回 nil、
	// 后者返回空切片，保持 map 时代 asSlice 的两种结果。
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
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
		tool.CacheControl = parseRawCacheControl(dto.CacheControl)
		tools = append(tools, tool)
	}
	return tools
}

func parseToolCalls(rawContent interface{}) []models.UnifiedToolCall {
	content, ok := asSlice(rawContent)
	if !ok {
		return nil
	}

	toolCalls := make([]models.UnifiedToolCall, 0)
	for _, item := range content {
		itemMap, ok := asMap(item)
		if !ok || maputil.String(itemMap, "type") != "tool_use" {
			continue
		}

		argsStr := "{}"
		if inputMap, ok := asMap(itemMap["input"]); ok {
			if argsBytes, err := json.Marshal(inputMap); err == nil {
				argsStr = string(argsBytes)
			}
		}

		index := len(toolCalls)
		toolCalls = append(toolCalls, models.UnifiedToolCall{
			ID:    maputil.String(itemMap, "id"),
			Type:  "function",
			Index: index,
			Function: models.UnifiedToolCallFunction{
				Name:      maputil.String(itemMap, "name"),
				Arguments: argsStr,
			},
			CacheControl: parseCacheControl(itemMap["cache_control"]),
		})
	}

	return toolCalls
}

// parseReasoning 提取 assistant 轮的 thinking 与 redacted_thinking 块。
//
// thinking 的可读文本与 signature 使用既有 reasoning 字段；redacted_thinking.data
// 是不透明密文，必须放入独立字段，禁止与文本拼接或尝试解析。统一模型当前
// 只保留一个 redacted_thinking 块；若收到多个，保留第一个非空 data。
func parseReasoning(rawContent interface{}) (text string, signature string, redactedData string) {
	content, ok := asSlice(rawContent)
	if !ok {
		return "", "", ""
	}

	var b strings.Builder
	for _, item := range content {
		itemMap, ok := asMap(item)
		if !ok {
			continue
		}

		switch maputil.String(itemMap, "type") {
		case "thinking":
			b.WriteString(maputil.String(itemMap, "thinking"))
			if sig := maputil.String(itemMap, "signature"); sig != "" {
				signature = sig
			}
		case "redacted_thinking":
			if redactedData == "" {
				redactedData = maputil.String(itemMap, "data")
			}
		}
	}

	return b.String(), signature, redactedData
}

func parseMessageContentAndToolResults(raw interface{}) (content interface{}, toolResultMessages []models.UnifiedMessage) {
	if raw == nil {
		return nil, nil
	}

	if str, ok := raw.(string); ok {
		return str, nil
	}

	items, ok := asSlice(raw)
	if !ok {
		// Keep behavior: passthrough unknown payloads (but this may reduce conversion quality).
		return raw, nil
	}

	parts := make([]models.UnifiedMessageContentPart, 0, len(items))

	for _, item := range items {
		itemMap, ok := asMap(item)
		if !ok {
			continue
		}

		if maputil.String(itemMap, "type") == "tool_result" {
			toolUseID := maputil.String(itemMap, "tool_use_id")
			if toolUseID == "" {
				continue
			}

			toolMsg := models.UnifiedMessage{
				Role:         "tool",
				ToolCallID:   toolUseID,
				Content:      parseToolResultContent(itemMap["content"]),
				CacheControl: parseCacheControl(itemMap["cache_control"]),
			}
			if isErr, ok := itemMap["is_error"].(bool); ok {
				toolMsg.ToolCallIsError = &isErr
			}

			toolResultMessages = append(toolResultMessages, toolMsg)
			continue
		}

		if part, ok := parseTextOrImageBlock(itemMap); ok {
			parts = append(parts, part)
		}
	}

	return collapseContentParts(parts), toolResultMessages
}

// parseToolResultContent 解析 tool_result 的 content。
//
// 曾经这里只留 text 块，而 computer-use / 截图类工具的结果整条都是图片块，
// 于是 content 变成空串，部分上游据此判 400。现在图片块一并保留，
// 由各出站适配器决定是原样透传（Anthropic 原生支持块数组）还是降级为占位文本。
func parseToolResultContent(raw interface{}) interface{} {
	switch v := raw.(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]models.UnifiedMessageContentPart, 0, len(v))
		for _, item := range v {
			itemMap, ok := asMap(item)
			if !ok {
				continue
			}
			if part, ok := parseTextOrImageBlock(itemMap); ok {
				parts = append(parts, part)
			}
		}

		if content := collapseContentParts(parts); content != nil {
			return content
		}
		return ""
	default:
		return ""
	}
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
func parseTextOrImageBlock(itemMap map[string]interface{}) (models.UnifiedMessageContentPart, bool) {
	switch maputil.String(itemMap, "type") {
	case "text":
		text := maputil.String(itemMap, "text")
		if text == "" {
			// Some callers use "content" for text blocks.
			text = maputil.String(itemMap, "content")
		}
		if text == "" {
			return models.UnifiedMessageContentPart{}, false
		}

		part := models.UnifiedMessageContentPart{
			Type: "text",
			Text: &text,
		}
		part.CacheControl = parseCacheControl(itemMap["cache_control"])
		return part, true

	case "image":
		source, ok := asMap(itemMap["source"])
		if !ok {
			return models.UnifiedMessageContentPart{}, false
		}

		var url string
		switch maputil.String(source, "type") {
		case "base64":
			mediaType := maputil.String(source, "media_type")
			data := maputil.String(source, "data")
			if mediaType == "" || data == "" {
				return models.UnifiedMessageContentPart{}, false
			}
			url = fmt.Sprintf("data:%s;base64,%s", mediaType, data)
		case "url":
			url = maputil.String(source, "url")
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
		part.CacheControl = parseCacheControl(itemMap["cache_control"])
		return part, true

	default:
		return models.UnifiedMessageContentPart{}, false
	}
}
