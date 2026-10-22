package responses

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/models"
)

// RequestTransformOptions 请求转换选项。
type RequestTransformOptions struct {
	MapReasoningEffort func(effort string) string
}

// TransformRequest 将 Responses API 请求转换为统一格式。
func TransformRequest(rawBody []byte, options RequestTransformOptions) (*models.UnifiedRequest, error) {
	var req ResponsesRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return nil, fmt.Errorf("failed to parse responses request: %w", err)
	}

	unified := &models.UnifiedRequest{
		Model:  req.Model,
		System: instructionsText(req.Instructions),
	}
	if req.Input.Text != nil {
		wasArray := false
		unified.TransformOptions.ArrayInputs = &wasArray
	} else if req.Input.Items != nil {
		wasArray := true
		unified.TransformOptions.ArrayInputs = &wasArray
	}

	if req.Stream != nil {
		unified.Stream = *req.Stream
	}
	if req.MaxOutputTokens != nil {
		unified.MaxTokens = *req.MaxOutputTokens
	}
	if req.Temperature != nil {
		unified.Temperature = req.Temperature
	}
	if req.TopP != nil {
		unified.TopP = req.TopP
	}

	var extractedSystem string
	unified.Messages, extractedSystem = convertInputToMessages(req.Input)
	if extractedSystem != "" {
		if unified.System != "" {
			unified.System = unified.System + "\n\n" + extractedSystem
		} else {
			unified.System = extractedSystem
		}
	}
	if len(req.Tools) > 0 {
		unified.Tools = convertToolsToUnified(req.Tools)
	}

	if req.ToolChoice != nil {
		unified.ToolChoice = &models.UnifiedToolChoice{}
		if req.ToolChoice.StringValue != nil {
			unified.ToolChoice.StringValue = req.ToolChoice.StringValue
		} else if obj := req.ToolChoice.ObjectValue; obj != nil {
			// 具名 tool_choice 的两种形态归一到统一模型的嵌套语义：
			// 官方扁平原 {"type":"function","name":"f"} 与 Chat 嵌套形
			// {"type":"function","function":{"name":"f"}}。
			// 两种都没拿到名字时整个 ToolChoice 置 nil——空壳会让出站 emit 出
			// `"tool_choice":{}` 这种上游会拒的形状（与 anthropic 入站同款判据）。
			name := obj.Name
			if obj.Function != nil && obj.Function.Name != "" {
				name = obj.Function.Name
			}
			if name != "" {
				unified.ToolChoice.ObjectValue = &models.UnifiedToolChoiceObject{
					Type: obj.Type,
					Function: &models.UnifiedToolChoiceFunction{
						Name: name,
					},
				}
			} else {
				unified.ToolChoice = nil
			}
		}
	}

	if req.Text != nil && req.Text.Format != nil && req.Text.Format.Type != "" {
		unified.ResponseFormat = &models.UnifiedResponseFormat{
			Type:       req.Text.Format.Type,
			JSONSchema: normalizeResponsesJSONSchema(req.Text.Format),
		}
	}

	if req.Metadata != nil {
		metadata := make(map[string]string, 0)
		for k, v := range req.Metadata {
			if s, ok := v.(string); ok && s != "" {
				metadata[k] = s
			}
		}
		if len(metadata) > 0 {
			unified.Metadata = metadata
		}
	}

	// effort 只认官方字段 reasoning.effort；metadata 是客户端自由 KV 标签，不是控制通道。
	effort := ""
	if req.Reasoning != nil && req.Reasoning.Effort != nil {
		effort = *req.Reasoning.Effort
	}
	if effort != "" {
		if options.MapReasoningEffort != nil {
			mapped := options.MapReasoningEffort(effort)
			unified.ReasoningEffort = &mapped
		} else {
			unified.ReasoningEffort = &effort
		}
	}
	if req.Reasoning != nil && req.Reasoning.MaxTokens != nil {
		unified.ReasoningBudget = req.Reasoning.MaxTokens
	}

	// 缓存与安全相关：与 TransformFromUnified 对称
	unified.ServiceTier = req.ServiceTier
	unified.SafetyIdentifier = req.SafetyIdentifier
	unified.PromptCacheKey = req.PromptCacheKey

	return unified, nil
}

func convertInputToMessages(input ResponsesInput) ([]models.UnifiedMessage, string) {
	var messages []models.UnifiedMessage
	var systemParts []string

	if input.Text != nil {
		messages = append(messages, models.UnifiedMessage{
			Role:    "user",
			Content: *input.Text,
		})
		return messages, ""
	}

	nonSystemCount := 0
	for _, item := range input.Items {
		if item.Role == "" {
			continue
		}
		if item.Role != "system" && item.Role != "developer" {
			nonSystemCount++
		}
	}
	shouldExtractSystem := nonSystemCount > 0

	// pendingCallGroup 标记「上一条产出的是 function_call 组头」——用于判定下一个
	// function_call 是否与它同轮（连续）从而并入同一条 assistant 消息。
	// 任何非 function_call 的 item 都会把它置回 false（见循环内各分支）。
	pendingCallGroup := false

	for _, item := range input.Items {
		switch item.Type {
		case "message", "input_text", "":
			pendingCallGroup = false
			if item.Role != "" {
				if (item.Role == "system" || item.Role == "developer") && shouldExtractSystem {
					systemContent := ""
					if item.Content != nil {
						if s, ok := item.Content.(string); ok {
							systemContent = s
						} else if parts, ok := item.Content.([]interface{}); ok {
							systemContent = extractTextFromParts(parts)
						}
					} else if item.Text != nil {
						systemContent = *item.Text
					}
					if systemContent != "" {
						systemParts = append(systemParts, systemContent)
					}
					continue
				}

				msg := models.UnifiedMessage{Role: item.Role}
				if item.Content != nil {
					switch v := item.Content.(type) {
					case string:
						msg.Content = v
					case []interface{}:
						if content, ok := parsePartsToUnifiedContent(v); ok {
							msg.Content = content
						}
					default:
						msg.Content = item.Content
					}
				} else if item.Text != nil {
					msg.Content = *item.Text
				}

				if item.Role == "tool" && item.CallID != nil {
					msg.ToolCallID = *item.CallID
				}

				messages = append(messages, msg)
			} else if item.Text != nil {
				messages = append(messages, models.UnifiedMessage{
					Role:    "user",
					Content: *item.Text,
				})
			}

		case "output_text":
			pendingCallGroup = false
			if item.Text != nil {
				messages = append(messages, models.UnifiedMessage{
					Role:    "assistant",
					Content: *item.Text,
				})
			}

		case "function_call":
			// arguments 在 Responses 协议里可省（无参工具的常见写法），
			// 缺省时兜底为空参数对象 "{}" 而不是丢弃整条 item——
			// 丢一条 function_call 就破坏「调用↔结果」配对，结果消息前面
			// 没有对应调用，上游直接判 400 且报错离真因隔了两层。
			if item.Name != nil {
				arguments := "{}"
				if item.Arguments != nil {
					arguments = *item.Arguments
				}
				toolCall := models.UnifiedToolCall{
					Type: "function",
					Function: models.UnifiedToolCallFunction{
						Name:      *item.Name,
						Arguments: arguments,
					},
				}
				if item.CallID != nil {
					toolCall.ID = *item.CallID
				} else if item.ID != "" {
					toolCall.ID = item.ID
				}
				// 同一轮的并行调用在 Responses 里是 input 数组中**连续**的多个
				// function_call item，必须落进同一条 assistant 消息的 ToolCalls 数组：
				// OpenAI Chat 协议要求「一批 tool_calls 的结果紧跟在该 assistant 之后」，
				// 拆成多条 assistant 消息会让结果前面隔着别的 assistant 消息，上游判
				// 工具配对断裂（11148 tool calls and tool results do not match）并顶死
				// 整条会话。反向转换（transform_to_responses.go）本就是「一条 assistant 的
				// 多个 tool_calls → 多个 function_call item」，此处合并与之对称。
				//
				// 合并边界＝连续性：只有紧邻上一个 function_call item 的调用才并入同一条
				// assistant；一旦出现别的 item（function_call_output / message 等）就封口，
				// 因为那意味着新一轮调用。
				if n := len(messages); n > 0 && messages[n-1].Role == "assistant" &&
					len(messages[n-1].ToolCalls) > 0 && messages[n-1].Content == nil &&
					pendingCallGroup {
					messages[n-1].ToolCalls = append(messages[n-1].ToolCalls, toolCall)
					continue
				}
				messages = append(messages, models.UnifiedMessage{
					Role:      "assistant",
					ToolCalls: []models.UnifiedToolCall{toolCall},
				})
				pendingCallGroup = true
			}

		case "function_call_output":
			pendingCallGroup = false
			// output 在 Responses 协议里可省，缺省时兜底为空串而不是丢弃整条
			// item——丢一条结果同样破坏「调用↔结果」配对。
			{
				msg := models.UnifiedMessage{
					Role: "tool",
				}
				switch v := item.Output.(type) {
				case nil:
					// 缺省 output：空串表示「调用成功但无内容」，不是未知
					msg.Content = ""
				case string:
					msg.Content = v
				case []interface{}:
					if content, ok := parsePartsToUnifiedContent(v); ok {
						msg.Content = content
					} else {
						// 数组形式但解析失败，降级为空串避免整条丢弃
						msg.Content = ""
					}
				default:
					// 未知类型降级为空串，避免泄漏内部结构
					msg.Content = ""
				}
				if item.CallID != nil {
					msg.ToolCallID = *item.CallID
				}
				messages = append(messages, msg)
			}
		}
	}

	return messages, strings.Join(systemParts, "\n\n")
}

func convertToolsToUnified(tools []ResponsesTool) []models.UnifiedTool {
	var unified []models.UnifiedTool
	for _, tool := range tools {
		if tool.Type == "function" && tool.Name != "" {
			unified = append(unified, models.UnifiedTool{
				Type: "function",
				Function: models.UnifiedFunc{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			})
		}
	}
	return unified
}

// instructionsText 取 instructions 的文本内容。
//
// 官方形状是纯字符串；数组形态（非官方写法）汇总其中各段的文本。
// 数组里有多段时用换行连接——各段通常是独立句子，粘连会改变语义。
//
// 入参是**指针**：未设置（nil）与「设为空串」在**序列化层**是两件事——指针 + omitempty
// 才能让「未设置」的请求根本不带这个键；取值层面两者都归到空串（统一模型的 System
// 不区分「没给」与「给了空串」）。
func instructionsText(instr *ResponsesInstructions) string {
	if instr == nil {
		return ""
	}
	if instr.Parts == nil {
		return instr.Text
	}
	parts := extractTextFromParts(instr.Parts)
	if parts == "" {
		return ""
	}
	// extractTextFromParts 把多段直接拼接（那是 content 数组的语义）；
	// instructions 的多段是独立句子，这里按行分段再连接。
	lines := make([]string, 0, len(instr.Parts))
	for _, raw := range instr.Parts {
		partMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		partType, _ := partMap["type"].(string)
		if partType != "input_text" && partType != "output_text" && partType != "text" {
			continue
		}
		if text, ok := partMap["text"].(string); ok && text != "" {
			lines = append(lines, text)
		}
	}
	if len(lines) == 0 {
		return parts
	}
	return strings.Join(lines, "\n")
}

// normalizeResponsesJSONSchema 把 text.format 的两种 json_schema 形态归一到统一模型
// 承载的内嵌对象形状 {"name","schema","strict"}。
//
// 内嵌对象是 Chat Completions 的自然形状（Chat 出站直接把它塞进
// response_format.json_schema），也是统一模型 UnifiedResponseFormat.JSONSchema 的
// 既有承载方式——归一到它，Chat / Anthropic 两条出站路径都不用改。
//
// 官方扁平原的 name / schema / strict 与 type 平级；嵌套形的同名三键在
// json_schema 子对象里。两形给的信息等价，故**官方扁平优先**：客户端同时给出
// 两形时（迁移期客户端、或误把 Chat 形状抄进 Responses 请求）以官方形状为准，
// 嵌套形只在扁平三键全缺时兜底。
func normalizeResponsesJSONSchema(format *ResponsesTextFormat) json.RawMessage {
	if format == nil {
		return nil
	}

	// 官方扁平优先：任一扁平键存在即按扁平形组装。注意 Schema 是 RawMessage，
	// 判 nil 而不是判长度——`schema:false` 是合法值（长度非 0），不能漏判。
	if format.Name != "" || format.Schema != nil || format.Strict != nil {
		normalized := map[string]interface{}{}
		if format.Name != "" {
			normalized["name"] = format.Name
		}
		if format.Schema != nil {
			// 原样透传：布尔 / 数组 / 对象三种合法形态都不该在网关侧被判非法。
			normalized["schema"] = json.RawMessage(format.Schema)
		}
		if format.Strict != nil {
			normalized["strict"] = *format.Strict
		}
		raw, err := json.Marshal(normalized)
		if err != nil {
			return nil
		}
		return raw
	}

	// 嵌套形兜底：扁平三键全缺时才有意义。
	if len(format.JSONSchema) > 0 {
		return format.JSONSchema
	}

	return nil
}

func extractTextFromParts(parts []interface{}) string {
	var textParts []string
	for _, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		partType, _ := partMap["type"].(string)
		if partType != "input_text" && partType != "output_text" && partType != "text" {
			continue
		}
		if text, ok := partMap["text"].(string); ok {
			textParts = append(textParts, text)
		}
	}
	return strings.Join(textParts, "")
}

func parsePartsToUnifiedContent(parts []interface{}) (any, bool) {
	textOnly := true
	textParts := make([]string, 0, len(parts))
	unifiedParts := make([]models.UnifiedMessageContentPart, 0, len(parts))

	for _, raw := range parts {
		partMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		partType, _ := partMap["type"].(string)
		switch partType {
		case "input_text", "output_text", "text":
			if text, ok := partMap["text"].(string); ok {
				textParts = append(textParts, text)
				txt := text
				unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{
					Type: "text",
					Text: &txt,
				})
			}

		case "input_image", "image_url":
			textOnly = false

			url := ""
			detail := (*string)(nil)

			if v, ok := partMap["image_url"]; ok && v != nil {
				switch vv := v.(type) {
				case string:
					url = vv
				case map[string]interface{}:
					if s, ok := vv["url"].(string); ok {
						url = s
					}
					if s, ok := vv["detail"].(string); ok {
						detail = &s
					}
				}
			}
			if url == "" {
				if s, ok := partMap["url"].(string); ok {
					url = s
				}
			}
			if detail == nil {
				if s, ok := partMap["detail"].(string); ok {
					detail = &s
				}
			}

			if url != "" {
				unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{
					Type: "image_url",
					ImageURL: &models.UnifiedImageURL{
						URL:    url,
						Detail: detail,
					},
				})
			}

		default:
			// Unknown part types should keep array form to avoid dropping non-text inputs.
			if partType != "" {
				textOnly = false
				unifiedParts = append(unifiedParts, models.UnifiedMessageContentPart{Type: partType})
			}
		}
	}

	if len(unifiedParts) == 0 {
		return nil, false
	}

	if textOnly {
		return strings.Join(textParts, ""), true
	}

	return unifiedParts, true
}
