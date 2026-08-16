package chat

import (
	"context"
	"strings"
	"testing"
	"time"
)

// processerAnthropic 的 parseUsage 曾只读 anthropic 原生 input_tokens / output_tokens，
// 丢弃 AnthropicUsage 上声明的 openai 兼容字段（prompt_tokens / completion_tokens /
// cached_tokens，为 kimi 等混合返回准备）。结果：只在兼容字段回填 usage 的上游走
// anthropic 直通时 token 统计恒为 0。本用例锁定「缺原生字段时回退到兼容字段」。
func TestProcesserAnthropic_ParseUsageFallsBackToOpenAICompatFields(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantPrompt   int64
		wantComplete int64
		wantTotal    int64
		wantCached   int64
	}{
		{
			name:         "仅 openai 兼容字段：回退填充",
			body:         `{"usage":{"prompt_tokens":120,"completion_tokens":30,"cached_tokens":16}}`,
			wantPrompt:   120,
			wantComplete: 30,
			wantTotal:    150,
			wantCached:   16,
		},
		{
			name:         "anthropic 原生字段：行为不变",
			body:         `{"usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":8}}`,
			wantPrompt:   100,
			wantComplete: 20,
			wantTotal:    120,
			wantCached:   8,
		},
		{
			name:         "原生与兼容并存：优先原生",
			body:         `{"usage":{"input_tokens":200,"output_tokens":40,"prompt_tokens":1,"completion_tokens":1}}`,
			wantPrompt:   200,
			wantComplete: 40,
			wantTotal:    240,
			wantCached:   0,
		},
		{
			// 原生 Anthropic 不返回 total_tokens，但 kimi 一类混合返回会给；
			// 归一收敛后口径统一为「上游给了 total 就采用」，不再硬算 input+output。
			name:         "上游给了 total：直接采用，不重算",
			body:         `{"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":99}}`,
			wantPrompt:   10,
			wantComplete: 5,
			wantTotal:    99,
			wantCached:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatLog, _, err := ProcesserAnthropic(
				context.Background(),
				strings.NewReader(tt.body),
				false, // 非流式
				time.Now(),
				true, // disablePerformanceTracking
				false,
			)
			if err != nil {
				t.Fatalf("ProcesserAnthropic: %v", err)
			}
			u := chatLog.Usage
			if u.PromptTokens != tt.wantPrompt {
				t.Errorf("PromptTokens = %d, want %d", u.PromptTokens, tt.wantPrompt)
			}
			if u.CompletionTokens != tt.wantComplete {
				t.Errorf("CompletionTokens = %d, want %d", u.CompletionTokens, tt.wantComplete)
			}
			if u.TotalTokens != tt.wantTotal {
				t.Errorf("TotalTokens = %d, want %d", u.TotalTokens, tt.wantTotal)
			}
			if u.PromptTokensDetails.CachedTokens != tt.wantCached {
				t.Errorf("CachedTokens = %d, want %d", u.PromptTokensDetails.CachedTokens, tt.wantCached)
			}
		})
	}
}
