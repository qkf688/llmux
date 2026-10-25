package openai

import (
	"encoding/json"
	"testing"
)

// 本文件覆盖 SanitizeToolCallFunctionNames 的三种核心行为：
//  1. 空名 tool_call 被丢弃，且配对的 role:tool 消息被连带删除；
//  2. 正常 tool_call 原样保留；
//  3. 非法 JSON / 全脏请求非阻塞返回（与 FillMissingToolCallIDs 同口径）。

func TestSanitizeToolCallFunctionNames(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		wantErr     bool
		wantChanged bool
	}{
		{
			name: "empty name tool call dropped with paired tool message",
			body: []byte(`{
				"model":"m",
				"messages":[
					{"role":"user","content":"hi"},
					{"role":"assistant","content":null,"tool_calls":[
						{"id":"call_a","type":"function","function":{"arguments":"{}"}},
						{"id":"call_b","type":"function","function":{"name":"ok_fn","arguments":"{}"}}
					]},
					{"role":"tool","tool_call_id":"call_a","content":"r1"},
					{"role":"tool","tool_call_id":"call_b","content":"r2"},
					{"role":"user","content":"next"}
				]
			}`),
			wantChanged: true,
		},
		{
			name: "missing function object dropped",
			body: []byte(`{
				"model":"m",
				"messages":[
					{"role":"assistant","tool_calls":[
						{"type":"function"}
					]}
				]
			}`),
			wantChanged: true,
		},
		{
			name: "valid tool call unchanged",
			body: []byte(`{
				"model":"m",
				"messages":[
					{"role":"assistant","tool_calls":[
						{"type":"function","function":{"name":"container_shell","arguments":"{}"}}
					]}
				]
			}`),
			wantChanged: false,
		},
		{
			name: "non-function tool call ignored",
			body: []byte(`{
				"model":"m",
				"messages":[
					{"role":"assistant","tool_calls":[
						{"type":"other","function":{}}
					]}
				]
			}`),
			wantChanged: false,
		},
		{
			name:    "invalid json is non-blocking",
			body:    []byte(`{"model":"m","messages":[`),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			patched, changed, err := SanitizeToolCallFunctionNames(tc.body)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
			if !changed {
				if string(patched) != string(tc.body) {
					t.Fatalf("unchanged request should return original body byte-for-byte")
				}
			}
		})
	}
}

// sanitizeMessages 跑一次 sanitize 并返回修补后的 messages 列表。
//
// 断言都落在「修补后请求长什么样」这一层：调用方只需关心 messages 的形状，
// 不必每个用例各写一遍「调用 → 断言 changed → 解 JSON → 取 messages」的样板。
func sanitizeMessages(t *testing.T, body []byte, wantChanged bool) []any {
	t.Helper()

	patched, changed, err := SanitizeToolCallFunctionNames(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed != wantChanged {
		t.Fatalf("changed = %v, want %v", changed, wantChanged)
	}

	var req map[string]any
	if err := json.Unmarshal(patched, &req); err != nil {
		t.Fatalf("patched body is not valid JSON: %v", err)
	}
	msgs, _ := req["messages"].([]any)
	return msgs
}

// TestSanitizeToolCallFunctionNames_DropsPairedToolMessages 验证核心自愈行为：
// 空名 tool_call 的配对 tool 消息必须被连带删除，否则上游会因
// 「找不到对应 tool_call」而 400，会话无法自愈。
func TestSanitizeToolCallFunctionNames_DropsPairedToolMessages(t *testing.T) {
	body := []byte(`{
		"model":"m",
		"messages":[
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_a","type":"function","function":{"arguments":"{}"}}
			]},
			{"role":"tool","tool_call_id":"call_a","content":"r1"},
			{"role":"user","content":"next"}
		]
	}`)

	msgs := sanitizeMessages(t, body, true)
	// 原始 3 条消息：assistant(带脏 tool_call) + tool(配对) + user。
	// 修补后：assistant(content 补空串) + user，tool 消息被删。
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after sanitize, got %d: %v", len(msgs), msgs)
	}

	first, _ := msgs[0].(map[string]any)
	if role, _ := first["role"].(string); role != "assistant" {
		t.Fatalf("first message should be assistant, got %v", first)
	}
	if _, hasToolCalls := first["tool_calls"]; hasToolCalls {
		t.Errorf("assistant message should have tool_calls key removed, got %v", first)
	}
	if content, _ := first["content"].(string); content != "" {
		t.Errorf("assistant content should be filled with empty string, got %q", content)
	}

	second, _ := msgs[1].(map[string]any)
	if role, _ := second["role"].(string); role != "user" {
		t.Fatalf("second message should be user, got %v", second)
	}
}

// TestSanitizeToolCallFunctionNames_KeepsOtherToolCalls 验证部分丢弃：
// 同一 assistant 消息里多个 tool_call，只删空名的那个，其余保留。
func TestSanitizeToolCallFunctionNames_KeepsOtherToolCalls(t *testing.T) {
	body := []byte(`{
		"model":"m",
		"messages":[
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_a","type":"function","function":{"arguments":"{}"}},
				{"id":"call_b","type":"function","function":{"name":"keep_me","arguments":"{}"}}
			]},
			{"role":"tool","tool_call_id":"call_a","content":"r1"},
			{"role":"tool","tool_call_id":"call_b","content":"r2"}
		]
	}`)

	msgs := sanitizeMessages(t, body, true)
	assistant, _ := msgs[0].(map[string]any)
	toolCalls, _ := assistant["tool_calls"].([]any)
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 remaining tool call, got %d", len(toolCalls))
	}
	tc0, _ := toolCalls[0].(map[string]any)
	fn, _ := tc0["function"].(map[string]any)
	if name, _ := fn["name"].(string); name != "keep_me" {
		t.Errorf("remaining tool call should be keep_me, got %q", name)
	}

	// 配对的 tool 消息：call_a 的被删，call_b 的保留。
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (assistant + tool for call_b), got %d", len(msgs))
	}
	toolMsg, _ := msgs[1].(map[string]any)
	if id, _ := toolMsg["tool_call_id"].(string); id != "call_b" {
		t.Errorf("remaining tool message should reference call_b, got %q", id)
	}
}

// TestSanitizeToolCallFunctionNames_AllDirtyKeepsAssistantMessage 验证全脏请求的
// 兜底行为：assistant 消息本身不删（只删内部 tool_calls），tool 消息被删，
// 产出单条 content 为空的 assistant 消息——结构合法，交上游判。
func TestSanitizeToolCallFunctionNames_AllDirtyKeepsAssistantMessage(t *testing.T) {
	body := []byte(`{
		"model":"m",
		"messages":[
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_a","type":"function","function":{"arguments":"{}"}}
			]},
			{"role":"tool","tool_call_id":"call_a","content":"r1"}
		]
	}`)

	msgs := sanitizeMessages(t, body, true)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message (assistant only), got %d", len(msgs))
	}
	first, _ := msgs[0].(map[string]any)
	if role, _ := first["role"].(string); role != "assistant" {
		t.Fatalf("should keep assistant message, got %v", first)
	}
	if _, hasToolCalls := first["tool_calls"]; hasToolCalls {
		t.Errorf("tool_calls key should be removed, got %v", first)
	}
	if content, _ := first["content"].(string); content != "" {
		t.Errorf("content should be filled with empty string, got %q", content)
	}
}

// TestSanitizeToolCallFunctionNames_OrphanToolMessageDropped 覆盖 id 对不上的形状：
// 被删的 tool_call id=call_a，紧随的 tool 消息 tool_call_id=call_z（客户端写回时
// 错位）。只按 id 相等配对时该孤儿消息会被留下，上游因「找不到对应 tool_call」400，
// 会话仍然卡死。删除作用域必须认「段内 tool 消息的 id 是否落在存活集合里」，
// 而不是只认 id 相等。
func TestSanitizeToolCallFunctionNames_OrphanToolMessageDropped(t *testing.T) {
	body := []byte(`{
		"model":"m",
		"messages":[
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_a","type":"function","function":{"arguments":"{}"}}
			]},
			{"role":"tool","tool_call_id":"call_z","content":"r1"},
			{"role":"user","content":"next"}
		]
	}`)

	msgs := sanitizeMessages(t, body, true)
	// 孤儿 tool 消息必须连带删掉：assistant(content 补空串) + user。
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (assistant + user), got %d: %v", len(msgs), msgs)
	}
	for _, raw := range msgs {
		m, _ := raw.(map[string]any)
		if role, _ := m["role"].(string); role == "tool" {
			t.Fatalf("orphan tool message with unmatched tool_call_id should be dropped, got %v", m)
		}
	}
}

// TestSanitizeToolCallFunctionNames_DuplicateIDAcrossRounds 覆盖跨轮复用同一 id：
// 脏轮 tool_call id=dup 被删，合法轮 assistant 的 tool_call 与 tool 消息复用同 id。
// 按全请求 id 扫描会把合法轮的 tool 消息一并删掉，合法轮 assistant 只剩无结果的
// tool_call → 下一轮上游 400，自愈反而制造新脏数据。删除必须按「被删 tool_call
// 所在 assistant 之后的连续 role:tool 段」作用域收敛。
func TestSanitizeToolCallFunctionNames_DuplicateIDAcrossRounds(t *testing.T) {
	body := []byte(`{
		"model":"m",
		"messages":[
			{"role":"assistant","content":"","tool_calls":[
				{"id":"dup","type":"function","function":{"arguments":"{}"}}
			]},
			{"role":"tool","tool_call_id":"dup","content":"r1"},
			{"role":"assistant","content":"","tool_calls":[
				{"id":"dup","type":"function","function":{"name":"ok_fn","arguments":"{}"}}
			]},
			{"role":"tool","tool_call_id":"dup","content":"r2"}
		]
	}`)

	msgs := sanitizeMessages(t, body, true)
	// 脏轮：assistant(content 补空串、tool_calls 移除) + tool 删除。
	// 合法轮：assistant(保留 tool_call) + tool 保留。
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (dirty assistant + valid assistant + valid tool), got %d: %v", len(msgs), msgs)
	}

	validAssistant, _ := msgs[1].(map[string]any)
	toolCalls, _ := validAssistant["tool_calls"].([]any)
	if len(toolCalls) != 1 {
		t.Fatalf("valid round tool_call should survive, got %d tool calls: %v", len(toolCalls), toolCalls)
	}
	validTool, _ := msgs[2].(map[string]any)
	if role, _ := validTool["role"].(string); role != "tool" {
		t.Fatalf("valid round tool message should survive, got %v", validTool)
	}
	if id, _ := validTool["tool_call_id"].(string); id != "dup" {
		t.Errorf("valid round tool message should keep its tool_call_id dup, got %q", id)
	}
}
