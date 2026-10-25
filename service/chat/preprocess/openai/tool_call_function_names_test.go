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

	patched, changed, err := SanitizeToolCallFunctionNames(body)
	if err != nil || !changed {
		t.Fatalf("sanitize should change the body, got changed=%v err=%v", changed, err)
	}

	var req map[string]any
	if err := json.Unmarshal(patched, &req); err != nil {
		t.Fatalf("patched body is not valid JSON: %v", err)
	}

	msgs, _ := req["messages"].([]any)
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

	patched, changed, err := SanitizeToolCallFunctionNames(body)
	if err != nil || !changed {
		t.Fatalf("sanitize should change the body, got changed=%v err=%v", changed, err)
	}

	var req map[string]any
	if err := json.Unmarshal(patched, &req); err != nil {
		t.Fatalf("patched body is not valid JSON: %v", err)
	}

	msgs, _ := req["messages"].([]any)
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

	patched, changed, err := SanitizeToolCallFunctionNames(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("all-dirty request should still be patched (assistant message kept)")
	}

	var req map[string]any
	if err := json.Unmarshal(patched, &req); err != nil {
		t.Fatalf("patched body is not valid JSON: %v", err)
	}

	msgs, _ := req["messages"].([]any)
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
