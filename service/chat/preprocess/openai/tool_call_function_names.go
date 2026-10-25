package openai

import (
	"encoding/json"
	"log/slog"
)

// SanitizeToolCallFunctionNames 就地清除 OpenAI Chat 请求里缺 function.name 的
// tool_call，并连带删掉与之配对的 role:tool 消息，返回修补后的 body。
//
// 为什么是「丢弃 + 连带删」而不是继续 400：这类脏 tool_call 是上游空名 tool_use
// 经客户端写回对话历史后的遗留——拒绝它等于把已经脏掉的会话永久卡死（重试永远
// 撞同一条脏消息），而丢弃只损失那一轮的工具上下文，会话可以自愈。
//
// 连带删 role:tool 消息的理由：tool 消息的 tool_call_id 指向被删的 tool_call，
// 留着会让上游因「找不到对应 tool_call」而 400。
//
// 调用方必须先跑 FillMissingToolCallIDs：配对依赖 tool_call.id 与
// tool_message.tool_call_id 的一致性，补 id 之后两者才能对上。
//
// Best-effort / non-breaking：请求无法解析时原样返回（与 FillMissingToolCallIDs
// 同口径）；修补后 messages 为空时同样原样返回（全脏的请求交给上游判，
// 不在网关层发明新语义）。
func SanitizeToolCallFunctionNames(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
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

	// 第一遍：找出所有要删的 tool_call（空名 / function 缺失或非对象），
	// 记录其 id 供第二遍配对 tool 消息。
	type dropRef struct {
		msgIndex int
		tcIndex  int
	}
	var drops []dropRef
	droppedIDs := map[string]bool{}

	for msgIndex := range rawMessages {
		msgMap, ok := rawMessages[msgIndex].(map[string]any)
		if !ok {
			continue
		}
		if role, _ := msgMap["role"].(string); role != "assistant" {
			continue
		}

		rawToolCalls, ok := msgMap["tool_calls"].([]any)
		if !ok || len(rawToolCalls) == 0 {
			continue
		}

		for tcIndex := range rawToolCalls {
			tcMap, ok := rawToolCalls[tcIndex].(map[string]any)
			if !ok {
				continue
			}

			// Future-proofing: 只处理 function 工具调用。
			if typ, _ := tcMap["type"].(string); typ != "" && typ != "function" {
				continue
			}

			drop := false
			functionRaw, exists := tcMap["function"]
			if !exists || functionRaw == nil {
				drop = true
			} else if functionMap, ok := functionRaw.(map[string]any); !ok {
				drop = true
			} else if name, _ := functionMap["name"].(string); name == "" {
				drop = true
			}
			if !drop {
				continue
			}

			drops = append(drops, dropRef{msgIndex, tcIndex})
			if id, _ := tcMap["id"].(string); id != "" {
				droppedIDs[id] = true
			}
		}
	}

	if len(drops) == 0 {
		return body, false, nil
	}

	// 第二遍：删 tool_call（从后往前删避免索引漂移），空数组时移除整个 key。
	for i := len(drops) - 1; i >= 0; i-- {
		d := drops[i]
		msgMap, _ := rawMessages[d.msgIndex].(map[string]any)
		if msgMap == nil {
			continue
		}
		rawToolCalls, _ := msgMap["tool_calls"].([]any)
		if d.tcIndex >= len(rawToolCalls) {
			continue
		}
		tcMap, _ := rawToolCalls[d.tcIndex].(map[string]any)
		if tcMap == nil {
			continue
		}
		slog.Warn("dropping tool call without function name",
			"message_index", d.msgIndex,
			"tool_call_index", d.tcIndex,
			"tool_call_id", tcMap["id"],
		)
		rawToolCalls = append(rawToolCalls[:d.tcIndex], rawToolCalls[d.tcIndex+1:]...)
		if len(rawToolCalls) == 0 {
			delete(msgMap, "tool_calls")
			// 丢掉全部 tool_calls 后 content 为 null/缺失的 assistant 消息
			// 会变成「无内容无工具调用」——多数上游拒收，补空串保证自愈成立。
			if content, exists := msgMap["content"]; !exists || content == nil {
				msgMap["content"] = ""
			}
		} else {
			msgMap["tool_calls"] = rawToolCalls
		}
	}

	// 第三遍：删配对的 role:tool 消息。
	kept := make([]any, 0, len(rawMessages))
	for _, raw := range rawMessages {
		msgMap, ok := raw.(map[string]any)
		if !ok {
			kept = append(kept, raw)
			continue
		}
		if role, _ := msgMap["role"].(string); role == "tool" {
			if id, _ := msgMap["tool_call_id"].(string); droppedIDs[id] {
				slog.Warn("dropping tool message for dropped tool call",
					"tool_call_id", id,
				)
				continue
			}
		}
		kept = append(kept, raw)
	}

	req["messages"] = kept

	updated, err := json.Marshal(req)
	if err != nil {
		return body, false, err
	}
	return updated, true, nil
}
