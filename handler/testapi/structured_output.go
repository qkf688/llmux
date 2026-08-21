package testapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/qkf688/llmux/consts"
	"github.com/tidwall/gjson"
)

var errStructuredOutputNotFound = errors.New("structured output not found in response")

// extractStructuredOutputJSON 按**上游响应体形状**取结构化输出：三个协议放 JSON 的位置不同。
// 判定用 wire format 而非 provider type——OpenAI 兼容上游的响应体同样是 OpenAI Chat 形状，
// 按「哪一家」分支会让每接一家新上游都要来这里加 case。
func extractStructuredOutputJSON(upstreamFormat consts.WireFormat, responseBody []byte) (payload json.RawMessage, rawOutput string, err error) {
	switch upstreamFormat {
	case consts.FormatAnthropic:
		result := gjson.GetBytes(responseBody, `content.#(type=="tool_use").input`)
		if !result.Exists() || result.Type == gjson.Null {
			return nil, "", errStructuredOutputNotFound
		}
		raw := strings.TrimSpace(result.Raw)
		if raw == "" {
			return nil, "", errStructuredOutputNotFound
		}
		return json.RawMessage(raw), raw, nil
	case consts.FormatOpenAIResponses:
		// Prefer output_text items
		outputItems := gjson.GetBytes(responseBody, "output")
		if outputItems.IsArray() {
			for _, item := range outputItems.Array() {
				if item.Get("type").String() == "output_text" {
					text := strings.TrimSpace(item.Get("text").String())
					if text != "" {
						return json.RawMessage(text), text, nil
					}
				}
				// message -> content -> output_text
				if item.Get("type").String() == "message" {
					content := item.Get("content")
					if content.IsArray() {
						for _, part := range content.Array() {
							if part.Get("type").String() == "output_text" {
								text := strings.TrimSpace(part.Get("text").String())
								if text != "" {
									return json.RawMessage(text), text, nil
								}
							}
						}
					}
				}
			}
		}
		return nil, "", errStructuredOutputNotFound
	default:
		// OpenAI chat completions (and OpenAI-compatible providers)
		content := strings.TrimSpace(gjson.GetBytes(responseBody, "choices.0.message.content").String())
		if content == "" {
			return nil, "", errStructuredOutputNotFound
		}
		return json.RawMessage(content), content, nil
	}
}

func validateStructuredOutputPayload(payload json.RawMessage) (map[string]interface{}, error) {
	if len(payload) == 0 {
		return nil, errors.New("empty payload")
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	allowedKeys := map[string]struct{}{
		"language": {},
		"version":  {},
		"features": {},
		"score":    {},
		"passed":   {},
	}

	for k := range obj {
		if _, ok := allowedKeys[k]; !ok {
			return nil, fmt.Errorf("unexpected key: %s", k)
		}
	}
	if len(obj) != len(allowedKeys) {
		return nil, fmt.Errorf("missing required keys: got %d keys", len(obj))
	}

	language, ok := obj["language"].(string)
	if !ok || strings.TrimSpace(language) == "" {
		return nil, errors.New("language must be a non-empty string")
	}

	version, ok := obj["version"].(float64)
	if !ok || math.IsNaN(version) || math.IsInf(version, 0) || math.Mod(version, 1) != 0 {
		return nil, errors.New("version must be an integer")
	}

	features, ok := obj["features"].([]interface{})
	if !ok {
		return nil, errors.New("features must be an array of strings")
	}
	for _, item := range features {
		if _, ok := item.(string); !ok {
			return nil, errors.New("features must be an array of strings")
		}
	}

	score, ok := obj["score"].(float64)
	if !ok || math.IsNaN(score) || math.IsInf(score, 0) {
		return nil, errors.New("score must be a number")
	}

	if _, ok := obj["passed"].(bool); !ok {
		return nil, errors.New("passed must be a boolean")
	}

	return obj, nil
}
