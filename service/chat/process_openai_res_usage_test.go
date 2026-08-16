package chat

import (
	"context"
	"strings"
	"testing"
	"time"
)

// processerOpenAiRes 的 usage 解析曾自带一套 OpenAIResUsage 结构体，只声明了
// input_tokens_details.cached_tokens，完全不读 output_tokens_details.reasoning_tokens。
// 结果：openai-res 上游走直通（无协议转换、侧信道不介入）时 reasoning 落库恒为 0。
// 本用例锁定「openai-res 的 reasoning / cache 明细都要落到统一模型」。
func TestProcesserOpenAiRes_ParsesReasoningAndCacheDetails(t *testing.T) {
	const bodyWithDetails = `{"usage":{` +
		`"input_tokens":100,"output_tokens":20,"total_tokens":120,` +
		`"input_tokens_details":{"cached_tokens":8},` +
		`"output_tokens_details":{"reasoning_tokens":15}}}`

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
			name:          "复数键名 details：cache 与 reasoning 都要取到",
			body:          bodyWithDetails,
			wantPrompt:    100,
			wantComplete:  20,
			wantTotal:     120,
			wantCached:    8,
			wantReasoning: 15,
		},
		{
			name:         "无明细：不虚构数值",
			body:         `{"usage":{"input_tokens":30,"output_tokens":5,"total_tokens":35}}`,
			wantPrompt:   30,
			wantComplete: 5,
			wantTotal:    35,
		},
		{
			name:         "total 缺失：回退 input+output",
			body:         `{"usage":{"input_tokens":7,"output_tokens":3}}`,
			wantPrompt:   7,
			wantComplete: 3,
			wantTotal:    10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatLog, _, err := ProcesserOpenAiRes(
				context.Background(),
				strings.NewReader(tt.body),
				false, // 非流式
				time.Now(),
				true, // disablePerformanceTracking
				false,
			)
			if err != nil {
				t.Fatalf("ProcesserOpenAiRes: %v", err)
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
			if u.CompletionTokensDetails.ReasoningTokens != tt.wantReasoning {
				t.Errorf("ReasoningTokens = %d, want %d", u.CompletionTokensDetails.ReasoningTokens, tt.wantReasoning)
			}
		})
	}
}
