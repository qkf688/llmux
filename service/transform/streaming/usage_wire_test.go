package streaming

import (
	"reflect"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/responses"
)

// 本文件锁定「写出方向」的 usage 装配契约：三个写出点共用一份装配实现后，
// 两条协议线的键名、total 回退口径、零值明细判据必须各自稳定且互相对齐。
// 这些断言存在的意义是防回归——历史上写出点各写一套时已发散过两次
// （直写上游 total 与落库分叉；details 判据附加父级 >0 前置条件）。

func TestUsageWireFromModel_KeysPerProtocolLine(t *testing.T) {
	// 同一份 Usage 交两条线，除键名外产出必须完全对应。
	u := models.Usage{PromptTokens: 100, CompletionTokens: 42, TotalTokens: 142}
	u.PromptTokensDetails.CachedTokens = 80
	u.CompletionTokensDetails.ReasoningTokens = 7

	tests := []struct {
		name string
		got  map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "responses 线",
			got:  responsesUsageFromModel(u),
			want: map[string]interface{}{
				"input_tokens":         100,
				"output_tokens":        42,
				"total_tokens":         142,
				"input_tokens_details": map[string]interface{}{"cached_tokens": 80},
				"output_tokens_details": map[string]interface{}{
					"reasoning_tokens": 7,
				},
			},
		},
		{
			name: "openai chat 线",
			got:  openAIUsageFromModel(u),
			want: map[string]interface{}{
				"prompt_tokens":         100,
				"completion_tokens":     42,
				"total_tokens":          142,
				"prompt_tokens_details": map[string]interface{}{"cached_tokens": 80},
				"completion_tokens_details": map[string]interface{}{
					"reasoning_tokens": 7,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Fatalf("装配结果不符\n got: %#v\nwant: %#v", tt.got, tt.want)
			}
		})
	}
}

func TestUsageWireFromModel_DetailsAndTotalRules(t *testing.T) {
	withDetails := func(cached, reasoning, promptAudio int64) models.Usage {
		u := models.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
		u.PromptTokensDetails.CachedTokens = cached
		u.PromptTokensDetails.AudioTokens = promptAudio
		u.CompletionTokensDetails.ReasoningTokens = reasoning
		return u
	}

	tests := []struct {
		name string
		in   models.Usage
		want map[string]interface{}
	}{
		{
			// 零值明细不写出：「字段缺失＝未知」与「上游明确报 0」是两种语义。
			name: "明细全为 0 时不产出 details 子对象",
			in:   withDetails(0, 0, 0),
			want: map[string]interface{}{"input_tokens": 10, "output_tokens": 5, "total_tokens": 15},
		},
		{
			// 判据只看 detail 本身，不附加父级 >0 前置条件：与入站归一同口径。
			name: "prompt 为 0 但 cached 有真值时仍写出",
			in: func() models.Usage {
				u := models.Usage{CompletionTokens: 5}
				u.PromptTokensDetails.CachedTokens = 3
				return u
			}(),
			want: map[string]interface{}{
				"input_tokens":         0,
				"output_tokens":        5,
				"total_tokens":         5,
				"input_tokens_details": map[string]interface{}{"cached_tokens": 3},
			},
		},
		{
			// audio 有意不写出：客户端契约未含该字段，落库侧仍保留。
			name: "audio 明细不进线格式",
			in:   withDetails(0, 0, 9),
			want: map[string]interface{}{"input_tokens": 10, "output_tokens": 5, "total_tokens": 15},
		},
		{
			// total 缺失回退 prompt+completion，与 models.ResolveTotalTokens 同源。
			name: "total 缺失时回退为 prompt+completion",
			in:   models.Usage{PromptTokens: 10, CompletionTokens: 5},
			want: map[string]interface{}{"input_tokens": 10, "output_tokens": 5, "total_tokens": 15},
		},
		{
			// 上游显式给了 total 就原样采用，即使不等于两者之和（可能含 cache token）。
			name: "上游 total 优先于重算",
			in:   models.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 999},
			want: map[string]interface{}{"input_tokens": 10, "output_tokens": 5, "total_tokens": 999},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := responsesUsageFromModel(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("装配结果不符\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestUsageFromResponses(t *testing.T) {
	tests := []struct {
		name string
		in   *responses.ResponsesUsage
		want models.Usage
	}{
		{
			name: "明细指针为 nil 时明细归零",
			in:   &responses.ResponsesUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			want: models.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		},
		{
			name: "total 缺失时套用同一回退口径",
			in:   &responses.ResponsesUsage{InputTokens: 10, OutputTokens: 5},
			want: models.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		},
		{
			name: "cache / reasoning 明细逐字段搬运",
			in: &responses.ResponsesUsage{
				InputTokens:        100,
				OutputTokens:       42,
				TotalTokens:        142,
				InputTokenDetails:  &responses.ResponsesInputTokenDetails{CachedTokens: 80},
				OutputTokenDetails: &responses.ResponsesOutputTokenDetails{ReasoningTokens: 7},
			},
			want: func() models.Usage {
				u := models.Usage{PromptTokens: 100, CompletionTokens: 42, TotalTokens: 142}
				u.PromptTokensDetails.CachedTokens = 80
				u.CompletionTokensDetails.ReasoningTokens = 7
				return u
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usageFromResponses(tt.in); got != tt.want {
				t.Fatalf("转换结果不符\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestBuildResponsesUsage_NormalizesUpstreamAliases(t *testing.T) {
	// 出站装配前先过 models.UsageFromMap，故与落库侧认同一批上游写法：
	// 旧实现只读 prompt_tokens_details.cached_tokens / completion_tokens_details.reasoning_tokens，
	// 这些别名会被漏掉，造成「客户端看 0、DB 记真值」。
	tests := []struct {
		name string
		in   map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "空 usage 不写出（返回 nil 交调用点决定是否落字段）",
			in:   map[string]interface{}{},
			want: nil,
		},
		{
			name: "openai 标准嵌套明细",
			in: map[string]interface{}{
				"prompt_tokens":             float64(100),
				"completion_tokens":         float64(42),
				"total_tokens":              float64(142),
				"prompt_tokens_details":     map[string]interface{}{"cached_tokens": float64(80)},
				"completion_tokens_details": map[string]interface{}{"reasoning_tokens": float64(7)},
			},
			want: map[string]interface{}{
				"input_tokens":          100,
				"output_tokens":         42,
				"total_tokens":          142,
				"input_tokens_details":  map[string]interface{}{"cached_tokens": 80},
				"output_tokens_details": map[string]interface{}{"reasoning_tokens": 7},
			},
		},
		{
			name: "deepseek 系顶层 prompt_cache_hit_tokens 也认",
			in: map[string]interface{}{
				"prompt_tokens":           float64(100),
				"completion_tokens":       float64(42),
				"prompt_cache_hit_tokens": float64(60),
			},
			want: map[string]interface{}{
				"input_tokens":         100,
				"output_tokens":        42,
				"total_tokens":         142,
				"input_tokens_details": map[string]interface{}{"cached_tokens": 60},
			},
		},
		{
			name: "reasoning 记在 usage 顶层也认",
			in: map[string]interface{}{
				"prompt_tokens":     float64(10),
				"completion_tokens": float64(20),
				"reasoning_tokens":  float64(5),
			},
			want: map[string]interface{}{
				"input_tokens":          10,
				"output_tokens":         20,
				"total_tokens":          30,
				"output_tokens_details": map[string]interface{}{"reasoning_tokens": 5},
			},
		},
		{
			name: "input_tokens 与 prompt_tokens 并存时取原生 input_tokens",
			// 混合返回上游（kimi 一类）会同时下发原生与兼容位。归一层规则是
			// 「原生优先」，出站装配走同一归一入口，故与落库口径恒等——这正是本次
			// 改动把 buildResponsesUsage 接到 UsageFromMap 上要锁住的行为。
			in: map[string]interface{}{
				"input_tokens":      float64(200),
				"prompt_tokens":     float64(1),
				"output_tokens":     float64(50),
				"completion_tokens": float64(1),
			},
			want: map[string]interface{}{
				"input_tokens":  200,
				"output_tokens": 50,
				"total_tokens":  250,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildResponsesUsage(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("装配结果不符\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}
