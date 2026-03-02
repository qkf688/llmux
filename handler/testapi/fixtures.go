package testapi

import (
	"errors"

	"github.com/atopos31/llmio/consts"
)

const (
	testOpenAI = `{
        "model": "gpt-4.1",
        "messages": [
            {
                "role": "user",
                "content": "Write a one-sentence bedtime story about a unicorn."
            }
        ]
    }`

	testOpenAIRes = `{
        "model": "gpt-5-nano",
        "input": "Write a one-sentence bedtime story about a unicorn."
    }`

	testAnthropic = `{
     	"model": "claude-sonnet-4-5",
     	"max_tokens": 1000,
     	"messages": [
       		{
         		"role": "user",
         		"content": "Write a one-sentence bedtime story about a unicorn."
       		}
     	]
	}`

	testStructuredOutputSchema = `{
		"type": "object",
		"properties": {
			"language": { "type": "string" },
			"version": { "type": "integer" },
			"features": { "type": "array", "items": { "type": "string" } },
			"score": { "type": "number" },
			"passed": { "type": "boolean" }
		},
		"required": ["language", "version", "features", "score", "passed"],
		"additionalProperties": false
	}`

	testOpenAIStructuredOutput = `{
		"model": "gpt-4.1",
		"temperature": 0,
		"messages": [
			{
				"role": "user",
				"content": "请严格按 JSON Schema 输出一个 JSON 对象，不要输出任何额外文本。"
			}
		],
		"response_format": {
			"type": "json_schema",
			"json_schema": {
				"name": "structured_output_test",
				"strict": true,
				"schema": ` + testStructuredOutputSchema + `
			}
		}
	}`

	testOpenAIResStructuredOutput = `{
		"model": "gpt-5-nano",
		"input": "请严格按 JSON Schema 输出一个 JSON 对象，不要输出任何额外文本。",
		"text": {
			"format": {
				"type": "json_schema",
				"json_schema": {
					"name": "structured_output_test",
					"strict": true,
					"schema": ` + testStructuredOutputSchema + `
				}
			}
		}
	}`

	testAnthropicStructuredOutput = `{
		"model": "claude-sonnet-4-5",
		"max_tokens": 1000,
		"messages": [
			{
				"role": "user",
				"content": "请调用 structured_output 工具，并在工具入参中填充 JSON Schema 所需字段。不要输出额外文本。"
			}
		],
		"tools": [
			{
				"name": "structured_output",
				"description": "Emit structured output that conforms to the schema.",
				"input_schema": ` + testStructuredOutputSchema + `
			}
		],
		"tool_choice": { "type": "tool", "name": "structured_output" }
	}`
)

var errInvalidProviderType = errors.New("invalid provider type")

func buildTestBody(providerType string) ([]byte, error) {
	switch providerType {
	case consts.StyleOpenAI:
		return []byte(testOpenAI), nil
	case consts.StyleAnthropic:
		return []byte(testAnthropic), nil
	case consts.StyleOpenAIRes:
		return []byte(testOpenAIRes), nil
	default:
		return nil, errInvalidProviderType
	}
}

func buildStructuredOutputTestBody(providerType string) ([]byte, error) {
	switch providerType {
	case consts.StyleOpenAI:
		return []byte(testOpenAIStructuredOutput), nil
	case consts.StyleAnthropic:
		return []byte(testAnthropicStructuredOutput), nil
	case consts.StyleOpenAIRes:
		return []byte(testOpenAIResStructuredOutput), nil
	default:
		return nil, errInvalidProviderType
	}
}
