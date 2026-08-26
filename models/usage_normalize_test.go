package models

import (
	"testing"
)

// 上游 usage 归一化：有序候选路径必须能吃下各家非标准写法，
// 且不会把「解析不出来」和「上游明确报 0」混在一起。
func TestUsageFromMap_ToleratesNonStandardShapes(t *testing.T) {
	tests := []struct {
		name               string
		usage              map[string]interface{}
		wantPrompt         int64
		wantComplete       int64
		wantTotal          int64
		wantCached         int64
		wantReasoning      int64
		wantReasoningKnown bool
		wantPromptAudio    int64
		wantCompleteAudio  int64
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
			wantPrompt: 100, wantComplete: 50, wantTotal: 150, wantCached: 64, wantReasoning: 30, wantReasoningKnown: true,
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
			wantPrompt: 200, wantComplete: 80, wantTotal: 280, wantCached: 128, wantReasoning: 40, wantReasoningKnown: true,
		},
		{
			name: "reasoning_tokens 放在 usage 顶层（非标准供应商）",
			usage: map[string]interface{}{
				"prompt_tokens":     float64(10),
				"completion_tokens": float64(20),
				"reasoning_tokens":  float64(15),
			},
			wantPrompt: 10, wantComplete: 20, wantTotal: 30, wantReasoning: 15, wantReasoningKnown: true,
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
			// kimi 一类供应商走 anthropic 协议时只回填 openai 兼容字段，
			// 不给 anthropic 原生 input_tokens / output_tokens。
			name: "anthropic 协议但只有 openai 兼容字段",
			usage: map[string]interface{}{
				"prompt_tokens":     float64(11),
				"completion_tokens": float64(22),
				"cached_tokens":     float64(9),
			},
			wantPrompt: 11, wantComplete: 22, wantTotal: 33, wantCached: 9,
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
			// 原生与兼容并存（kimi 一类混合返回）：兼容位常是垃圾值，原生必须胜出。
			name: "原生与顶层兼容字段并存：原生优先",
			usage: map[string]interface{}{
				"input_tokens":            float64(200),
				"output_tokens":           float64(40),
				"cache_read_input_tokens": float64(32),
				"prompt_tokens":           float64(1),
				"completion_tokens":       float64(1),
				"cached_tokens":           float64(1),
			},
			wantPrompt: 200, wantComplete: 40, wantTotal: 240, wantCached: 32,
		},
		{
			// 上游明确报了 0 的拆分：known=true 区分于「没报」——非推理模型真实报 0，
			// 前端据此显示 0 而不是「未知」。
			name: "details 存在但为 0：不虚构数值，但 known 为 true",
			usage: map[string]interface{}{
				"prompt_tokens":             float64(5),
				"completion_tokens":         float64(5),
				"completion_tokens_details": map[string]interface{}{"reasoning_tokens": float64(0)},
			},
			wantPrompt: 5, wantComplete: 5, wantTotal: 10, wantReasoningKnown: true,
		},
		{
			// audio 明细：旧 openai processer 靠 json tag 能读到，收敛后必须由候选表补齐。
			name: "openai 音频明细：prompt/completion 两侧 audio_tokens",
			usage: map[string]interface{}{
				"prompt_tokens":             float64(50),
				"completion_tokens":         float64(20),
				"total_tokens":              float64(70),
				"prompt_tokens_details":     map[string]interface{}{"audio_tokens": float64(12)},
				"completion_tokens_details": map[string]interface{}{"audio_tokens": float64(6)},
			},
			wantPrompt: 50, wantComplete: 20, wantTotal: 70, wantPromptAudio: 12, wantCompleteAudio: 6,
		},
		{
			name: "openai-res 复数键名 audio 明细",
			usage: map[string]interface{}{
				"input_tokens":          float64(50),
				"output_tokens":         float64(20),
				"total_tokens":          float64(70),
				"input_tokens_details":  map[string]interface{}{"audio_tokens": float64(4)},
				"output_tokens_details": map[string]interface{}{"audio_tokens": float64(2)},
			},
			wantPrompt: 50, wantComplete: 20, wantTotal: 70, wantPromptAudio: 4, wantCompleteAudio: 2,
		},
		{
			name:  "空 usage：全零，不 panic",
			usage: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UsageFromMap(tt.usage)
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
			if got.CompletionTokensDetails.ReasoningTokensKnown != tt.wantReasoningKnown {
				t.Errorf("ReasoningTokensKnown = %v, want %v", got.CompletionTokensDetails.ReasoningTokensKnown, tt.wantReasoningKnown)
			}
			if got.PromptTokensDetails.AudioTokens != tt.wantPromptAudio {
				t.Errorf("PromptTokensDetails.AudioTokens = %d, want %d", got.PromptTokensDetails.AudioTokens, tt.wantPromptAudio)
			}
			if got.CompletionTokensDetails.AudioTokens != tt.wantCompleteAudio {
				t.Errorf("CompletionTokensDetails.AudioTokens = %d, want %d", got.CompletionTokensDetails.AudioTokens, tt.wantCompleteAudio)
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

// known 标记：键存在即 true（哪怕值为 0，或首个候选 malformed 后由下一个候选兜住），
// 全部缺失才 false——「上游没报」与「上游明确报 0」的区分点。
func TestUsageFieldPresent_ExistsVsMissing(t *testing.T) {
	tests := []struct {
		name  string
		usage map[string]interface{}
		want  bool
	}{
		{
			name:  "嵌套键存在但值为 0",
			usage: map[string]interface{}{"completion_tokens_details": map[string]interface{}{"reasoning_tokens": float64(0)}},
			want:  true,
		},
		{
			name: "首候选 malformed，次候选键存在",
			usage: map[string]interface{}{
				"completion_tokens_details": "not-an-object",
				"reasoning_tokens":          float64(0),
			},
			want: true,
		},
		{
			name:  "键全部缺失",
			usage: map[string]interface{}{"prompt_tokens": float64(1)},
			want:  false,
		},
		{
			name:  "空 usage",
			usage: map[string]interface{}{},
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usageFieldPresent(tt.usage, reasoningTokenPaths); got != tt.want {
				t.Errorf("usageFieldPresent = %v, want %v", got, tt.want)
			}
		})
	}
}

// total 口径：上游给了 total 就用上游的，没给才回退 prompt+completion。
func TestResolveTotalTokens(t *testing.T) {
	if got := ResolveTotalTokens(10, 5, 99); got != 99 {
		t.Errorf("ResolveTotalTokens(10,5,99) = %d, want 99 (upstream total wins)", got)
	}
	if got := ResolveTotalTokens(10, 5, 0); got != 15 {
		t.Errorf("ResolveTotalTokens(10,5,0) = %d, want 15 (fallback)", got)
	}
}
