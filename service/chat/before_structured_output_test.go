package chat

import "testing"

// Anthropic 的结构化输出有两条路：原生 output_config.format（json_schema 约束）与
// 工具模拟（tools + tool_choice）。能力标记必须取并集——只认 toolCall 时，「只发
// format、不带 tools」的请求（Claude Code 的会话标题生成即此类）会被判无该能力，
// strict 能力匹配就不剔除「未开启 structured_output」的关联，请求带着 format 打到
// 不支持的端点后静默退化成散文。
func TestBeforerAnthropic_StructuredOutputCoversBothShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "原生 output_config.format（json_schema）",
			body: `{"model":"m","max_tokens":64,"messages":[{"role":"user","content":"hi"}],` +
				`"output_config":{"format":{"type":"json_schema","schema":{"type":"object"}}}}`,
			want: true,
		},
		{
			name: "工具模拟（tools + tool_choice 强制调用）",
			body: `{"model":"m","max_tokens":64,"messages":[{"role":"user","content":"hi"}],` +
				`"tools":[{"name":"t","input_schema":{"type":"object"}}],"tool_choice":{"type":"tool","name":"t"}}`,
			want: true,
		},
		{
			name: "两种形态都没有",
			body: `{"model":"m","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`,
			want: false,
		},
		{
			name: "format 存在但 type 不是 json_schema",
			body: `{"model":"m","max_tokens":64,"messages":[{"role":"user","content":"hi"}],` +
				`"output_config":{"format":{"type":"text"}}}`,
			want: false,
		},
		{
			// adaptive thinking 的 effort 也在 output_config 里，但它与输出格式无关。
			name: "只有 effort 的 output_config（不是结构化输出）",
			body: `{"model":"m","max_tokens":64,"messages":[{"role":"user","content":"hi"}],` +
				`"output_config":{"effort":"high"}}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, err := BeforerAnthropic([]byte(tt.body))
			if err != nil {
				t.Fatalf("BeforerAnthropic: %v", err)
			}
			if before.structuredOutput != tt.want {
				t.Fatalf("structuredOutput = %v, want %v（body=%s）", before.structuredOutput, tt.want, tt.body)
			}
		})
	}
}
