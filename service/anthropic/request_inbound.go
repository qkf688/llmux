package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/shared"
	"strings"

	"github.com/qkf688/llmux/common/maputil"
)

// TransformToUnified 将 Anthropic 请求格式转换为统一格式。
func TransformToUnified(ctx context.Context, rawBody []byte) (*models.UnifiedRequest, error) {
	var req map[string]interface{}
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, err
	}

	unified := &models.UnifiedRequest{
		Model:  maputil.String(req, "model"),
		Stream: maputil.Bool(req, "stream"),
	}
	unified.System, unified.SystemParts = parseSystem(req["system"])

	if maxTokens, ok := req["max_tokens"].(float64); ok {
		unified.MaxTokens = int(maxTokens)
	}
	if temp, ok := req["temperature"].(float64); ok {
		unified.Temperature = &temp
	}
	if topP, ok := req["top_p"].(float64); ok {
		unified.TopP = &topP
	}

	unified.Messages = parseMessages(req["messages"])
	unified.Tools = parseTools(req["tools"])

	if stopSeqs := maputil.StringSlice(req, "stop_sequences"); len(stopSeqs) > 0 {
		unified.Stop = &models.UnifiedStop{Multiple: stopSeqs}
	}

	if metadata := maputil.StringMap(req, "metadata"); len(metadata) > 0 {
		unified.Metadata = metadata
	}

	if thinking, ok := asMap(req["thinking"]); ok {
		thinkingType := maputil.String(thinking, "type")
		budgetTokens := maputil.Int64(thinking, "budget_tokens")
		if thinkingType == "enabled" && budgetTokens > 0 {
			effort := ThinkingBudgetToReasoningEffort(budgetTokens)
			if effort != "" {
				unified.ReasoningEffort = &effort
			}
			unified.ReasoningBudget = &budgetTokens
		}
	}

	// output_config.effort 是 Claude 4.6 adaptive thinking 的显式档位字段，
	// 优先级高于 thinking.budget_tokens 反推（显式意图优先）。
	// 归一化走 NormalizeReasoningEffort，与 OpenAI 入站行为一致；
	// budget 仍取 thinking.budget_tokens（两字段并存，出站 budget 优先已有实现不变）。
	if outputConfig, ok := asMap(req["output_config"]); ok {
		if effortStr := maputil.String(outputConfig, "effort"); effortStr != "" {
			normalized := shared.NormalizeReasoningEffort(ctx, effortStr)
			unified.ReasoningEffort = &normalized
		}
	}

	// tool_choice (best-effort): keep unified semantics as OpenAI-style tool_choice.
	if rawToolChoice, exists := req["tool_choice"]; exists && rawToolChoice != nil {
		unified.ToolChoice = &models.UnifiedToolChoice{}

		if v, ok := rawToolChoice.(string); ok && v != "" {
			unified.ToolChoice.StringValue = &v
		} else if tcMap, ok := asMap(rawToolChoice); ok {
			tcType := maputil.String(tcMap, "type")
			switch tcType {
			case "tool":
				name := maputil.String(tcMap, "name")
				if name != "" {
					unified.ToolChoice.ObjectValue = &models.UnifiedToolChoiceObject{
						Type: "function",
						Function: &models.UnifiedToolChoiceFunction{
							Name: name,
						},
					}
				}
			default:
				if tcType != "" {
					unified.ToolChoice.StringValue = &tcType
				}
			}
		}

		if unified.ToolChoice.StringValue == nil && unified.ToolChoice.ObjectValue == nil {
			unified.ToolChoice = nil
		}
	}

	return unified, nil
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

func parseTools(raw interface{}) []models.UnifiedTool {
	items, ok := asSlice(raw)
	if !ok {
		return nil
	}

	tools := make([]models.UnifiedTool, 0, len(items))
	for _, item := range items {
		toolMap, ok := asMap(item)
		if !ok {
			continue
		}

		tool := models.UnifiedTool{
			Type: "function",
			Function: models.UnifiedFunc{
				Name:        maputil.String(toolMap, "name"),
				Description: maputil.String(toolMap, "description"),
				Parameters:  toolMap["input_schema"],
			},
		}
		tool.CacheControl = parseCacheControl(toolMap["cache_control"])
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
