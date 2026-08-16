package streaming

import (
	"testing"

	"github.com/qkf688/llmux/models"
)

// anthropic 流式路径此前在交侧信道**之前**先用白名单只取 input_tokens /
// output_tokens / cache_read_input_tokens 三个原生键，等于在调用点重写了一份
// 「哪些键算 usage」的知识。后果：kimi 一类走 anthropic 协议但只回 openai 兼容
// 字段的上游，快照全零被 SetUpstreamUsage 丢弃，流式 token 统计恒为 0——正是
// 非流 ParseResponse 已修掉的那个分叉。
//
// 本用例锁定「合并后的完整快照交候选表识别，键取舍不在调用点做」。
func TestMergeAnthropicUsage_CapturedThroughCandidateTable(t *testing.T) {
	tests := []struct {
		name         string
		startUsage   map[string]interface{}
		deltaUsage   map[string]interface{}
		wantPrompt   int64
		wantComplete int64
		wantTotal    int64
		wantCached   int64
		wantCaptured bool
	}{
		{
			name:         "anthropic 原生键：两处合并后各侧都在",
			startUsage:   map[string]interface{}{"input_tokens": float64(100), "cache_read_input_tokens": float64(8)},
			deltaUsage:   map[string]interface{}{"output_tokens": float64(20)},
			wantPrompt:   100,
			wantComplete: 20,
			wantTotal:    120,
			wantCached:   8,
			wantCaptured: true,
		},
		{
			name:         "只回 openai 兼容字段的上游：不能被白名单过滤成全零",
			startUsage:   map[string]interface{}{"prompt_tokens": float64(100), "cached_tokens": float64(8)},
			deltaUsage:   map[string]interface{}{"completion_tokens": float64(20)},
			wantPrompt:   100,
			wantComplete: 20,
			wantTotal:    120,
			wantCached:   8,
			wantCaptured: true,
		},
		{
			name:         "delta 显式带 0 不抹掉 start 的 input 真值",
			startUsage:   map[string]interface{}{"input_tokens": float64(100)},
			deltaUsage:   map[string]interface{}{"input_tokens": float64(0), "output_tokens": float64(20)},
			wantPrompt:   100,
			wantComplete: 20,
			wantTotal:    120,
			wantCaptured: true,
		},
		{
			name:         "上游显式给 total：采用上游值，不重算 prompt+completion",
			startUsage:   map[string]interface{}{"input_tokens": float64(100)},
			deltaUsage:   map[string]interface{}{"output_tokens": float64(20), "total_tokens": float64(99)},
			wantPrompt:   100,
			wantComplete: 20,
			wantTotal:    99,
			wantCaptured: true,
		},
		{
			name:         "两处都没有可识别的 token：快照被门禁丢弃",
			startUsage:   map[string]interface{}{"service_tier": "standard"},
			deltaUsage:   nil,
			wantCaptured: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sideChannel := models.NewTransformSideChannel(false)
			state := &realtimeStreamState{
				sideChannel:         sideChannel,
				anthropicStartUsage: tt.startUsage,
			}

			captureUpstreamUsageMap(state, mergeAnthropicUsage(state.anthropicStartUsage, tt.deltaUsage))

			u, ok := sideChannel.UpstreamUsage()
			if ok != tt.wantCaptured {
				t.Fatalf("captured = %v, want %v (usage=%+v)", ok, tt.wantCaptured, u)
			}
			if !tt.wantCaptured {
				return
			}
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
