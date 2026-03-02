package anthropic

import (
	"encoding/json"
	"testing"
)

func TestTransformToUnified(t *testing.T) {
	raw := []byte(`{
		"model":"claude-3-5-sonnet",
		"stream":true,
		"system":[
			{"type":"text","text":"rule-1"},
			{"type":"text","content":"rule-2"}
		],
		"messages":[
			{
				"role":"assistant",
				"content":[{"type":"tool_use","id":"tool_1","name":"calc","input":{"x":1}}],
				"cache_control":{"type":"ephemeral"}
			},
			{
				"role":"user",
				"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"done"}]
			}
		],
		"tools":[
			{
				"name":"calc",
				"description":"do math",
				"input_schema":{"type":"object"},
				"cache_control":{"type":"ephemeral"}
			}
		],
		"tool_choice":{"type":"tool","name":"calc"},
		"stop_sequences":["a","b"],
		"metadata":{"trace_id":"abc"},
		"thinking":{"type":"enabled","budget_tokens":30000}
	}`)

	unified, err := TransformToUnified(raw)
	if err != nil {
		t.Fatalf("TransformToUnified returned error: %v", err)
	}

	if unified.Model != "claude-3-5-sonnet" {
		t.Fatalf("unexpected model: %s", unified.Model)
	}
	if !unified.Stream {
		t.Fatal("expected stream=true")
	}
	if unified.System != "rule-1\nrule-2" {
		t.Fatalf("unexpected system: %q", unified.System)
	}
	if len(unified.SystemParts) != 2 {
		t.Fatalf("expected 2 system parts, got %d", len(unified.SystemParts))
	}
	if unified.SystemParts[0].Type != "text" || unified.SystemParts[0].Text == nil || *unified.SystemParts[0].Text != "rule-1" {
		t.Fatalf("unexpected system part[0]: %+v", unified.SystemParts[0])
	}
	if unified.SystemParts[1].Type != "text" || unified.SystemParts[1].Text == nil || *unified.SystemParts[1].Text != "rule-2" {
		t.Fatalf("unexpected system part[1]: %+v", unified.SystemParts[1])
	}
	if len(unified.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(unified.Messages))
	}
	if len(unified.Messages[0].ToolCalls) != 1 {
		t.Fatalf("expected tool call parsed from content, got %d", len(unified.Messages[0].ToolCalls))
	}
	if unified.Messages[1].ToolCallID != "tool_1" {
		t.Fatalf("expected tool_call_id=tool_1, got %q", unified.Messages[1].ToolCallID)
	}
	if unified.Messages[0].CacheControl == nil || unified.Messages[0].CacheControl.Type != "ephemeral" {
		t.Fatal("expected message cache_control to be parsed")
	}
	if len(unified.Tools) != 1 || unified.Tools[0].CacheControl == nil || unified.Tools[0].CacheControl.Type != "ephemeral" {
		t.Fatal("expected tool cache_control to be parsed")
	}
	if unified.Stop == nil || len(unified.Stop.Multiple) != 2 {
		t.Fatal("expected stop_sequences to be mapped")
	}
	if unified.Metadata["trace_id"] != "abc" {
		t.Fatalf("unexpected metadata: %+v", unified.Metadata)
	}
	if unified.ReasoningEffort == nil || *unified.ReasoningEffort != "medium" {
		t.Fatalf("unexpected reasoning_effort: %v", unified.ReasoningEffort)
	}
	if unified.ReasoningBudget == nil || *unified.ReasoningBudget != 30000 {
		t.Fatalf("unexpected reasoning_budget: %v", unified.ReasoningBudget)
	}
	if unified.ToolChoice == nil || unified.ToolChoice.ObjectValue == nil || unified.ToolChoice.ObjectValue.Function == nil || unified.ToolChoice.ObjectValue.Function.Name != "calc" {
		t.Fatalf("unexpected tool_choice: %+v", unified.ToolChoice)
	}
}

func TestTransformRoundTripPreservesSystemArray(t *testing.T) {
	raw := []byte(`{
		"model":"claude-3-5-sonnet",
		"system":[
			{"type":"text","text":"rule-1"},
			{"type":"text","content":"rule-2"}
		],
		"messages":[{"role":"user","content":"hi"}]
	}`)

	unified, err := TransformToUnified(raw)
	if err != nil {
		t.Fatalf("TransformToUnified returned error: %v", err)
	}

	body, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified returned error: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}

	system, ok := req["system"].([]interface{})
	if !ok {
		t.Fatalf("expected system to be array, got %T", req["system"])
	}
	if len(system) != 2 {
		t.Fatalf("expected 2 system blocks, got %d", len(system))
	}

	block0 := system[0].(map[string]interface{})
	if block0["type"] != "text" || block0["text"] != "rule-1" {
		t.Fatalf("unexpected system[0]: %+v", block0)
	}
	block1 := system[1].(map[string]interface{})
	if block1["type"] != "text" || block1["text"] != "rule-2" {
		t.Fatalf("unexpected system[1]: %+v", block1)
	}
}

func TestTransformFromUnified(t *testing.T) {
	stop := "done"
	effort := "high"

	unified := &UnifiedRequest{
		Model:  "claude-3-5-sonnet",
		Stream: true,
		Messages: []UnifiedMessage{
			{Role: "system", Content: "follow rules"},
			{Role: "user", Content: "hello"},
			{
				Role:    "assistant",
				Content: "calling tool",
				ToolCalls: []UnifiedToolCall{
					{
						ID:   "tool_1",
						Type: "function",
						Function: UnifiedToolCallFunction{
							Name:      "calc",
							Arguments: `{"x":1}`,
						},
					},
				},
			},
			{Role: "tool", ToolCallID: "tool_1", Content: "42"},
		},
		Stop:            &UnifiedStop{Single: &stop},
		ReasoningEffort: &effort,
	}

	body, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified returned error: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}

	if req["max_tokens"].(float64) != 8192 {
		t.Fatalf("expected default max_tokens=8192, got %v", req["max_tokens"])
	}
	if req["system"].(string) != "follow rules" {
		t.Fatalf("expected extracted system message, got %v", req["system"])
	}

	messages, ok := req["messages"].([]interface{})
	if !ok || len(messages) != 3 {
		t.Fatalf("expected 3 non-system messages, got %d", len(messages))
	}

	toolResultMsg := messages[2].(map[string]interface{})
	if toolResultMsg["role"] != "user" {
		t.Fatalf("expected tool role converted to user, got %v", toolResultMsg["role"])
	}

	thinking, ok := req["thinking"].(map[string]interface{})
	if !ok {
		t.Fatal("expected thinking field")
	}
	if thinking["budget_tokens"].(float64) != 50000 {
		t.Fatalf("expected high effort => 50000 budget, got %v", thinking["budget_tokens"])
	}
}

func TestParseResponse(t *testing.T) {
	raw := []byte(`{
		"id":"msg_1",
		"model":"claude-3-5-sonnet",
		"stop_reason":"end_turn",
		"content":[
			{"type":"text","text":"hello"},
			{"type":"tool_use","id":"tool_1","name":"calc","input":{"x":1}}
		],
		"usage":{"input_tokens":10,"output_tokens":20}
	}`)

	unified, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("ParseResponse returned error: %v", err)
	}

	if len(unified.Choices) != 1 {
		t.Fatalf("expected one choice, got %d", len(unified.Choices))
	}
	if unified.Choices[0].FinishReason != "stop" {
		t.Fatalf("expected finish_reason=stop, got %s", unified.Choices[0].FinishReason)
	}
	if unified.Choices[0].Message == nil || unified.Choices[0].Message.Content != "hello" {
		t.Fatalf("unexpected message content: %+v", unified.Choices[0].Message)
	}
	if len(unified.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(unified.Choices[0].Message.ToolCalls))
	}
	if unified.Usage == nil || unified.Usage.TotalTokens != 30 {
		t.Fatalf("unexpected usage: %+v", unified.Usage)
	}
}

func TestFormatResponse(t *testing.T) {
	unified := &UnifiedResponse{
		ID:    "msg_1",
		Model: "claude-3-5-sonnet",
		Choices: []UnifiedChoice{
			{
				Index: 0,
				Message: &UnifiedMessage{
					Role:    "assistant",
					Content: "hello",
					ToolCalls: []UnifiedToolCall{
						{
							ID:   "tool_1",
							Type: "function",
							Function: UnifiedToolCallFunction{
								Name:      "calc",
								Arguments: `{"x":1}`,
							},
						},
					},
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
		},
	}

	body, err := FormatResponse(unified)
	if err != nil {
		t.Fatalf("FormatResponse returned error: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal result failed: %v", err)
	}

	if resp["stop_reason"] != "tool_use" {
		t.Fatalf("expected stop_reason=tool_use, got %v", resp["stop_reason"])
	}

	content, ok := resp["content"].([]interface{})
	if !ok || len(content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(content))
	}

	usage, ok := resp["usage"].(map[string]interface{})
	if !ok {
		t.Fatal("expected usage field")
	}
	if usage["input_tokens"].(float64) != 10 || usage["output_tokens"].(float64) != 20 {
		t.Fatalf("unexpected usage values: %+v", usage)
	}
}
