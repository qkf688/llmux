package openai

import (
	"encoding/json"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform/shared"
)

func parseOpenAIChatStop(raw json.RawMessage) *models.UnifiedStop {
	if len(raw) == 0 || shared.IsJSONNull(raw) {
		return nil
	}
	stop := &models.UnifiedStop{}
	if value, ok := shared.RawString(raw); ok {
		stop.Single = &value
		return stop
	}
	var values shared.OptionalStringSeq
	if err := json.Unmarshal(raw, &values); err == nil && values.Set {
		stop.Multiple = values.Value
		return stop
	}
	return stop
}

func parseOpenAIChatResponseFormat(raw json.RawMessage) *models.UnifiedResponseFormat {
	var rf openAIChatResponseFormat
	if !shared.DecodeJSONObject(raw, &rf) {
		return nil
	}
	return &models.UnifiedResponseFormat{
		Type:       rf.Type.Value,
		JSONSchema: rf.JSONSchema,
	}
}

func parseOpenAIChatToolChoice(raw json.RawMessage) *models.UnifiedToolChoice {
	if len(raw) == 0 || shared.IsJSONNull(raw) {
		return nil
	}
	choice := &models.UnifiedToolChoice{}
	if value, ok := shared.RawString(raw); ok {
		choice.StringValue = &value
		return choice
	}

	var obj openAIChatToolChoiceObject
	if !shared.DecodeJSONObject(raw, &obj) {
		return choice
	}
	unifiedObj := models.UnifiedToolChoiceObject{Type: obj.Type.Value}
	var fn openAIChatToolChoiceFunction
	if shared.DecodeJSONObject(obj.Function, &fn) {
		unifiedObj.Function = &models.UnifiedToolChoiceFunction{Name: fn.Name.Value}
	}
	choice.ObjectValue = &unifiedObj
	return choice
}

func parseOpenAIChatStreamOptions(raw json.RawMessage) *models.UnifiedStreamOptions {
	var streamOptions openAIChatStreamOptions
	if !shared.DecodeJSONObject(raw, &streamOptions) {
		return nil
	}
	return &models.UnifiedStreamOptions{IncludeUsage: streamOptions.IncludeUsage.Value}
}

func parseOpenAIChatAudio(raw json.RawMessage) *models.UnifiedAudio {
	var audio openAIChatAudio
	if !shared.DecodeJSONObject(raw, &audio) {
		return nil
	}
	return &models.UnifiedAudio{
		Voice:  audio.Voice.Value,
		Format: audio.Format.Value,
	}
}
