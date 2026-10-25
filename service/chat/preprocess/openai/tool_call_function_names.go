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
// 连带删 role:tool 消息的作用域是「被删 tool_call 所在 assistant 消息之后的连续
// role:tool 段」，不是全请求按 tool_call_id 扫——后者会把复用同一 id 的其它轮
// 结果一并删掉，那轮 assistant 只剩没有结果的 tool_call，下一轮上游照样 400
// （自愈反而制造新的脏数据）。段内判据认「tool 消息的 tool_call_id 是否落在该条
// assistant 存活 tool_call 的 id 集合里」：落不进去的就是与已删调用配对的孤儿
// （客户端写回时 id 会错位），留着会让上游因「找不到对应 tool_call」400。
//
// 调用方必须先跑 FillMissingToolCallIDs：配对依赖 tool_call.id 与
// tool_message.tool_call_id 的一致性，补 id 之后存活集合才有意义。
//
// Best-effort / non-blocking：请求无法解析时原样返回（与 FillMissingToolCallIDs
// 同口径）；全脏请求仍会改写（assistant 消息保留、tool_calls 移除、content 补
// 空串），交上游判定，不在网关层发明新语义。
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
	// 并记下「哪些 assistant 消息删过 tool_call」——只有这些消息之后的
	// role:tool 段才需要连带清理。
	type dropRef struct {
		msgIndex int
		tcIndex  int
	}
	var drops []dropRef
	dirtyAssistants := map[int]bool{}

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
			dirtyAssistants[msgIndex] = true
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

	// 第三遍：算每条脏 assistant 消息删除后仍存活的 tool_call id 集合。
	// 从删除后的最终状态取——中途状态会把「同样待删的其它条目」算成存活。
	survivingByAssistant := map[int]map[string]bool{}
	for msgIndex := range dirtyAssistants {
		surviving := map[string]bool{}
		msgMap, _ := rawMessages[msgIndex].(map[string]any)
		if msgMap != nil {
			if rawToolCalls, ok := msgMap["tool_calls"].([]any); ok {
				for _, raw := range rawToolCalls {
					tcMap, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					id, _ := tcMap["id"].(string)
					surviving[id] = true
				}
			}
		}
		survivingByAssistant[msgIndex] = surviving
	}

	// 第四遍：圈定每条 role:tool 消息归属的「连续段」——即它前面那条删过
	// tool_call 的 assistant 消息之后的连续 role:tool 序列。OpenAI 形状里
	// tool 消息紧跟发出调用的 assistant，故段边界取到首个非 tool 角色为止。
	toolRunOwner := map[int]int{}
	for msgIndex := range dirtyAssistants {
		for next := msgIndex + 1; next < len(rawMessages); next++ {
			nextMsg, ok := rawMessages[next].(map[string]any)
			if !ok {
				break
			}
			if role, _ := nextMsg["role"].(string); role != "tool" {
				break
			}
			toolRunOwner[next] = msgIndex
		}
	}

	// 第五遍：段内按存活集合判删 tool 消息，重建 messages。
	kept := make([]any, 0, len(rawMessages))
	for msgIndex, raw := range rawMessages {
		msgMap, ok := raw.(map[string]any)
		if !ok {
			kept = append(kept, raw)
			continue
		}
		if owner, inRun := toolRunOwner[msgIndex]; inRun {
			id, _ := msgMap["tool_call_id"].(string)
			if !survivingByAssistant[owner][id] {
				slog.Warn("dropping tool message without surviving tool call",
					"message_index", msgIndex,
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
