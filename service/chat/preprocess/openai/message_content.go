package openai

import (
	"encoding/json"

	"github.com/tidwall/gjson"
)

// FillMissingMessageContent best-effort normalizes OpenAI Chat `messages[].content`.
//
// Some clients omit `content` (or send it as `null`) on assistant/tool messages when using tool calls.
// Many OpenAI-compatible providers validate this strictly and return a 400. This function patches
// missing/null `content` to an empty string to improve compatibility.
//
// Best-effort / non-breaking: if the request cannot be parsed, this returns unchanged.
func FillMissingMessageContent(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	// Fast-path: only patch when we actually detect missing/null content.
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() || len(messages.Array()) == 0 {
		return body, false, nil
	}

	needPatch := false
	messages.ForEach(func(_, msg gjson.Result) bool {
		content := msg.Get("content")
		if !content.Exists() || content.Type == gjson.Null {
			needPatch = true
			return false
		}
		return true
	})
	if !needPatch {
		return body, false, nil
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return body, false, nil
	}

	rawMessages, ok := req["messages"].([]any)
	if !ok || len(rawMessages) == 0 {
		return body, false, nil
	}

	changed := false
	for idx := range rawMessages {
		msgMap, ok := rawMessages[idx].(map[string]any)
		if !ok {
			continue
		}
		content, exists := msgMap["content"]
		if exists && content != nil {
			continue
		}
		msgMap["content"] = ""
		changed = true
	}

	if !changed {
		return body, false, nil
	}

	updated, err := json.Marshal(req)
	if err != nil {
		return body, false, err
	}
	return updated, true, nil
}

