package responses

import (
	"encoding/json"
	"testing"
)

// Responses 协议里同一轮的并行工具调用是 input 数组中**连续**的多个 function_call item，
// 其 function_call_output 紧随其后。转换到 OpenAI Chat 形态时，这批调用必须落进
// **同一条** assistant 消息的 tool_calls 数组——Chat 协议要求「一批 tool_calls 的结果
// 紧跟在该 assistant 之后」。若每个 function_call 各成一条 assistant 消息，结果消息
// 前面会隔着别的 assistant 消息，上游判「tool calls and tool results do not match」
// 并顶死整条会话（上游错误码 11148）。
//
// 本文件锁定两件事：
//  1. 入站：连续的 function_call 合并进同一条 unified assistant 消息；
//  2. 出站（OpenAI Chat）：配对对称——每个 tool_call id 都有结果，且结果紧跟其调用组。

func TestTransformRequest_ParallelFunctionCallsMergeIntoOneAssistant(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"读两个文件"}]},
		{"type":"function_call","call_id":"call_A","name":"read_file","arguments":"{\"p\":\"a\"}"},
		{"type":"function_call","call_id":"call_B","name":"read_file","arguments":"{\"p\":\"b\"}"},
		{"type":"function_call_output","call_id":"call_A","output":"A"},
		{"type":"function_call_output","call_id":"call_B","output":"B"}
	]}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	// 期望形状：user / assistant(tool_calls=[A,B]) / tool(A) / tool(B)
	if len(unified.Messages) != 4 {
		t.Fatalf("期望 4 条消息（user + 合并后的 assistant + 2 条 tool），实际 %d 条: %#v",
			len(unified.Messages), unified.Messages)
	}

	assistant := unified.Messages[1]
	if assistant.Role != "assistant" {
		t.Fatalf("第 2 条应为 assistant，实际 %q", assistant.Role)
	}
	if len(assistant.ToolCalls) != 2 {
		t.Fatalf("并行调用应合并进同一条 assistant 消息，实际 tool_calls 数 %d: %#v",
			len(assistant.ToolCalls), assistant.ToolCalls)
	}
	if assistant.ToolCalls[0].ID != "call_A" || assistant.ToolCalls[1].ID != "call_B" {
		t.Fatalf("合并后 tool_calls id 应保持原顺序 [call_A call_B]，实际 [%s %s]",
			assistant.ToolCalls[0].ID, assistant.ToolCalls[1].ID)
	}

	// 结果消息紧随其后，且 id 与调用一一对应
	for i, wantID := range []string{"call_A", "call_B"} {
		got := unified.Messages[2+i]
		if got.Role != "tool" {
			t.Fatalf("第 %d 条应为 tool，实际 %q", 3+i, got.Role)
		}
		if got.ToolCallID != wantID {
			t.Fatalf("第 %d 条 tool 的 tool_call_id 应为 %q，实际 %q", 3+i, wantID, got.ToolCallID)
		}
	}
}

// 单个 function_call 不受影响（不产生「多一条空 assistant」这类回归）。
func TestTransformRequest_SingleFunctionCallUnchanged(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
		{"type":"function_call","call_id":"call_A","name":"read_file","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_A","output":"A"}
	]}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if len(unified.Messages) != 3 {
		t.Fatalf("期望 3 条消息，实际 %d 条: %#v", len(unified.Messages), unified.Messages)
	}
	if unified.Messages[1].Role != "assistant" || len(unified.Messages[1].ToolCalls) != 1 {
		t.Fatalf("单调用应仍是一条 assistant 带 1 个 tool_call，实际 %#v", unified.Messages[1])
	}
	if unified.Messages[2].Role != "tool" || unified.Messages[2].ToolCallID != "call_A" {
		t.Fatalf("tool 结果消息不正确: %#v", unified.Messages[2])
	}
}

// 被非 function_call item 隔开的两批调用**不得**合并：它们是两轮独立调用。
func TestTransformRequest_SeparateCallBatchesNotMerged(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":[
		{"type":"function_call","call_id":"call_A","name":"t","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_A","output":"A"},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"再来"}]},
		{"type":"function_call","call_id":"call_B","name":"t","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_B","output":"B"}
	]}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	assistantCount := 0
	for _, m := range unified.Messages {
		if m.Role == "assistant" {
			assistantCount++
			if len(m.ToolCalls) != 1 {
				t.Fatalf("两轮独立调用不得合并，实际 %#v", m)
			}
		}
	}
	if assistantCount != 2 {
		t.Fatalf("期望 2 条 assistant 消息，实际 %d", assistantCount)
	}
}

// 出站（OpenAI Chat）配对对称性：每个 tool_call id 都有结果，且结果紧跟其调用组。
// 这是上游判 11148 的直接判据——断在「结果不紧跟调用」这一位置型断裂上。
func TestTransformRequest_ChatOutboundPairingSymmetric(t *testing.T) {
	raw := []byte(`{"model":"gpt-4.1","input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"读两个文件"}]},
		{"type":"function_call","call_id":"call_A","name":"read_file","arguments":"{}"},
		{"type":"function_call","call_id":"call_B","name":"read_file","arguments":"{}"},
		{"type":"function_call_output","call_id":"call_A","output":"A"},
		{"type":"function_call_output","call_id":"call_B","output":"B"},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"继续"}]}
	]}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	out, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified failed: %v", err)
	}

	var body struct {
		Messages []struct {
			Role      string `json:"role"`
			ToolCalls []struct {
				ID string `json:"id"`
			} `json:"tool_calls"`
			ToolCallID string `json:"tool_call_id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(out, &body); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// 逐条走一遍 messages，维护「当前 assistant 组待回收的 call id 集合」：
	//   - 遇 assistant：上一组若还有没等到结果的 call，说明结果被后来的 assistant
	//     打断了（位置型断裂——上游 11148 判的正是这一种）；
	//   - 遇 tool：其 id 必须已在待回收集合内（否则结果与调用脱节）；
	//   - 遇其它角色：待回收集合必须为空（说明没有悬空调用）。
	pending := map[string]bool{}
	for i, m := range body.Messages {
		switch m.Role {
		case "assistant":
			if len(pending) > 0 {
				t.Fatalf("第 %d 条 assistant 打断了上一组工具配对（%d 个调用没等到结果）: %s",
					i, len(pending), out)
			}
			for _, tc := range m.ToolCalls {
				pending[tc.ID] = true
			}
		case "tool":
			if !pending[m.ToolCallID] {
				t.Fatalf("第 %d 条 tool 结果 %q 的调用不在紧邻的 assistant 组内（配对断裂）: %s",
					i, m.ToolCallID, out)
			}
			delete(pending, m.ToolCallID)
		default:
			if len(pending) > 0 {
				t.Fatalf("第 %d 条 %s 消息打断了工具配对（%d 个调用悬空）: %s",
					i, m.Role, len(pending), out)
			}
		}
	}
	if len(pending) > 0 {
		t.Fatalf("结尾仍有 %d 个 tool_call 没有结果: %s", len(pending), out)
	}
}
