package streaming

import (
	"testing"
)

// 上游 usage 归一化：有序候选路径必须能吃下各家非标准写法，
// 且不会把「解析不出来」和「上游明确报 0」混在一起。
func TestUsageFromUpstreamMap_ToleratesNonStandardShapes(t *testing.T) {
	tests := []struct {
		name          string
		usage         map[string]interface{}
		wantPrompt    int64
		wantComplete  int64
		wantTotal     int64
		wantCached    int64
		wantReasoning int64
	}{
		{
			name: "openai chat 标准嵌套 details",
			usage: map[string]interface{}{
				"prompt_tokens":             float64(100),
				"completion_tokens":         float64(50),
				"total_tokens":              float64(150),
				"prompt_tokens_details":     map[string]interface{}{"cached_tokens": float64(64)},
				"completion_tokens_details": map[string]interface{}{"reasoning_tokens": float64(30)},
			},
			wantPrompt: 100, wantComplete: 50, wantTotal: 150, wantCached: 64, wantReasoning: 30,
		},
		{
			name: "openai-res 复数键名 details",
			usage: map[string]interface{}{
				"input_tokens":         float64(200),
				"output_tokens":        float64(80),
				"total_tokens":         float64(280),
				"input_tokens_details": map[string]interface{}{"cached_tokens": float64(128)},
				// 这里刻意用复数 output_tokens_details——单数曾是线格式 bug 的源头。
				"output_tokens_details": map[string]interface{}{"reasoning_tokens": float64(40)},
			},
			wantPrompt: 200, wantComplete: 80, wantTotal: 280, wantCached: 128, wantReasoning: 40,
		},
		{
			name: "reasoning_tokens 放在 usage 顶层（非标准供应商）",
			usage: map[string]interface{}{
				"prompt_tokens":     float64(10),
				"completion_tokens": float64(20),
				"reasoning_tokens":  float64(15),
			},
			wantPrompt: 10, wantComplete: 20, wantTotal: 30, wantReasoning: 15,
		},
		{
			name: "prompt_cache_hit_tokens 表示缓存命中（DeepSeek 风格）",
			usage: map[string]interface{}{
				"prompt_tokens":           float64(10),
				"completion_tokens":       float64(20),
				"prompt_cache_hit_tokens": float64(8),
			},
			wantPrompt: 10, wantComplete: 20, wantTotal: 30, wantCached: 8,
		},
		{
			name: "anthropic 原生：cache_read_input_tokens 即 cached_tokens",
			usage: map[string]interface{}{
				"input_tokens":            float64(300),
				"output_tokens":           float64(60),
				"cache_read_input_tokens": float64(256),
			},
			wantPrompt: 300, wantComplete: 60, wantTotal: 360, wantCached: 256,
		},
		{
			name: "total_tokens 缺失时回退为 prompt+completion",
			usage: map[string]interface{}{
				"prompt_tokens":     float64(7),
				"completion_tokens": float64(3),
			},
			wantPrompt: 7, wantComplete: 3, wantTotal: 10,
		},
		{
			name: "details 存在但为 0：不虚构数值",
			usage: map[string]interface{}{
				"prompt_tokens":             float64(5),
				"completion_tokens":         float64(5),
				"completion_tokens_details": map[string]interface{}{"reasoning_tokens": float64(0)},
			},
			wantPrompt: 5, wantComplete: 5, wantTotal: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usageFromUpstreamMap(tt.usage)
			if got.PromptTokens != tt.wantPrompt {
				t.Errorf("PromptTokens = %d, want %d", got.PromptTokens, tt.wantPrompt)
			}
			if got.CompletionTokens != tt.wantComplete {
				t.Errorf("CompletionTokens = %d, want %d", got.CompletionTokens, tt.wantComplete)
			}
			if got.TotalTokens != tt.wantTotal {
				t.Errorf("TotalTokens = %d, want %d", got.TotalTokens, tt.wantTotal)
			}
			if got.PromptTokensDetails.CachedTokens != tt.wantCached {
				t.Errorf("CachedTokens = %d, want %d", got.PromptTokensDetails.CachedTokens, tt.wantCached)
			}
			if got.CompletionTokensDetails.ReasoningTokens != tt.wantReasoning {
				t.Errorf("ReasoningTokens = %d, want %d", got.CompletionTokensDetails.ReasoningTokens, tt.wantReasoning)
			}
		})
	}
}

// 候选路径按优先级取值：标准位置有值时不应被后面的兼容项覆盖。
func TestPickUsageField_PrefersEarlierCandidate(t *testing.T) {
	usage := map[string]interface{}{
		"completion_tokens_details": map[string]interface{}{"reasoning_tokens": float64(99)},
		"reasoning_tokens":          float64(1),
	}
	if got := pickUsageField(usage, reasoningTokenPaths); got != 99 {
		t.Errorf("pickUsageField = %d, want 99 (standard nested path must win)", got)
	}
}

// 中间节点类型不符时不能 panic，直接落到下一个候选。
func TestPickUsageField_SkipsMalformedNode(t *testing.T) {
	usage := map[string]interface{}{
		"completion_tokens_details": "not-an-object",
		"reasoning_tokens":          float64(12),
	}
	if got := pickUsageField(usage, reasoningTokenPaths); got != 12 {
		t.Errorf("pickUsageField = %d, want 12 (must fall through malformed node)", got)
	}
}
