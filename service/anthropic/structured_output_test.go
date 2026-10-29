package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qkf688/llmux/models"
)

// Anthropic 的结构化输出走 output_config.format，而统一模型承载的是 Chat 的内嵌形状
// （response_format.json_schema = {name,schema,strict}）。入站此前只认 output_config.effort，
// format 整段被静默丢弃——客户端（实测 Claude Code 的会话标题生成）要求 json_schema
// 约束，上游却收到无 response_format 的请求，回来的是散文而非 {"title":...}。
func TestTransformToUnified_OutputConfigFormatBecomesResponseFormat(t *testing.T) {
	raw := []byte(`{
		"model": "m", "max_tokens": 1024,
		"messages": [{"role":"user","content":"hi"}],
		"output_config": {
			"effort": "high",
			"format": {"type":"json_schema","schema":{"type":"object","properties":{"title":{"type":"string"}},"required":["title"]}}
		}
	}`)

	unified, err := TransformToUnified(context.Background(), raw)
	if err != nil {
		t.Fatalf("TransformToUnified 失败: %v", err)
	}
	if unified.ResponseFormat == nil {
		t.Fatal("output_config.format 丢失：统一模型 ResponseFormat 为 nil")
	}
	if unified.ResponseFormat.Type != "json_schema" {
		t.Fatalf("ResponseFormat.Type = %q，期望 json_schema", unified.ResponseFormat.Type)
	}

	var embedded struct {
		Name   string          `json:"name"`
		Schema json.RawMessage `json:"schema"`
	}
	if err := json.Unmarshal(unified.ResponseFormat.JSONSchema, &embedded); err != nil {
		t.Fatalf("JSONSchema 不是内嵌对象形状: %v（原文 %s）", err, unified.ResponseFormat.JSONSchema)
	}
	// Anthropic 的 format 没有 name 字段，但 Chat / Responses 出站都要求它非空。
	if embedded.Name == "" {
		t.Fatalf("内嵌对象的 name 不能为空（Chat/Responses 出站要求非空）: %s", unified.ResponseFormat.JSONSchema)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(embedded.Schema, &schema); err != nil {
		t.Fatalf("schema 未原样带走: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("schema 内容被改写: %v", schema)
	}
	if _, ok := schema["required"]; !ok {
		t.Fatalf("schema 字段丢失（required）: %v", schema)
	}

	// effort 与 format 同存时互不干扰：effort 仍走 reasoning 档位。
	if unified.ReasoningEffort == nil || *unified.ReasoningEffort != "high" {
		t.Fatalf("output_config.effort 应仍解析为 high，实际 %v", unified.ReasoningEffort)
	}
}

func TestTransformToUnified_OutputConfigFormatRobustness(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"format 非对象", `{"model":"m","max_tokens":8,"messages":[{"role":"user","content":"hi"}],"output_config":{"format":"json_schema"}}`},
		{"type 不是 json_schema", `{"model":"m","max_tokens":8,"messages":[{"role":"user","content":"hi"}],"output_config":{"format":{"type":"json_object","schema":{"type":"object"}}}}`},
		{"缺 schema", `{"model":"m","max_tokens":8,"messages":[{"role":"user","content":"hi"}],"output_config":{"format":{"type":"json_schema"}}}`},
		{"format 为 null", `{"model":"m","max_tokens":8,"messages":[{"role":"user","content":"hi"}],"output_config":{"format":null}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := TransformToUnified(context.Background(), []byte(tt.body))
			if err != nil {
				t.Fatalf("不应整条请求 400: %v", err)
			}
			if unified.ResponseFormat != nil {
				t.Fatalf("该形态不应产出 ResponseFormat，实际 %+v", unified.ResponseFormat)
			}
		})
	}
}

// 反向：统一模型 → Anthropic 请求必须写回 output_config.format（name / strict 在该协议
// 无位置，只带 type 与 schema）。缺了它，OpenAI/Responses 客户端要求的结构化输出
// 打到 Anthropic 上游会退化成自由文本。
func TestTransformFromUnified_ResponseFormatBecomesOutputConfig(t *testing.T) {
	unified := &models.UnifiedRequest{
		Model:     "claude-x",
		MaxTokens: 1024,
		Messages: []models.UnifiedMessage{
			{Role: "user", Content: "hi"},
		},
		ResponseFormat: &models.UnifiedResponseFormat{
			Type:       "json_schema",
			JSONSchema: json.RawMessage(`{"name":"title_out","strict":true,"schema":{"type":"object","properties":{"title":{"type":"string"}}}}`),
		},
	}

	body, err := TransformFromUnified(unified)
	if err != nil {
		t.Fatalf("TransformFromUnified 失败: %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal 失败: %v", err)
	}
	outputConfig, ok := req["output_config"].(map[string]interface{})
	if !ok {
		t.Fatalf("缺少 output_config: %s", body)
	}
	format, ok := outputConfig["format"].(map[string]interface{})
	if !ok {
		t.Fatalf("缺少 output_config.format: %s", body)
	}
	if format["type"] != "json_schema" {
		t.Fatalf("format.type = %v，期望 json_schema", format["type"])
	}
	schema, ok := format["schema"].(map[string]interface{})
	if !ok {
		t.Fatalf("format.schema 缺失或非对象: %v", format)
	}
	if schema["type"] != "object" {
		t.Fatalf("schema 内容不符: %v", schema)
	}
	// name / strict 是 Chat 的形状，Anthropic 不认这两个键。
	for _, banned := range []string{"name", "strict"} {
		if _, ok := format[banned]; ok {
			t.Fatalf("Anthropic 的 format 不应带 %q: %v", banned, format)
		}
	}
	// effort 未设置时不应凭空产出 effort 键（保留原语义：不设置 = 不约束）。
	if _, ok := outputConfig["effort"]; ok {
		t.Fatalf("未设置 effort 时不应写出该键: %v", outputConfig)
	}
}

// 非 json_schema 的输出格式在 Anthropic 无对应语义，不得伪造成 json_schema
// （带 type 无 schema 的 format 会被上游 400）。
func TestTransformFromUnified_NonJSONSchemaFormatOmitted(t *testing.T) {
	for _, typ := range []string{"json_object", "text"} {
		unified := &models.UnifiedRequest{
			Model:          "claude-x",
			MaxTokens:      64,
			Messages:       []models.UnifiedMessage{{Role: "user", Content: "hi"}},
			ResponseFormat: &models.UnifiedResponseFormat{Type: typ},
		}
		body, err := TransformFromUnified(unified)
		if err != nil {
			t.Fatalf("TransformFromUnified(%s) 失败: %v", typ, err)
		}
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal 失败: %v", err)
		}
		if _, ok := req["output_config"]; ok {
			t.Fatalf("%s 无 Anthropic 对应格式，不应写出 output_config: %s", typ, body)
		}
	}
}
