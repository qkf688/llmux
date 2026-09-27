package anthropic

import (
	"encoding/json"
	"fmt"

	"github.com/qkf688/llmux/common/maputil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/shared"
)

// anthropicResponseDecode ParseResponse 的入站 DTO（Stage 27，与 openai/responses 同口径）：
// 用类型化解码替代「全 map 宽容解析」，字段类型不符即报错——此前 `{"usage":"boom"}` 等
// 形状非法的响应会拆出「全零/空」的零值成功，坏供应商被伪装成可用（200 + 垃圾形状被判
// 成功、加分、继续接流量）。形状必需 = id/model（缺任何一个都拼不出可提交的统一响应）；
// type 恒为 "message" 但部分兼容网关会省略，刻意不进必需集（收敛只挡真垃圾，不挡方言）。
// 残余边界（`{"code":401,…}` 类「合法 JSON 对象但语义是错误页」）见 framework-conventions
// §6，不在本判据扩权。
type anthropicResponseDecode struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	// StopReason 宽容：*string 区分「缺失」与空串，null/缺失都是合法（stop_reason 非必需）；
	// 类型不符（数字等）由解码直接报错。
	StopReason *string               `json:"stop_reason"`
	Usage      *anthropicUsageDecode `json:"usage"`
}

// anthropicUsageDecode usage 的形状守卫：usage 若出现，至少要有一个数字型 token 键
// （input_tokens / output_tokens / prompt_tokens / completion_tokens / total_tokens），
// 全缺失（如 `{"usage":{}}`）或类型不符（如字符串 "boom"）= 形状非法——数字键的
// 解码错误由 *int64 承担，`{"usage":"boom"}` 顶层类型错误也由 *anthropicUsageDecode
// 解码直接报错。raw 保留原始字节供 UsageFromMap 的多方言候选键表宽容吸收
// （cache_read_input_tokens 等键不进 typed 守卫）。
type anthropicUsageDecode struct {
	raw json.RawMessage `json:"-"`
	// 全部用指针：nil = 上游没给该键（kimi 等混合方言只给兼容键），与「给了 0」区分。
	InputTokens      *int64 `json:"input_tokens"`
	OutputTokens     *int64 `json:"output_tokens"`
	PromptTokens     *int64 `json:"prompt_tokens"`
	CompletionTokens *int64 `json:"completion_tokens"`
	TotalTokens      *int64 `json:"total_tokens"`
}

// UnmarshalJSON 先解原始字节再解形状必需键（标准两段式，保住 UsageFromMap 需要的全键）。
func (u *anthropicUsageDecode) UnmarshalJSON(data []byte) error {
	u.raw = append(u.raw[:0], data...)
	type alias anthropicUsageDecode
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*u = anthropicUsageDecode(a)
	u.raw = append(u.raw[:0], data...)
	return nil
}

// hasAnyTokenKey usage 对象里是否至少有一个数字型 token 键。
func (u *anthropicUsageDecode) hasAnyTokenKey() bool {
	return u.InputTokens != nil || u.OutputTokens != nil ||
		u.PromptTokens != nil || u.CompletionTokens != nil || u.TotalTokens != nil
}

// ParseResponse 将 Anthropic 响应格式转换为统一格式。
func ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	var resp anthropicResponseDecode
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 anthropic 响应失败: %w", err)
	}
	if resp.ID == "" || resp.Model == "" {
		return nil, fmt.Errorf("anthropic 响应形状非法: id/model 缺失 (type=%q id=%q model=%q)", resp.Type, resp.ID, resp.Model)
	}
	if resp.Usage != nil && !resp.Usage.hasAnyTokenKey() {
		return nil, fmt.Errorf("anthropic 响应形状非法: usage 缺少数字型 token 字段")
	}

	unified := &models.UnifiedResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: 0,
		Model:   resp.Model,
	}

	finishReason := ""
	if resp.StopReason != nil {
		finishReason = *resp.StopReason
	}
	switch finishReason {
	case "end_turn", "stop_sequence":
		finishReason = "stop"
	case "tool_use":
		finishReason = "tool_calls"
	case "max_tokens":
		finishReason = "length"
	}

	// content 形状守卫：缺失/null 合法（空回复），非数组且非字符串的标量（数字、布尔）
	// = 形状非法——parseContentBlocks 会把它兜底透传成「原始 JSON 内容」，垃圾值变成
	// 可提交响应。守卫放在这里而不是 parseContentBlocks：那是请求/响应共用的单次遍历，
	// 请求侧语义（system 标量透传）不同，不在此扩权。
	if len(resp.Content) > 0 && !shared.IsJSONNull(resp.Content) {
		if _, isStr := shared.RawString(resp.Content); !isStr {
			if _, isArray := decodeRawArray(resp.Content); !isArray {
				return nil, fmt.Errorf("anthropic 响应形状非法: content 既非字符串也非块数组")
			}
		}
	}

	// 与请求入站共用 parseContentBlocks 的单次遍历（#28）。响应侧只取 content 与
	// tool_calls：thinking 仍走下面的 extractThinking，因为两者语义**不同**——
	// extractThinking 的 thinking/signature 是「后一个块整体覆盖前一个（含空值覆盖）」，
	// 而 parseContentBlocks 是「文本顺序拼接 + signature 仅非空时覆盖」。
	// 统一两者会改变响应回写行为，不在本次范围内。
	parsed := parseContentBlocks(resp.Content)
	content := parsed.content
	toolCalls := parsed.toolCalls

	reasoningText, reasoningSig, redactedData := extractThinking(resp.Content)
	if reasoningText != "" {
		switch v := content.(type) {
		case string:
			if v != "" {
				content = reasoningText + "\n\n---\n\n" + v
			} else {
				content = reasoningText
			}
		case []models.UnifiedMessageContentPart:
			prefix := reasoningText
			if len(v) > 0 {
				prefix += "\n\n---\n\n"
			}
			v2 := make([]models.UnifiedMessageContentPart, 0, len(v)+1)
			v2 = append(v2, models.UnifiedMessageContentPart{Type: "text", Text: &prefix})
			v2 = append(v2, v...)
			content = v2
		case nil:
			content = reasoningText
		case json.RawMessage:
			// content 是兜底透传的原始 JSON（既不是 string 也不是块数组）。按 JSON
			// 文本拼接——落到下面的 %v 会把 []byte 打成字节数组，那是纯噪声。
			if len(v) > 0 {
				content = reasoningText + "\n\n---\n\n" + string(v)
			} else {
				content = reasoningText
			}
		default:
			// Keep behavior: best-effort stringify unknown content shapes.
			content = fmt.Sprintf("%v", v)
		}
	}

	message := &models.UnifiedMessage{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	}
	if reasoningText != "" {
		message.ReasoningContent = &reasoningText
	}
	if reasoningSig != "" {
		message.ReasoningSignature = &reasoningSig
	}
	if redactedData != "" {
		message.RedactedThinkingData = &redactedData
	}

	unified.Choices = []models.UnifiedChoice{{
		Index:        0,
		Message:      message,
		FinishReason: finishReason,
	}}

	if resp.Usage != nil {
		// 归一走 models.UsageFromMap：anthropic 原生 input/output/cache_read 字段、
		// kimi 一类混合返回的 openai 兼容字段、上游给定的 total 口径，全部由候选路径表
		// 统一吸收，与流式侧信道 / chat processer 落库同源，不再本地手写一套解析。
		// 走 map 是刻意保留：UsageFromMap 的候选键表覆盖多方言，typed DTO 只守形状
		// （input/output 数字必需），其余键仍由 map 宽容吸收。
		var usageMap map[string]interface{}
		if err := json.Unmarshal(resp.Usage.raw, &usageMap); err == nil && usageMap != nil {
			u := models.UsageFromMap(usageMap)
			unified.Usage = &u
		}
	}

	return unified, nil
}

func extractThinking(raw json.RawMessage) (thinking string, signature string, redactedData string) {
	if len(raw) == 0 {
		return "", "", ""
	}
	var items []interface{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return "", "", ""
	}

	for _, item := range items {
		itemMap, ok := asMap(item)
		if !ok {
			continue
		}
		switch maputil.String(itemMap, "type") {
		case "thinking":
			thinking = maputil.String(itemMap, "thinking")
			signature = maputil.String(itemMap, "signature")
		case "redacted_thinking":
			if redactedData == "" {
				redactedData = maputil.String(itemMap, "data")
			}
		}
	}

	return thinking, signature, redactedData
}
