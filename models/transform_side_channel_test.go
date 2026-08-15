package models

import "testing"

// token 全为 0 的快照必须被丢弃：上游只要回包带 usage 对象（哪怕字段全 0 或键名
// 不认识），协议适配器就会产出非 nil 的全零 Usage。若照收，落库侧会把它当成最可信
// 来源并跳过 missing 告警，UsageSource 就失去了区分「上游没给」与「链路丢了」的作用。
func TestTransformSideChannel_SetUpstreamUsageDiscardsAllZero(t *testing.T) {
	tests := []struct {
		name    string
		usage   Usage
		wantOK  bool
		wantTot int64
	}{
		{
			name:   "全零：丢弃",
			usage:  Usage{},
			wantOK: false,
		},
		{
			name: "只有 details 非零、token 全零：仍视为无有效计数",
			usage: func() Usage {
				u := Usage{}
				u.CompletionTokensDetails.ReasoningTokens = 30
				return u
			}(),
			wantOK: false,
		},
		{
			name:    "只有 prompt_tokens：算有效",
			usage:   Usage{PromptTokens: 7},
			wantOK:  true,
			wantTot: 0,
		},
		{
			name:    "正常快照",
			usage:   Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			wantOK:  true,
			wantTot: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewTransformSideChannel(false)
			c.SetUpstreamUsage(tt.usage)

			got, ok := c.UpstreamUsage()
			if ok != tt.wantOK {
				t.Fatalf("UpstreamUsage ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got.TotalTokens != tt.wantTot {
				t.Errorf("TotalTokens = %d, want %d", got.TotalTokens, tt.wantTot)
			}
		})
	}
}

// 全零快照不能把先前记录的有效值抹掉——「最后观测胜出」只适用于有效观测。
func TestTransformSideChannel_AllZeroDoesNotOverwriteValid(t *testing.T) {
	c := NewTransformSideChannel(false)
	c.SetUpstreamUsage(Usage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120})
	c.SetUpstreamUsage(Usage{})

	got, ok := c.UpstreamUsage()
	if !ok || got.TotalTokens != 120 {
		t.Errorf("UpstreamUsage = (%d, %v), want (120, true)", got.TotalTokens, ok)
	}
}

// nil 接收者上的所有方法必须安全：直通路径调用方直接传 nil。
func TestTransformSideChannel_NilReceiverSafe(t *testing.T) {
	var c *TransformSideChannel

	if c.WantsRawBody() {
		t.Error("WantsRawBody on nil = true, want false")
	}
	c.AppendRawBody("x")
	if got := c.RawBody(); got != "" {
		t.Errorf("RawBody on nil = %q, want empty", got)
	}
	c.SetUpstreamUsage(Usage{TotalTokens: 1})
	if _, ok := c.UpstreamUsage(); ok {
		t.Error("UpstreamUsage on nil returned ok=true, want false")
	}
}
