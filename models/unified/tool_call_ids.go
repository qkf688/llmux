package unified

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
)

var fallbackToolCallIDCounter uint64

// NormalizeToolCallIDs fills missing tool_call IDs to keep tool calling consistent.
//
// Some clients omit `tool_calls[].id` on assistant messages while still sending
// `tool_call_id` on subsequent tool messages. This normalizes both sides:
// - Fills missing assistant tool_call IDs from subsequent tool messages when possible.
// - Fills missing tool message tool_call_id from the corresponding assistant tool_call.
// - Falls back to generating unique IDs if no candidate exists.
func (r *UnifiedRequest) NormalizeToolCallIDs() {
	if r == nil || len(r.Messages) == 0 {
		return
	}

	used := make(map[string]struct{}, len(r.Messages))
	for i := range r.Messages {
		msg := r.Messages[i]
		if msg.ToolCallID != "" {
			used[msg.ToolCallID] = struct{}{}
		}
		for _, tc := range msg.ToolCalls {
			if tc.ID != "" {
				used[tc.ID] = struct{}{}
			}
		}
	}

	for msgIndex := 0; msgIndex < len(r.Messages); msgIndex++ {
		msg := &r.Messages[msgIndex]
		if msg.Role != "assistant" || len(msg.ToolCalls) == 0 {
			continue
		}

		toolMsgs := make([]*UnifiedMessage, 0, len(msg.ToolCalls))
		for nextIndex := msgIndex + 1; nextIndex < len(r.Messages); nextIndex++ {
			nextMsg := &r.Messages[nextIndex]
			if nextMsg.Role != "tool" {
				break
			}
			toolMsgs = append(toolMsgs, nextMsg)
		}

		existingToolCallIDs := make(map[string]struct{}, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			if tc.ID != "" {
				existingToolCallIDs[tc.ID] = struct{}{}
			}
		}

		candidates := make([]string, 0, len(toolMsgs))
		for _, toolMsg := range toolMsgs {
			if toolMsg.ToolCallID == "" {
				continue
			}
			if _, alreadyUsedByToolCalls := existingToolCallIDs[toolMsg.ToolCallID]; alreadyUsedByToolCalls {
				continue
			}
			candidates = append(candidates, toolMsg.ToolCallID)
		}

		candidateIndex := 0
		for i := range msg.ToolCalls {
			if msg.ToolCalls[i].ID != "" {
				continue
			}

			var id string
			if candidateIndex < len(candidates) {
				id = candidates[candidateIndex]
				candidateIndex++
			} else {
				id = generateUniqueToolCallID(used)
			}

			msg.ToolCalls[i].ID = id
			used[id] = struct{}{}
		}

		// Best-effort: fill missing tool message IDs from tool_calls by position.
		for i := 0; i < len(toolMsgs) && i < len(msg.ToolCalls); i++ {
			if toolMsgs[i].ToolCallID == "" && msg.ToolCalls[i].ID != "" {
				toolMsgs[i].ToolCallID = msg.ToolCalls[i].ID
			}
		}
	}
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
