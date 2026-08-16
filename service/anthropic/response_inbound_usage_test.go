package anthropic

import (
	"testing"
)

// ParseResponse（非流入站）曾自己手写 usage 解析，只认 anthropic 原生
// input_tokens / output_tokens / cache_read_input_tokens。而 processer 侧早已
// 补上「缺原生字段时回退 openai 兼容字段」（kimi 一类混合返回），两处口径分叉：
// 同一个上游走非流路径 token 统计仍为 0。本用例锁定两条路径同归一。
func TestParseResponse_UsageNormalization(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		wantPrompt    int64
		wantComplete  int64
		wantTotal     int64
		wantCached    int64
		wantReasoning int64
	}{
		{
			name:         "anthropic 原生字段",
			body:         `{"id":"msg_1","model":"claude","content":[],"usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":8}}`,
			wantPrompt:   100,
			wantComplete: 20,
			wantTotal:    120,
			wantCached:   8,
		},
		{
			name:         "仅 openai 兼容字段：回退填充",
			body:         `{"id":"msg_2","model":"kimi","content":[],"usage":{"prompt_tokens":120,"completion_tokens":30,"cached_tokens":16}}`,
			wantPrompt:   120,
			wantComplete: 30,
			wantTotal:    150,
			wantCached:   16,
		},
		{
			name:         "上游给了 total：直接采用，不重算",
			body:         `{"id":"msg_3","model":"claude","content":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":99}}`,
			wantPrompt:   10,
			wantComplete: 5,
			wantTotal:    99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ParseResponse([]byte(tt.body))
			if err != nil {
				t.Fatalf("ParseResponse: %v", err)
			}
			if resp.Usage == nil {
				t.Fatal("Usage = nil, want parsed usage")
			}
			u := resp.Usage
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
			if u.CompletionTokensDetails.ReasoningTokens != tt.wantReasoning {
				t.Errorf("ReasoningTokens = %d, want %d", u.CompletionTokensDetails.ReasoningTokens, tt.wantReasoning)
			}
		})
	}
}
