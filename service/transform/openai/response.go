package openai

import (
	"bytes"
	"encoding/json"

	"github.com/qkf688/llmux/models"
)

// ParseResponse parses an OpenAI Chat Completion response body into the
// unified response representation.
func ParseResponse(body []byte) (*models.UnifiedResponse, error) {
	var resp openAIChatCompletionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	unified := &models.UnifiedResponse{
		ID:                resp.ID,
		Object:            resp.Object,
		Created:           resp.Created,
		Model:             resp.Model,
		SystemFingerprint: resp.SystemFingerprint,
		ServiceTier:       resp.ServiceTier,
	}

	if resp.Error != nil {
		unified.Error = &models.ResponseError{
			Detail: models.ErrorDetail{
				Code:      resp.Error.Code,
				Message:   resp.Error.Message,
				Type:      resp.Error.Type,
				Param:     resp.Error.Param,
				RequestID: resp.Error.RequestID,
			},
		}
	}

	if len(resp.Choices) > 0 {
		unified.Choices = make([]models.UnifiedChoice, 0, len(resp.Choices))
		for _, choice := range resp.Choices {
			content := parseOpenAIMessageContent(choice.Message.Content)
			unified.Choices = append(unified.Choices, models.UnifiedChoice{
				Index: choice.Index,
				Message: &models.UnifiedMessage{
					Role:      choice.Message.Role,
					Content:   content,
					ToolCalls: parseOpenAIResponseToolCalls(choice.Message.ToolCalls),
				},
				FinishReason: choice.FinishReason,
			})
		}
	}

	if resp.Usage != nil {
		unified.Usage = &models.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		if resp.Usage.PromptTokensDetails != nil {
			unified.Usage.PromptTokensDetails.CachedTokens = resp.Usage.PromptTokensDetails.CachedTokens
			unified.Usage.PromptTokensDetails.AudioTokens = resp.Usage.PromptTokensDetails.AudioTokens
		}
		if resp.Usage.CompletionTokensDetails != nil {
			unified.Usage.CompletionTokensDetails.ReasoningTokens = resp.Usage.CompletionTokensDetails.ReasoningTokens
			unified.Usage.CompletionTokensDetails.AudioTokens = resp.Usage.CompletionTokensDetails.AudioTokens
			// details 非 nil = 上游明确报告过拆分（含 0），known 标记供落库/展示区分「没报」与「真 0」。
			unified.Usage.CompletionTokensDetails.ReasoningTokensKnown = true
		}
	}

	return unified, nil
}

// FormatResponse formats a unified response as an OpenAI Chat Completion
// response body.
func FormatResponse(unified *models.UnifiedResponse) ([]byte, error) {
	if unified.Error != nil && len(unified.Choices) == 0 {
		return json.Marshal(struct {
			Error *openAIResponseErrorEnvelope `json:"error"`
		}{
			Error: &openAIResponseErrorEnvelope{
				Message:   unified.Error.Detail.Message,
				Type:      unified.Error.Detail.Type,
				Code:      unified.Error.Detail.Code,
				Param:     unified.Error.Detail.Param,
				RequestID: unified.Error.Detail.RequestID,
			},
		})
	}

	resp := openAIChatCompletionResponseOut{
		ID:                unified.ID,
		Object:            unified.Object,
		Created:           unified.Created,
		Model:             unified.Model,
		Choices:           []openAIChatCompletionChoiceOut{},
		SystemFingerprint: unified.SystemFingerprint,
		ServiceTier:       unified.ServiceTier,
	}

	if len(unified.Choices) > 0 {
		resp.Choices = make([]openAIChatCompletionChoiceOut, 0, len(unified.Choices))
		for _, choice := range unified.Choices {
			if choice.Message == nil {
				continue
			}
			msg := openAIChatCompletionMessageOut{
				Role:    choice.Message.Role,
				Content: choice.Message.Content,
			}
			if len(choice.Message.ToolCalls) > 0 {
				msg.ToolCalls = make([]openAIToolCallOut, 0, len(choice.Message.ToolCalls))
				for _, tc := range choice.Message.ToolCalls {
					msg.ToolCalls = append(msg.ToolCalls, openAIToolCallOut{
						ID:   tc.ID,
						Type: tc.Type,
						Function: openAIToolCallFunctionOut{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				}
			}

			resp.Choices = append(resp.Choices, openAIChatCompletionChoiceOut{
				Index: choice.Index,
				Message: openAIChatCompletionMessageOut{
					Role:      msg.Role,
					Content:   msg.Content,
					ToolCalls: msg.ToolCalls,
				},
				FinishReason: choice.FinishReason,
			})
		}
	}

	if unified.Usage != nil {
		resp.Usage = &openAIUsageOut{
			PromptTokens:     unified.Usage.PromptTokens,
			CompletionTokens: unified.Usage.CompletionTokens,
			TotalTokens:      unified.Usage.TotalTokens,
		}
		if unified.Usage.PromptTokensDetails.CachedTokens > 0 || unified.Usage.PromptTokensDetails.AudioTokens > 0 {
			resp.Usage.PromptTokensDetails = &openAITokenDetailsOut{
				CachedTokens: unified.Usage.PromptTokensDetails.CachedTokens,
				AudioTokens:  unified.Usage.PromptTokensDetails.AudioTokens,
			}
		}
		if unified.Usage.CompletionTokensDetails.ReasoningTokens > 0 || unified.Usage.CompletionTokensDetails.AudioTokens > 0 {
			resp.Usage.CompletionTokensDetails = &openAICompletionTokenDetailsOut{
				ReasoningTokens: unified.Usage.CompletionTokensDetails.ReasoningTokens,
				AudioTokens:     unified.Usage.CompletionTokensDetails.AudioTokens,
			}
		}
	}

	return json.Marshal(resp)
}

func parseOpenAIResponseToolCalls(tcs []openAIToolCall) []models.UnifiedToolCall {
	if len(tcs) == 0 {
		return nil
	}

	toolCalls := make([]models.UnifiedToolCall, 0, len(tcs))
	for _, tc := range tcs {
		toolCalls = append(toolCalls, models.UnifiedToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: models.UnifiedToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: normalizeOpenAIToolCallArguments(tc.Function.Arguments),
			},
		})
	}
	return toolCalls
}

func normalizeOpenAIToolCallArguments(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "{}"
	}

	var argsStr string
	if err := json.Unmarshal(trimmed, &argsStr); err == nil {
		if argsStr == "" {
			return "{}"
		}
		return argsStr
	}

	return string(trimmed)
}

func parseOpenAIMessageContent(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}

	var str string
	if err := json.Unmarshal(trimmed, &str); err == nil {
		return str
	}

	var parts []models.UnifiedMessageContentPart
	if err := json.Unmarshal(trimmed, &parts); err == nil {
		return parts
	}

	// Best-effort: preserve unknown shapes as raw JSON (without converting into maps).
	return rawJSON(trimmed)
}

type rawJSON json.RawMessage

func (r rawJSON) MarshalJSON() ([]byte, error) {
	return []byte(r), nil
}

type openAIChatCompletionResponse struct {
	ID                string                       `json:"id"`
	Object            string                       `json:"object"`
	Created           int64                        `json:"created"`
	Model             string                       `json:"model"`
	Choices           []openAIChatCompletionChoice `json:"choices"`
	Usage             *openAIUsage                 `json:"usage,omitempty"`
	Error             *openAIResponseErrorEnvelope `json:"error,omitempty"`
	SystemFingerprint string                       `json:"system_fingerprint,omitempty"`
	ServiceTier       string                       `json:"service_tier,omitempty"`
}

type openAIChatCompletionChoice struct {
	Index        int                         `json:"index"`
	Message      openAIChatCompletionMessage `json:"message"`
	FinishReason string                      `json:"finish_reason"`
}

type openAIChatCompletionMessage struct {
	Role      string           `json:"role"`
	Content   json.RawMessage  `json:"content,omitempty"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIToolCall struct {
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function openAIToolCallFunction `json:"function,omitempty"`
}

type openAIToolCallFunction struct {
	Name      string          `json:"name,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type openAIUsage struct {
	PromptTokens            int64                         `json:"prompt_tokens"`
	CompletionTokens        int64                         `json:"completion_tokens"`
	TotalTokens             int64                         `json:"total_tokens"`
	PromptTokensDetails     *openAITokenDetails           `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *openAICompletionTokenDetails `json:"completion_tokens_details,omitempty"`
}

type openAITokenDetails struct {
	CachedTokens int64 `json:"cached_tokens,omitempty"`
	AudioTokens  int64 `json:"audio_tokens,omitempty"`
}

type openAICompletionTokenDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
	AudioTokens     int64 `json:"audio_tokens,omitempty"`
}

type openAIResponseErrorEnvelope struct {
	Message   string `json:"message"`
	Type      string `json:"type,omitempty"`
	Code      string `json:"code,omitempty"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type openAIChatCompletionResponseOut struct {
	ID                string                          `json:"id"`
	Object            string                          `json:"object"`
	Created           int64                           `json:"created"`
	Model             string                          `json:"model"`
	Choices           []openAIChatCompletionChoiceOut `json:"choices"`
	Usage             *openAIUsageOut                 `json:"usage,omitempty"`
	Error             *openAIResponseErrorEnvelope    `json:"error,omitempty"`
	SystemFingerprint string                          `json:"system_fingerprint,omitempty"`
	ServiceTier       string                          `json:"service_tier,omitempty"`
}

type openAIChatCompletionChoiceOut struct {
	Index        int                            `json:"index"`
	Message      openAIChatCompletionMessageOut `json:"message"`
	FinishReason string                         `json:"finish_reason,omitempty"`
}

type openAIChatCompletionMessageOut struct {
	Role      string              `json:"role"`
	Content   any                 `json:"content,omitempty"`
	ToolCalls []openAIToolCallOut `json:"tool_calls,omitempty"`
}

type openAIToolCallOut struct {
	ID       string                    `json:"id,omitempty"`
	Type     string                    `json:"type,omitempty"`
	Function openAIToolCallFunctionOut `json:"function,omitempty"`
}

type openAIToolCallFunctionOut struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openAIUsageOut struct {
	PromptTokens            int64                            `json:"prompt_tokens"`
	CompletionTokens        int64                            `json:"completion_tokens"`
	TotalTokens             int64                            `json:"total_tokens"`
	PromptTokensDetails     *openAITokenDetailsOut           `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *openAICompletionTokenDetailsOut `json:"completion_tokens_details,omitempty"`
}

type openAITokenDetailsOut struct {
	CachedTokens int64 `json:"cached_tokens,omitempty"`
	AudioTokens  int64 `json:"audio_tokens,omitempty"`
}

type openAICompletionTokenDetailsOut struct {
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
	AudioTokens     int64 `json:"audio_tokens,omitempty"`
}
