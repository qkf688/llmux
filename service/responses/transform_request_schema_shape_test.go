package responses

import (
	"encoding/json"
	"testing"
)

// 本文件覆盖「schema 不是 object」的两种合法形态：布尔与数组。
//
// JSON Schema 规范允许根 schema 是布尔（`true` 表示任意值、`false` 表示不允许任何值），
// 数组则是常见的文档写法。把扁平形的 schema 声明成 map[string]interface{} 会让这类
// 请求在 json.Unmarshal 阶段整条 400——而改动前扁平 schema 键根本不被解析，
// 请求是能过的。本次为支持扁平形而新增字段，却把「原本能过」的请求变窄了。
//
// 出站侧同样：拆平内嵌对象时若只认 map，非 object schema 会被静默丢弃，
// 而注释承诺的「拆不出来就整段回退」因 name 非空而不触发。

// TestTransformRequest_TextFormatNonObjectSchemaIsTolerated 断言 schema 为布尔或数组时
// 整条请求不被 400，且 schema 内容被保留。
func TestTransformRequest_TextFormatNonObjectSchemaIsTolerated(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
	}{
		{name: "boolean_true", raw: `{"model":"m","input":"hi","text":{"format":{"type":"json_schema","name":"b","schema":true}}}`},
		{name: "boolean_false", raw: `{"model":"m","input":"hi","text":{"format":{"type":"json_schema","name":"b","schema":false}}}`},
		{name: "array", raw: `{"model":"m","input":"hi","text":{"format":{"type":"json_schema","name":"a","schema":[{"type":"object"}]}}}`},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			unified, err := TransformRequest([]byte(c.raw), RequestTransformOptions{})
			if err != nil {
				t.Fatalf("schema 为非 object 的合法形态不应让整条请求失败: %v", err)
			}
			if unified.ResponseFormat == nil {
				t.Fatal("response_format 整体丢失")
			}

			var schema map[string]interface{}
			if err := json.Unmarshal(unified.ResponseFormat.JSONSchema, &schema); err != nil {
				t.Fatalf("归一后的 json_schema 不是合法 JSON: %v（原文 %s）",
					err, unified.ResponseFormat.JSONSchema)
			}
			if schema["name"] != "b" && schema["name"] != "a" {
				t.Errorf("json_schema.name 丢失: %#v", schema)
			}
			if _, ok := schema["schema"]; !ok {
				t.Errorf("json_schema.schema 丢失: %#v", schema)
			}
		})
	}
}

// TestTransformFromUnified_NonObjectSchemaSurvives 断言出站拆平时非 object 的 schema 不被丢弃。
//
// 判据打在最终 JSON 上：`text.format.schema` 键必须存在且值与原样一致。
func TestTransformFromUnified_NonObjectSchemaSurvives(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		raw        string
		wantSchema string
	}{
		{name: "boolean_true", raw: `{"model":"m","input":"hi","text":{"format":{"type":"json_schema","name":"b","schema":true}}}`, wantSchema: `true`},
		{name: "array", raw: `{"model":"m","input":"hi","text":{"format":{"type":"json_schema","name":"a","schema":[{"type":"object"}]}}}`, wantSchema: `[{"type":"object"}]`},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			unified, err := TransformRequest([]byte(c.raw), RequestTransformOptions{})
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

			format, ok := nestedMapNoFatal(req, "text", "format")
			if !ok {
				t.Fatalf("出站缺 text.format: %s", string(out))
			}
			schemaRaw, ok := format["schema"]
			if !ok {
				t.Fatalf("出站 text.format.schema 丢失（注释承诺「拆不出来整段回退」）: %s", string(out))
			}

			got, err := json.Marshal(schemaRaw)
			if err != nil {
				t.Fatalf("schema 序列化失败: %v", err)
			}
			var normWant interface{}
			if err := json.Unmarshal([]byte(c.wantSchema), &normWant); err != nil {
				t.Fatalf("期望值解析失败: %v", err)
			}
			normWantBytes, _ := json.Marshal(normWant)
			if string(got) != string(normWantBytes) {
				t.Errorf("出站 schema = %s，期望 %s", got, normWantBytes)
			}
		})
	}
}

// nestedMapNoFatal 沿 keys 逐层取嵌套对象，任一层缺失返回 false（不 Fatal）。
func nestedMapNoFatal(m map[string]interface{}, keys ...string) (map[string]interface{}, bool) {
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}
