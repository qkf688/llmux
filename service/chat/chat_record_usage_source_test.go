package chat

import (
	"testing"

	"github.com/qkf688/llmux/models"
)

func TestResolveUsageSource(t *testing.T) {
	upstream := models.Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}
	upstream.CompletionTokensDetails.ReasoningTokens = 40

	tests := []struct {
		name string
		// processer 解析出的 usage（下游流或直通上游响应）
		parsed models.Usage
		// nil 表示未走协议转换（直通）
		sideChannel func() *models.TransformSideChannel
		wantSource  string
		wantTotal   int64
		wantReason  int64
	}{
		{
			name:   "旁路有上游 usage：覆盖 processer 结果并标 upstream",
			parsed: models.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
			sideChannel: func() *models.TransformSideChannel {
				c := models.NewTransformSideChannel(false)
				c.SetUpstreamUsage(upstream)
				return c
			},
			wantSource: models.UsageSourceUpstream,
			wantTotal:  150,
			wantReason: 40,
		},
		{
			name:        "直通路径：processer 读的就是上游响应，标 passthrough",
			parsed:      models.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			sideChannel: func() *models.TransformSideChannel { return nil },
			wantSource:  models.UsageSourcePassthrough,
			wantTotal:   15,
		},
		{
			name:   "走了转换但旁路没拿到：退化为下游反解，标 downstream",
			parsed: models.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			sideChannel: func() *models.TransformSideChannel {
				return models.NewTransformSideChannel(false)
			},
			wantSource: models.UsageSourceDownstream,
			wantTotal:  15,
		},
		{
			name:        "两条路都为空：标 missing，不静默当成上游报 0",
			parsed:      models.Usage{},
			sideChannel: func() *models.TransformSideChannel { return nil },
			wantSource:  models.UsageSourceMissing,
			wantTotal:   0,
		},
		{
			name:   "只有 prompt_tokens 也算有效，不判 missing",
			parsed: models.Usage{PromptTokens: 7},
			sideChannel: func() *models.TransformSideChannel {
				return models.NewTransformSideChannel(false)
			},
			wantSource: models.UsageSourceDownstream,
			wantTotal:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := models.ChatLog{Usage: tt.parsed}
			resolveUsageSource(&log, tt.sideChannel(), "p1")

			if log.UsageSource != tt.wantSource {
				t.Errorf("UsageSource = %q, want %q", log.UsageSource, tt.wantSource)
			}
			if log.Usage.TotalTokens != tt.wantTotal {
				t.Errorf("TotalTokens = %d, want %d", log.Usage.TotalTokens, tt.wantTotal)
			}
			if log.Usage.CompletionTokensDetails.ReasoningTokens != tt.wantReason {
				t.Errorf("ReasoningTokens = %d, want %d",
					log.Usage.CompletionTokensDetails.ReasoningTokens, tt.wantReason)
			}
		})
	}
}
