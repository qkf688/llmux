package openai

import (
	"encoding/json"

	"github.com/qkf688/llmux/service/transform/shared"
)

type openAIChatCompletionRequest struct {
	Model               shared.Optional[string]        `json:"model"`
	Messages            shared.RawArray                `json:"messages"`
	Stream              shared.Optional[bool]          `json:"stream"`
	MaxTokens           shared.OptionalNumber[int]     `json:"max_tokens"`
	Temperature         shared.OptionalNumber[float64] `json:"temperature"`
	TopP                shared.OptionalNumber[float64] `json:"top_p"`
	Tools               shared.RawArray                `json:"tools"`
	ReasoningEffort     shared.Optional[string]        `json:"reasoning_effort"`
	FrequencyPenalty    shared.OptionalNumber[float64] `json:"frequency_penalty"`
	PresencePenalty     shared.OptionalNumber[float64] `json:"presence_penalty"`
	Seed                shared.OptionalNumber[int64]   `json:"seed"`
	LogitBias           shared.OptionalInt64Map        `json:"logit_bias"`
	Stop                json.RawMessage                `json:"stop"`
	User                shared.Optional[string]        `json:"user"`
	Metadata            shared.OptionalStringMap       `json:"metadata"`
	Logprobs            shared.Optional[bool]          `json:"logprobs"`
	TopLogprobs         shared.OptionalNumber[int64]   `json:"top_logprobs"`
	MaxCompletionTokens shared.OptionalNumber[int64]   `json:"max_completion_tokens"`
	Store               shared.Optional[bool]          `json:"store"`
	ResponseFormat      json.RawMessage                `json:"response_format"`
	ToolChoice          json.RawMessage                `json:"tool_choice"`
	ParallelToolCalls   shared.Optional[bool]          `json:"parallel_tool_calls"`
	StreamOptions       json.RawMessage                `json:"stream_options"`
	Modalities          shared.OptionalStringSeq       `json:"modalities"`
	Audio               json.RawMessage                `json:"audio"`
	ServiceTier         shared.Optional[string]        `json:"service_tier"`
	SafetyIdentifier    shared.Optional[string]        `json:"safety_identifier"`
	PromptCacheKey      shared.Optional[string]        `json:"prompt_cache_key"`
}

type openAIChatMessage struct {
	Role       shared.Optional[string] `json:"role"`
	Content    json.RawMessage         `json:"content"`
	ToolCalls  shared.RawArray         `json:"tool_calls"`
	ToolCallID shared.Optional[string] `json:"tool_call_id"`
}

type openAIChatContentPart struct {
	Type       shared.Optional[string] `json:"type"`
	Text       shared.Optional[string] `json:"text"`
	ImageURL   json.RawMessage         `json:"image_url"`
	InputAudio json.RawMessage         `json:"input_audio"`
}

type openAIChatImageURL struct {
	URL    shared.Optional[string] `json:"url"`
	Detail shared.Optional[string] `json:"detail"`
}

type openAIChatInputAudio struct {
	Data   shared.Optional[string] `json:"data"`
	Format shared.Optional[string] `json:"format"`
}

type openAIChatTool struct {
	Function json.RawMessage `json:"function"`
}

type openAIChatFunction struct {
	Name        shared.Optional[string] `json:"name"`
	Description shared.Optional[string] `json:"description"`
	Parameters  json.RawMessage         `json:"parameters"`
}

type openAIChatToolCall struct {
	ID       shared.Optional[string] `json:"id"`
	Type     shared.Optional[string] `json:"type"`
	Function json.RawMessage         `json:"function"`
}

type openAIChatToolCallFunction struct {
	Name      shared.Optional[string] `json:"name"`
	Arguments json.RawMessage         `json:"arguments"`
}

type openAIChatResponseFormat struct {
	Type       shared.Optional[string] `json:"type"`
	JSONSchema json.RawMessage         `json:"json_schema"`
}

type openAIChatToolChoiceObject struct {
	Type     shared.Optional[string] `json:"type"`
	Function json.RawMessage         `json:"function"`
}

type openAIChatToolChoiceFunction struct {
	Name shared.Optional[string] `json:"name"`
}

type openAIChatStreamOptions struct {
	IncludeUsage shared.Optional[bool] `json:"include_usage"`
}

type openAIChatAudio struct {
	Voice  shared.Optional[string] `json:"voice"`
	Format shared.Optional[string] `json:"format"`
}
