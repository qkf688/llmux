package responses

import (
	"encoding/json"
	"testing"
)

// 本文件覆盖 Responses 入站的两条「静默丢弃」缺陷：
//
//  1. tool_choice 的官方扁平原 {"type":"function","name":"f"}：本仓 DTO 只认 Chat
//     Completions 的嵌套形 {"type":"function","function":{"name":"f"}}，扁平形入站后
//     name 落空 → unified.ToolChoice 两侧皆 nil → 「必须调用某工具」静默退化为
//     「不限制」。
//  2. function_call 缺 arguments / function_call_output 缺 output：这两个键在 Responses
//     协议里可省，缺省时整条 item 被跳过 → 历史回灌时调用与结果双双消失，工具配对断裂。
//
// 两条都是「客户端按官方文档写、llmux 静默失效」型故障，故断言打在 unified 与
// 出站 JSON 上。

// TestTransformRequest_ToolChoiceFlatOfficialShape 断言官方扁平的具名 tool_choice
// 入站后不丢 name。
func TestTransformRequest_ToolChoiceFlatOfficialShape(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":"hi","tools":[{"type":"function","name":"get_weather"}],
		"tool_choice":{"type":"function","name":"get_weather"}}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if unified.ToolChoice == nil {
		t.Fatal("tool_choice 整体丢失")
	}
	obj := unified.ToolChoice.ObjectValue
	if obj == nil {
		t.Fatalf("tool_choice 的对象形态丢失: %#v", unified.ToolChoice)
	}
	if obj.Type != "function" {
		t.Errorf("tool_choice.type = %q，期望 function", obj.Type)
	}
	if obj.Function == nil {
		t.Fatal("tool_choice 的 function 丢失")
	}
	if obj.Function.Name != "get_weather" {
		t.Errorf("tool_choice 的函数名 = %q，期望 get_weather", obj.Function.Name)
	}
}

// TestTransformRequest_ToolChoiceNestedStillWorks 断言嵌套形态（Chat Completions 风格）
// 入站仍被正确识别——修官方扁平日不能把既有形状改坏。
func TestTransformRequest_ToolChoiceNestedStillWorks(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":"hi","tool_choice":{"type":"function","function":{"name":"get_weather"}}}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	obj := unified.ToolChoice.ObjectValue
	if obj == nil || obj.Function == nil || obj.Function.Name != "get_weather" {
		t.Fatalf("嵌套形态的 tool_choice 应保留函数名，实际 %#v", unified.ToolChoice)
	}
}

// TestTransformRequest_ToolChoiceStringUnaffected 断言字符串形态不受影响。
func TestTransformRequest_ToolChoiceStringUnaffected(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":"hi","tool_choice":"required"}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if unified.ToolChoice == nil || unified.ToolChoice.StringValue == nil {
		t.Fatalf("字符串 tool_choice 应保留: %#v", unified.ToolChoice)
	}
	if *unified.ToolChoice.StringValue != "required" {
		t.Errorf("tool_choice = %q，期望 required", *unified.ToolChoice.StringValue)
	}
	if unified.ToolChoice.ObjectValue != nil {
		t.Errorf("字符串形态不应带 ObjectValue: %#v", unified.ToolChoice.ObjectValue)
	}
}

// TestTransformRequest_FunctionCallWithoutArgumentsIsKept 断言缺 arguments 的
// function_call 不被整条丢弃，且参数兜底为 "{}"（空参数对象的合法 JSON）。
//
// 为什么兜底而不是丢弃：Responses 协议里 arguments 可省（无参工具的常见写法），
// 丢一条 function_call 就破坏「调用↔结果」配对——结果消息前面没有对应的调用，
// 上游直接判 400，且错误离真因隔了两层。
func TestTransformRequest_FunctionCallWithoutArgumentsIsKept(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":[
		{"type":"function_call","call_id":"c1","name":"ping"}
	]}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if len(unified.Messages) != 1 {
		t.Fatalf("期望 1 条 assistant 消息，实际 %d 条: %#v", len(unified.Messages), unified.Messages)
	}
	assistant := unified.Messages[0]
	if assistant.Role != "assistant" {
		t.Fatalf("消息角色 = %q，期望 assistant", assistant.Role)
	}
	if len(assistant.ToolCalls) != 1 {
		t.Fatalf("期望 1 个 tool_call，实际 %d 个: %#v", len(assistant.ToolCalls), assistant.ToolCalls)
	}
	tc := assistant.ToolCalls[0]
	if tc.Function.Name != "ping" {
		t.Errorf("工具名 = %q，期望 ping", tc.Function.Name)
	}
	if tc.Function.Arguments != "{}" {
		t.Errorf("缺省 arguments 应兜底为 %q，实际 %q", "{}", tc.Function.Arguments)
	}
}

// TestTransformRequest_FunctionCallOutputWithoutOutputIsKept 断言缺 output 的
// function_call_output 不被整条丢弃，且内容兜底为空串。
func TestTransformRequest_FunctionCallOutputWithoutOutputIsKept(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":[
		{"type":"function_call","call_id":"c1","name":"ping","arguments":"{}"},
		{"type":"function_call_output","call_id":"c1"}
	]}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if len(unified.Messages) != 2 {
		t.Fatalf("期望 2 条消息（assistant + tool），实际 %d 条: %#v", len(unified.Messages), unified.Messages)
	}
	toolMsg := unified.Messages[1]
	if toolMsg.Role != "tool" {
		t.Fatalf("第二条消息角色 = %q，期望 tool", toolMsg.Role)
	}
	if toolMsg.ToolCallID != "c1" {
		t.Errorf("tool_call_id = %q，期望 c1", toolMsg.ToolCallID)
	}
	content, _ := toolMsg.Content.(string)
	if content != "" {
		t.Errorf("缺省 output 应兜底为空串，实际 %q", content)
	}
}

// TestTransformRequest_MissingArgumentsAndOutputKeepPairing 断言「调用缺参数 + 结果缺
// 内容」这一组合下配对仍完整：assistant 带 tool_calls、tool 消息紧随其后。
//
// 这是本次修复的核心判据：两条守卫各自独立修不完整，只要有一侧仍丢弃，配对就断。
func TestTransformRequest_MissingArgumentsAndOutputKeepPairing(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":[
		{"type":"function_call","call_id":"c1","name":"ping"},
		{"type":"function_call_output","call_id":"c1"}
	]}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if len(unified.Messages) != 2 {
		t.Fatalf("期望 2 条消息，实际 %d 条: %#v", len(unified.Messages), unified.Messages)
	}
	if len(unified.Messages[0].ToolCalls) != 1 {
		t.Fatalf("assistant 应带 1 个 tool_call，实际 %#v", unified.Messages[0].ToolCalls)
	}
	if unified.Messages[1].Role != "tool" {
		t.Fatalf("第二条应为 tool 消息，实际角色 %q", unified.Messages[1].Role)
	}
	if unified.Messages[1].ToolCallID != unified.Messages[0].ToolCalls[0].ID {
		t.Errorf("tool 消息的 tool_call_id=%q 与调用的 id=%q 不配对",
			unified.Messages[1].ToolCallID, unified.Messages[0].ToolCalls[0].ID)
	}
}

// TestTransformFromUnified_ToolChoiceEmitsFlatShape 断言出站按官方扁平形态 emit。
//
// 出站形状必须与本仓入站能解的形状一致：若出站仍发嵌套形，则「同一份请求回灌」
// 这条路径上 tool_choice 会在自己家门口丢一次。
func TestTransformFromUnified_ToolChoiceEmitsFlatShape(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":"hi","tools":[{"type":"function","name":"get_weather"}],
		"tool_choice":{"type":"function","name":"get_weather"}}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	out, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified failed: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatalf("出站 JSON 解析失败: %v", err)
	}

	tc, ok := req["tool_choice"].(map[string]interface{})
	if !ok {
		t.Fatalf("tool_choice 应为对象: %#v", req["tool_choice"])
	}
	if tc["type"] != "function" {
		t.Errorf("tool_choice.type = %v，期望 function", tc["type"])
	}
	if tc["name"] != "get_weather" {
		t.Errorf("tool_choice.name = %v，期望 get_weather（官方扁平形态）", tc["name"])
	}
	if _, hasNested := tc["function"]; hasNested {
		t.Errorf("tool_choice 不应带嵌套的 function 键: %#v", tc)
	}
}
