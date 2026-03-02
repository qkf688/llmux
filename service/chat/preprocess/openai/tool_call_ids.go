package openai

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

var fallbackToolCallIDCounter uint64

// FillMissingToolCallIDs best-effort normalizes OpenAI Chat `tool_calls[].id`.
//
// Some clients omit `tool_calls[].id` on assistant messages while still sending
// `tool_call_id` on subsequent tool messages. This function fills missing IDs
// without performing a full request format conversion (safe for passthrough).
func FillMissingToolCallIDs(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		// Keep behavior non-breaking: if we can't parse, don't block the request.
		return body, false, nil
	}

	rawMessages, ok := req["messages"].([]any)
	if !ok || len(rawMessages) == 0 {
		return body, false, nil
	}

	used := collectUsedToolCallIDs(rawMessages)
	changed := false

	for msgIndex := 0; msgIndex < len(rawMessages); msgIndex++ {
		msgMap, ok := rawMessages[msgIndex].(map[string]any)
		if !ok {
			continue
		}
		role, _ := msgMap["role"].(string)
		if role != "assistant" {
			continue
		}

		rawToolCalls, ok := msgMap["tool_calls"].([]any)
		if !ok || len(rawToolCalls) == 0 {
			continue
		}

		existingToolCallIDs := make(map[string]struct{}, len(rawToolCalls))
		missingToolCallIndices := make([]int, 0, len(rawToolCalls))
		toolCallMaps := make([]map[string]any, len(rawToolCalls))

		for i := range rawToolCalls {
			tcMap, ok := rawToolCalls[i].(map[string]any)
			if !ok {
				continue
			}
			toolCallMaps[i] = tcMap
			id, _ := tcMap["id"].(string)
			if id == "" {
				missingToolCallIndices = append(missingToolCallIndices, i)
				continue
			}
			existingToolCallIDs[id] = struct{}{}
		}

		toolMsgs := make([]map[string]any, 0, len(rawToolCalls))
		for nextIndex := msgIndex + 1; nextIndex < len(rawMessages); nextIndex++ {
			nextMsg, ok := rawMessages[nextIndex].(map[string]any)
			if !ok {
				break
			}
			nextRole, _ := nextMsg["role"].(string)
			if nextRole != "tool" {
				break
			}
			toolMsgs = append(toolMsgs, nextMsg)
		}

		candidates := make([]string, 0, len(toolMsgs))
		for _, toolMsg := range toolMsgs {
			id, _ := toolMsg["tool_call_id"].(string)
			if id == "" {
				continue
			}
			if _, alreadyUsedByToolCalls := existingToolCallIDs[id]; alreadyUsedByToolCalls {
				continue
			}
			candidates = append(candidates, id)
		}

		candidateIndex := 0
		for _, tcIndex := range missingToolCallIndices {
			tcMap := toolCallMaps[tcIndex]
			if tcMap == nil {
				continue
			}

			var id string
			if candidateIndex < len(candidates) {
				id = candidates[candidateIndex]
				candidateIndex++
			} else {
				id = generateUniqueToolCallID(used)
			}

			if id == "" {
				continue
			}

			tcMap["id"] = id
			used[id] = struct{}{}
			changed = true
		}

		// Best-effort: fill missing tool_call_id on tool messages from tool_calls by position.
		for i := 0; i < len(toolMsgs) && i < len(rawToolCalls); i++ {
			toolMsg := toolMsgs[i]
			if toolMsg == nil {
				continue
			}
			existing, _ := toolMsg["tool_call_id"].(string)
			if existing != "" {
				continue
			}

			tcMap := toolCallMaps[i]
			if tcMap == nil {
				continue
			}
			id, _ := tcMap["id"].(string)
			if id == "" {
				continue
			}

			toolMsg["tool_call_id"] = id
			changed = true
		}
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

func collectUsedToolCallIDs(messages []any) map[string]struct{} {
	used := make(map[string]struct{}, len(messages))
	for _, raw := range messages {
		msg, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		if id, _ := msg["tool_call_id"].(string); id != "" {
			used[id] = struct{}{}
		}

		rawToolCalls, ok := msg["tool_calls"].([]any)
		if !ok {
			continue
		}
		for _, rawToolCall := range rawToolCalls {
			tc, ok := rawToolCall.(map[string]any)
			if !ok {
				continue
			}
			if id, _ := tc["id"].(string); id != "" {
				used[id] = struct{}{}
			}
		}
	}
	return used
}

func generateUniqueToolCallID(used map[string]struct{}) string {
	for attempts := 0; attempts < 8; attempts++ {
		buf := make([]byte, 12)
		if _, err := rand.Read(buf); err != nil {
			break
		}
		id := "call_" + hex.EncodeToString(buf)
		if _, exists := used[id]; !exists {
			return id
		}
	}

	for {
		n := atomic.AddUint64(&fallbackToolCallIDCounter, 1)
		id := fmt.Sprintf("call_%d", n)
		if _, exists := used[id]; !exists {
			return id
		}
	}
}
