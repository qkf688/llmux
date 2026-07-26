package openai

import (
	"encoding/json"
)

type openAIChatCompletionRequest struct {
	Model               openAIString    `json:"model"`
	Messages            openAIRawArray  `json:"messages"`
	Stream              openAIBool      `json:"stream"`
	MaxTokens           openAIInt       `json:"max_tokens"`
	Temperature         openAIFloat     `json:"temperature"`
	TopP                openAIFloat     `json:"top_p"`
	Tools               openAIRawArray  `json:"tools"`
	ReasoningEffort     openAIString    `json:"reasoning_effort"`
	FrequencyPenalty    openAIFloat     `json:"frequency_penalty"`
	PresencePenalty     openAIFloat     `json:"presence_penalty"`
	Seed                openAIInt64     `json:"seed"`
	LogitBias           openAIInt64Map  `json:"logit_bias"`
	Stop                json.RawMessage `json:"stop"`
	User                openAIString    `json:"user"`
	Metadata            openAIStringMap `json:"metadata"`
	Logprobs            openAIBool      `json:"logprobs"`
	TopLogprobs         openAIInt64     `json:"top_logprobs"`
	MaxCompletionTokens openAIInt64     `json:"max_completion_tokens"`
	Store               openAIBool      `json:"store"`
	ResponseFormat      json.RawMessage `json:"response_format"`
	ToolChoice          json.RawMessage `json:"tool_choice"`
	ParallelToolCalls   openAIBool      `json:"parallel_tool_calls"`
	StreamOptions       json.RawMessage `json:"stream_options"`
	Modalities          openAIStringSeq `json:"modalities"`
	Audio               json.RawMessage `json:"audio"`
}

type openAIChatMessage struct {
	Role       openAIString    `json:"role"`
	Content    json.RawMessage `json:"content"`
	ToolCalls  openAIRawArray  `json:"tool_calls"`
	ToolCallID openAIString    `json:"tool_call_id"`
}

type openAIChatContentPart struct {
	Type       openAIString    `json:"type"`
	Text       openAIString    `json:"text"`
	ImageURL   json.RawMessage `json:"image_url"`
	InputAudio json.RawMessage `json:"input_audio"`
}

type openAIChatImageURL struct {
	URL    openAIString `json:"url"`
	Detail openAIString `json:"detail"`
}

type openAIChatInputAudio struct {
	Data   openAIString `json:"data"`
	Format openAIString `json:"format"`
}

type openAIChatTool struct {
	Function json.RawMessage `json:"function"`
}

type openAIChatFunction struct {
	Name        openAIString    `json:"name"`
	Description openAIString    `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openAIChatToolCall struct {
	ID       openAIString    `json:"id"`
	Type     openAIString    `json:"type"`
	Function json.RawMessage `json:"function"`
}

type openAIChatToolCallFunction struct {
	Name      openAIString    `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type openAIChatResponseFormat struct {
	Type       openAIString    `json:"type"`
	JSONSchema json.RawMessage `json:"json_schema"`
}

type openAIChatToolChoiceObject struct {
	Type     openAIString    `json:"type"`
	Function json.RawMessage `json:"function"`
}

type openAIChatToolChoiceFunction struct {
	Name openAIString `json:"name"`
}

type openAIChatStreamOptions struct {
	IncludeUsage openAIBool `json:"include_usage"`
}

type openAIChatAudio struct {
	Voice  openAIString `json:"voice"`
	Format openAIString `json:"format"`
}
