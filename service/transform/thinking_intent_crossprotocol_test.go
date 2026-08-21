package transform

import (
	"encoding/json"
	"testing"

	"github.com/qkf688/llmux/service/responses"
	"github.com/qkf688/llmux/service/transform/openai"
)

// TestResponsesBudgetOnly_ToOpenAIBodyKeepsThinkingIntent 是**跨协议端到端**回归：
// Responses 客户端只给 reasoning.max_tokens（不给 effort）→ 出站到 OpenAI 兼容上游时，
// body 里必须仍有 thinking 意图（reasoning_effort）。
//
// 为什么要打在 body 上而不只断 unified 字段：openai 出站的 emit 条件是
// `unified.ReasoningEffort != nil`（request.go 的 reasoning_effort 段），
// 只断 unified 会漏掉「字段填了但出站没 emit」这类断层。这是本改动**唯一**真正修复的路径
// ——anthropic 入站早已反推 effort，openai 入站协议本身没有 budget 字段，
// 只有 responses→OpenAI 兼容上游这一条会静默丢思考意图。
func TestResponsesBudgetOnly_ToOpenAIBodyKeepsThinkingIntent(t *testing.T) {
	raw := []byte(`{"model":"gpt-5","input":"hi","reasoning":{"max_tokens":20000}}`)

	unified, err := responses.TransformRequest(raw, responses.RequestTransformOptions{})
	if err != nil {
		t.Fatalf("responses.TransformRequest failed: %v", err)
	}

	out, err := openai.FromUnified(unified)
	if err != nil {
		t.Fatalf("openai.FromUnified failed: %v", err)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(out, &body); err != nil {
		t.Fatalf("failed to unmarshal openai body: %v", err)
	}

	effortRaw, ok := body["reasoning_effort"]
	if !ok {
		t.Fatalf("reasoning_effort 键缺失，thinking 意图整体丢失（上游会按不思考处理）: %s", out)
	}
	var effort string
	if err := json.Unmarshal(effortRaw, &effort); err != nil {
		t.Fatalf("reasoning_effort 不是字符串: %s", effortRaw)
	}
	if effort != "medium" {
		t.Errorf("expected reasoning_effort=medium（20000 的反推档位）, got %q", effort)
	}
}
