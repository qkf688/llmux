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
