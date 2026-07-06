package openai

import (
	"bytes"
	"encoding/json"

	"github.com/atopos31/llmio/models"
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

type openAIString struct {
	Value string
	Set   bool
}

func (f *openAIString) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = value
		f.Set = true
	}
	return nil
}

type openAIBool struct {
	Value bool
	Set   bool
}

func (f *openAIBool) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var value bool
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = value
		f.Set = true
	}
	return nil
}

type openAIFloat struct {
	Value float64
	Set   bool
}

func (f *openAIFloat) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var value float64
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = value
		f.Set = true
	}
	return nil
}

type openAIInt struct {
	Value int
	Set   bool
}

func (f *openAIInt) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var value float64
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = int(value)
		f.Set = true
	}
	return nil
}

type openAIInt64 struct {
	Value int64
	Set   bool
}

func (f *openAIInt64) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var value float64
	if err := json.Unmarshal(data, &value); err == nil {
		f.Value = int64(value)
		f.Set = true
	}
	return nil
}

type openAIInt64Map struct {
	Value map[string]int64
	Set   bool
}

func (f *openAIInt64Map) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]int64, len(raw))
	for key, value := range raw {
		if num, ok := value.(float64); ok {
			result[key] = int64(num)
		}
	}
	f.Value = result
	f.Set = true
	return nil
}

type openAIStringMap struct {
	Value map[string]string
	Set   bool
}

func (f *openAIStringMap) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]string, len(raw))
	for key, value := range raw {
		if str, ok := value.(string); ok {
			result[key] = str
		}
	}
	f.Value = result
	f.Set = true
	return nil
}

type openAIStringSeq struct {
	Value []string
	Set   bool
}

func (f *openAIStringSeq) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var raw []interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, value := range raw {
		if str, ok := value.(string); ok {
			result = append(result, str)
		}
	}
	f.Value = result
	f.Set = true
	return nil
}

type openAIRawArray []json.RawMessage

func (a *openAIRawArray) UnmarshalJSON(data []byte) error {
	if isOpenAINullRaw(data) {
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		*a = raw
	}
	return nil
}

func decodeOpenAIChatObject(data json.RawMessage, target interface{}) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	return json.Unmarshal(data, target) == nil
}

func isOpenAINullRaw(data json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}

func rawOpenAIValue(data json.RawMessage) interface{} {
	if len(data) == 0 {
		return nil
	}
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	return value
}

func rawOpenAIString(data json.RawMessage) (string, bool) {
	var value string
	if len(data) == 0 || json.Unmarshal(data, &value) != nil {
		return "", false
	}
	return value, true
}

func parseOpenAIChatMessages(messages openAIRawArray) ([]models.UnifiedMessage, string) {
	decoded := make([]openAIChatMessage, 0, len(messages))
	for _, raw := range messages {
		var msg openAIChatMessage
		if !decodeOpenAIChatObject(raw, &msg) {
			continue
		}
		decoded = append(decoded, msg)
	}

	nonSystemCount := 0
	for _, msg := range decoded {
		if msg.Role.Value != "system" {
			nonSystemCount++
		}
	}
	extractSystem := nonSystemCount > 0

	var unifiedMessages []models.UnifiedMessage
	system := ""
	for _, msg := range decoded {
		role := msg.Role.Value
		if role == "system" && extractSystem {
			if content, ok := rawOpenAIString(msg.Content); ok && content != "" {
				if system != "" {
					system += "\n\n" + content
				} else {
					system = content
				}
			}
			continue
		}

		unified := models.UnifiedMessage{
			Role:      role,
			Content:   parseOpenAIChatMessageContent(msg.Content),
			ToolCalls: parseOpenAIChatToolCalls(msg.ToolCalls),
		}
		if role == "tool" && msg.ToolCallID.Set {
			unified.ToolCallID = msg.ToolCallID.Value
		}
		unifiedMessages = append(unifiedMessages, unified)
	}

	return unifiedMessages, system
}

func parseOpenAIChatMessageContent(raw json.RawMessage) interface{} {
	content := rawOpenAIValue(raw)
	if content == nil {
		return nil
	}

	var rawParts []json.RawMessage
	if err := json.Unmarshal(raw, &rawParts); err != nil {
		return content
	}

	parts := make([]models.UnifiedMessageContentPart, 0, len(rawParts))
	for _, rawPart := range rawParts {
		var part openAIChatContentPart
		if !decodeOpenAIChatObject(rawPart, &part) {
			continue
		}

		unifiedPart := models.UnifiedMessageContentPart{Type: part.Type.Value}
		switch unifiedPart.Type {
		case "text":
			if part.Text.Set {
				text := part.Text.Value
				unifiedPart.Text = &text
			}
		case "image_url":
			var image openAIChatImageURL
			if decodeOpenAIChatObject(part.ImageURL, &image) {
				var detail *string
				if image.Detail.Set {
					detail = &image.Detail.Value
				}
				unifiedPart.ImageURL = &models.UnifiedImageURL{
					URL:    image.URL.Value,
					Detail: detail,
				}
			}
		case "input_audio":
			var audio openAIChatInputAudio
			if decodeOpenAIChatObject(part.InputAudio, &audio) {
				unifiedPart.InputAudio = &models.UnifiedInputAudio{
					Data:   audio.Data.Value,
					Format: audio.Format.Value,
				}
			}
		}

		parts = append(parts, unifiedPart)
	}
	if len(parts) == 0 {
		return content
	}
	return parts
}

func parseOpenAIChatTools(tools openAIRawArray) []models.UnifiedTool {
	var unified []models.UnifiedTool
	for _, raw := range tools {
		var tool openAIChatTool
		if !decodeOpenAIChatObject(raw, &tool) {
			continue
		}
		var fn openAIChatFunction
		if !decodeOpenAIChatObject(tool.Function, &fn) {
			continue
		}
		unified = append(unified, models.UnifiedTool{
			Type: "function",
			Function: models.UnifiedFunc{
				Name:        fn.Name.Value,
				Description: fn.Description.Value,
				Parameters:  rawOpenAIValue(fn.Parameters),
			},
		})
	}
	return unified
}

func parseOpenAIChatToolCalls(toolCalls openAIRawArray) []models.UnifiedToolCall {
	var unified []models.UnifiedToolCall
	for _, raw := range toolCalls {
		var toolCall openAIChatToolCall
		if !decodeOpenAIChatObject(raw, &toolCall) {
			continue
		}
		var fn openAIChatToolCallFunction
		if !decodeOpenAIChatObject(toolCall.Function, &fn) {
			continue
		}
		unified = append(unified, models.UnifiedToolCall{
			ID:   toolCall.ID.Value,
			Type: toolCall.Type.Value,
			Function: models.UnifiedToolCallFunction{
				Name:      fn.Name.Value,
				Arguments: normalizeOpenAIChatToolCallArguments(fn.Arguments),
			},
		})
	}
	return unified
}

func normalizeOpenAIChatToolCallArguments(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if str != "" {
			return str
		}
		return "{}"
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if b, err := json.Marshal(obj); err == nil && len(b) > 0 && string(b) != "null" {
			return string(b)
		}
	}
	var arr []interface{}
	if err := json.Unmarshal(raw, &arr); err == nil {
		if b, err := json.Marshal(arr); err == nil && len(b) > 0 && string(b) != "null" {
			return string(b)
		}
	}
	return "{}"
}

func parseOpenAIChatStop(raw json.RawMessage) *models.UnifiedStop {
	if len(raw) == 0 || isOpenAINullRaw(raw) {
		return nil
	}
	stop := &models.UnifiedStop{}
	if value, ok := rawOpenAIString(raw); ok {
		stop.Single = &value
		return stop
	}
	var values openAIStringSeq
	if err := json.Unmarshal(raw, &values); err == nil && values.Set {
		stop.Multiple = values.Value
		return stop
	}
	return stop
}

func parseOpenAIChatResponseFormat(raw json.RawMessage) *models.UnifiedResponseFormat {
	var rf openAIChatResponseFormat
	if !decodeOpenAIChatObject(raw, &rf) {
		return nil
	}
	return &models.UnifiedResponseFormat{
		Type:       rf.Type.Value,
		JSONSchema: rf.JSONSchema,
	}
}

func parseOpenAIChatToolChoice(raw json.RawMessage) *models.UnifiedToolChoice {
	if len(raw) == 0 || isOpenAINullRaw(raw) {
		return nil
	}
	choice := &models.UnifiedToolChoice{}
	if value, ok := rawOpenAIString(raw); ok {
		choice.StringValue = &value
		return choice
	}

	var obj openAIChatToolChoiceObject
	if !decodeOpenAIChatObject(raw, &obj) {
		return choice
	}
	unifiedObj := models.UnifiedToolChoiceObject{Type: obj.Type.Value}
	var fn openAIChatToolChoiceFunction
	if decodeOpenAIChatObject(obj.Function, &fn) {
		unifiedObj.Function = &models.UnifiedToolChoiceFunction{Name: fn.Name.Value}
	}
	choice.ObjectValue = &unifiedObj
	return choice
}

func parseOpenAIChatStreamOptions(raw json.RawMessage) *models.UnifiedStreamOptions {
	var streamOptions openAIChatStreamOptions
	if !decodeOpenAIChatObject(raw, &streamOptions) {
		return nil
	}
	return &models.UnifiedStreamOptions{IncludeUsage: streamOptions.IncludeUsage.Value}
}

func parseOpenAIChatAudio(raw json.RawMessage) *models.UnifiedAudio {
	var audio openAIChatAudio
	if !decodeOpenAIChatObject(raw, &audio) {
		return nil
	}
	return &models.UnifiedAudio{
		Voice:  audio.Voice.Value,
		Format: audio.Format.Value,
	}
}
