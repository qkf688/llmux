package responses

import (
	"encoding/json"
	"testing"
)

// 本文件覆盖 Responses 入站的两条形状缺陷：
//
//  1. text.format 的 json_schema：OpenAI 官方是**扁平**的
//     {"type":"json_schema","name":"n","schema":{...},"strict":true}
//     （openai-go SDK v2 的 responses.ResponseFormatTextJSONSchemaConfigParam 只有
//     Name / Schema / Strict / Type 四个平级字段，无嵌套 json_schema 键；
//     openai-python 的 ResponseFormatTextConfigParam 同款）。
//     本仓 DTO 只认嵌套形 → 官方扁平原入站后 schema 整段丢失（不报错、不留痕），
//     出站退化成 "response_format":{"type":"json_schema"} 这种上游拒的形状。
//
//  2. instructions 传数组形态：官方两个 SDK 都是纯字符串（Go: param.Opt[string]、
//     Python: Optional[str]），数组是非官方写法。本仓声明成 string → 数组入站
//     在 json.Unmarshal 阶段整条请求 400，连 model/input 都读不到。
//     非官方写法不该让整条请求不可用，宽容解析即可。

// TestTransformRequest_TextFormatFlatOfficialShape 断言官方扁平的 json_schema
// 入站后 schema 不丢。
func TestTransformRequest_TextFormatFlatOfficialShape(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":"hi","text":{"format":{"type":"json_schema",
		"name":"my_schema","schema":{"type":"object","properties":{"a":{"type":"string"}}},
		"strict":true}}}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if unified.ResponseFormat == nil {
		t.Fatal("response_format 整体丢失")
	}
	if unified.ResponseFormat.Type != "json_schema" {
		t.Errorf("response_format.type = %q，期望 json_schema", unified.ResponseFormat.Type)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(unified.ResponseFormat.JSONSchema, &schema); err != nil {
		t.Fatalf("json_schema 不是合法 JSON: %v（原文 %s）", err, unified.ResponseFormat.JSONSchema)
	}
	if schema["name"] != "my_schema" {
		t.Errorf("json_schema.name = %v，期望 my_schema", schema["name"])
	}
	if _, ok := schema["schema"]; !ok {
		t.Errorf("json_schema.schema 丢失: %#v", schema)
	}
	if schema["strict"] != true {
		t.Errorf("json_schema.strict = %v，期望 true", schema["strict"])
	}
}

// TestTransformRequest_TextFormatNestedStillWorks 断言嵌套形（Chat Completions 风格）
// 入站仍被识别——改成认官方扁平时不能把既有形状改坏。
func TestTransformRequest_TextFormatNestedStillWorks(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":"hi","text":{"format":{"type":"json_schema",
		"json_schema":{"name":"nested","schema":{"type":"object"},"strict":false}}}}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}

	if unified.ResponseFormat == nil {
		t.Fatal("response_format 整体丢失")
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(unified.ResponseFormat.JSONSchema, &schema); err != nil {
		t.Fatalf("json_schema 不是合法 JSON: %v", err)
	}
	if schema["name"] != "nested" {
		t.Errorf("json_schema.name = %v，期望 nested", schema["name"])
	}
}

// TestTransformRequest_TextFormatJSONObjectUnaffected 断言非 schema 的 format
// 类型不受影响（json_object / text）。
func TestTransformRequest_TextFormatJSONObjectUnaffected(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":"hi","text":{"format":{"type":"json_object"}}}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if unified.ResponseFormat == nil || unified.ResponseFormat.Type != "json_object" {
		t.Fatalf("json_object 形态应被保留: %#v", unified.ResponseFormat)
	}
}

// TestTransformFromUnified_TextFormatEmitsFlatShape 断言出站按官方扁平原 emit。
//
// 出站形状必须与官方一致：扁平 {"type":"json_schema","name":...,"schema":...}。
// 嵌套形（Chat 的 response_format.json_schema）发给真 OpenAI Responses 端点会被拒。
func TestTransformFromUnified_TextFormatEmitsFlatShape(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","input":"hi","text":{"format":{"type":"json_schema",
		"name":"my_schema","schema":{"type":"object"},"strict":true}}}`)

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

	text, ok := req["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("text 应为对象: %#v", req["text"])
	}
	format, ok := text["format"].(map[string]interface{})
	if !ok {
		t.Fatalf("text.format 应为对象: %#v", text["format"])
	}
	if format["type"] != "json_schema" {
		t.Errorf("text.format.type = %v，期望 json_schema", format["type"])
	}
	if format["name"] != "my_schema" {
		t.Errorf("text.format.name = %v，期望 my_schema", format["name"])
	}
	if _, ok := format["schema"]; !ok {
		t.Errorf("text.format.schema 丢失: %#v", format)
	}
	if _, hasNested := format["json_schema"]; hasNested {
		t.Errorf("text.format 不应带嵌套的 json_schema 键（官方是扁平的）: %#v", format)
	}
}

// TestTransformRequest_InstructionsArrayIsTolerated 断言 instructions 传数组形态
// 时整条请求不被 400，且其中的文本被提取进 System。
//
// 官方两个 SDK 的 instructions 都是纯字符串，数组是非官方写法——但非官方写法不该
// 让整条请求不可用（原来在 json.Unmarshal 阶段就失败，model/input 都读不到）。
func TestTransformRequest_InstructionsArrayIsTolerated(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","instructions":[{"type":"input_text","text":"你是助手"}],"input":"hi"}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("instructions 数组形态不应让整条请求失败: %v", err)
	}
	if unified.Model != "m" {
		t.Errorf("model = %q，期望 m", unified.Model)
	}
	if unified.System != "你是助手" {
		t.Errorf("system = %q，期望「你是助手」", unified.System)
	}
	if len(unified.Messages) != 1 {
		t.Errorf("input 应正常解析出 1 条消息，实际 %d 条", len(unified.Messages))
	}
}

// TestTransformRequest_InstructionsStringStillWorks 断言字符串形态不受影响。
func TestTransformRequest_InstructionsStringStillWorks(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"model":"m","instructions":"you are helpful","input":"hi"}`)

	unified, err := TransformRequest(raw, RequestTransformOptions{})
	if err != nil {
		t.Fatalf("TransformRequest failed: %v", err)
	}
	if unified.System != "you are helpful" {
		t.Errorf("system = %q，期望 you are helpful", unified.System)
	}
}
