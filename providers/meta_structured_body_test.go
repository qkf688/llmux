package providers

import (
	"encoding/json"
	"testing"
)

// TestStructuredBodyOpenAIRes_MatchesOfficialFlatShape 锁住「协议检测请求体的
// text.format 形状」与官方 Responses 形状一致。
//
// 为什么要这条：检测请求体是**手写**的 JSON 字符串，而真实出站请求由
// service/responses 的 buildResponsesTextFormat 装配。两处手写同一语义的形状，
// 一旦漂移，检测结论与真实流量行为就会相反——曾出现检测用嵌套形
// （{"type":"json_schema","json_schema":{...}}，即 Chat 的 response_format 形状），
// 而真 OpenAI Responses 端点只认扁平形（name/schema/strict 与 type 平级），
// 结果是「端点明明支持结构化输出，检测却判不支持」。
//
// 判据打在 JSON 结构上：format 必须有扁平三键，且**不得**带嵌套 json_schema 子键。
func TestStructuredBodyOpenAIRes_MatchesOfficialFlatShape(t *testing.T) {
	m, ok := MetadataOf(TypeOpenAIRes)
	if !ok {
		t.Fatalf("MetadataOf(%q) not found", TypeOpenAIRes)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(m.StructuredBody, &body); err != nil {
		t.Fatalf("StructuredBody 不是合法 JSON: %v", err)
	}

	text, ok := body["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("StructuredBody 缺 text 对象: %#v", body)
	}
	format, ok := text["format"].(map[string]interface{})
	if !ok {
		t.Fatalf("StructuredBody 缺 text.format 对象: %#v", text)
	}

	if format["type"] != "json_schema" {
		t.Errorf("text.format.type = %v，期望 json_schema", format["type"])
	}
	if _, ok := format["name"]; !ok {
		t.Errorf("text.format 缺扁平的 name 键（官方形状要求与 type 平级）: %#v", format)
	}
	if _, ok := format["schema"]; !ok {
		t.Errorf("text.format 缺扁平的 schema 键（官方形状要求与 type 平级）: %#v", format)
	}
	if _, hasNested := format["json_schema"]; hasNested {
		t.Errorf("text.format 不应带嵌套 json_schema 子键（那是 Chat 的 response_format 形状，真 Responses 端点会拒）: %#v", format)
	}
}
