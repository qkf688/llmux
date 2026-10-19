package protocolprobe

import (
	"strings"
	"testing"

	"github.com/qkf688/llmux/consts"
)

func fields(hit, miss []string) FieldReport { return FieldReport{Hit: hit, Miss: miss} }

func TestVerdictOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   Input
		want Verdict
	}{
		{
			name: "both_passed",
			in:   Input{NonStream: LegPassed, Stream: LegPassed},
			want: VerdictUsable,
		},
		{
			name: "only_non_stream",
			in:   Input{NonStream: LegPassed, Stream: LegFailed},
			want: VerdictPartial,
		},
		{
			// 流式回了 SSE 但不合规仍算「没通过」——它是实现质量问题，不是可用。
			name: "stream_noncompliant_is_not_usable",
			in:   Input{NonStream: LegPassed, Stream: LegNonCompliant},
			want: VerdictPartial,
		},
		{
			name: "both_failed",
			in:   Input{NonStream: LegFailed, Stream: LegFailed},
			want: VerdictUnusable,
		},
		{
			name: "unknown_legs",
			in:   Input{NonStream: LegUnknown, Stream: LegUnknown},
			want: VerdictUnusable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := VerdictOf(tc.in); got != tc.want {
				t.Fatalf("VerdictOf = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRecommend_Branches(t *testing.T) {
	t.Parallel()
	usable := func(p consts.Protocol) Input {
		return Input{Protocol: p, NonStream: LegPassed, Stream: LegPassed}
	}

	t.Run("no_usable_protocol", func(t *testing.T) {
		t.Parallel()
		inputs := []Input{
			{Protocol: consts.ProtocolOpenAI, NonStream: LegFailed, Stream: LegFailed},
			{Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegNonCompliant},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		if rec.Protocol != "" {
			t.Fatalf("protocol = %q, want 空（无可用协议时不得点名）", rec.Protocol)
		}
		if rec.Confidence != ConfidenceLow {
			t.Fatalf("confidence = %q, want low", rec.Confidence)
		}
		if !containsSubstring(rec.Actions, "部分支持") {
			t.Fatalf("actions 未提示部分支持: %v", rec.Actions)
		}
		// 没有可用协议时也要把「无从勾选端点」说清，别让用户以为漏了建议。
		if !containsSubstring(rec.Actions, "没有可勾选的协议端点") {
			t.Fatalf("actions 未说明无可勾选端点: %v", rec.Actions)
		}
	})

	t.Run("single_usable_is_best", func(t *testing.T) {
		t.Parallel()
		inputs := []Input{
			usable(consts.ProtocolAnthropic),
			{Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegFailed},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		if rec.Protocol != string(consts.ProtocolAnthropic) {
			t.Fatalf("protocol = %q, want anthropic", rec.Protocol)
		}
		if rec.Confidence != ConfidenceHigh {
			t.Fatalf("confidence = %q, want high（唯一可用无需再比谁原生）", rec.Confidence)
		}
		if !containsSubstring(rec.Actions, "改为 Anthropic") {
			t.Fatalf("actions 未给出改主类型的建议: %v", rec.Actions)
		}
		if !containsSubstring(rec.Actions, "建议该模型勾选 Anthropic 协议端点") {
			t.Fatalf("actions 未给出按模型勾选端点的动作: %v", rec.Actions)
		}
	})

	t.Run("lone_same_family_wins_with_full_fields", func(t *testing.T) {
		t.Parallel()
		inputs := []Input{
			{
				Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyAnthropic,
				Fields: fields([]string{"a", "b", "c"}, nil),
			},
			{
				Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyAnthropic, // 错误信封是 Anthropic 形状 → 这条路被转译
				Fields: fields([]string{"a"}, []string{"b", "c"}),
			},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		if rec.Protocol != string(consts.ProtocolAnthropic) {
			t.Fatalf("protocol = %q, want anthropic", rec.Protocol)
		}
		if rec.Confidence != ConfidenceHigh {
			t.Fatalf("confidence = %q, want high（唯一同族且字段全命中）", rec.Confidence)
		}
		if !containsSubstring(rec.Actions, "转译路径") {
			t.Fatalf("actions 未提示转译路径不建议开端点: %v", rec.Actions)
		}
		if !containsSubstring(rec.Actions, "建议该模型勾选 Anthropic 协议端点") {
			t.Fatalf("actions 未给出按模型勾选端点的动作: %v", rec.Actions)
		}
	})

	t.Run("lone_same_family_with_missing_fields_is_medium", func(t *testing.T) {
		t.Parallel()
		inputs := []Input{
			{
				Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyAnthropic,
				Fields: fields([]string{"a"}, []string{"b"}),
			},
			{
				Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyUnknown,
				Fields: fields([]string{"a", "b", "c"}, nil),
			},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		if rec.Confidence != ConfidenceMedium {
			t.Fatalf("confidence = %q, want medium（字段没全命中不许给 high）", rec.Confidence)
		}
	})

	t.Run("multiple_same_family_rank_by_fields", func(t *testing.T) {
		t.Parallel()
		inputs := []Input{
			{
				Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyOpenAI, Fields: fields([]string{"a"}, []string{"b", "c"}),
			},
			{
				Protocol: consts.ProtocolResponses, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyOpenAI, Fields: fields([]string{"a", "b"}, []string{"c"}),
			},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		if rec.Protocol != string(consts.ProtocolResponses) {
			t.Fatalf("protocol = %q, want responses（字段命中更多）", rec.Protocol)
		}
		if rec.Confidence != ConfidenceMedium {
			t.Fatalf("confidence = %q, want medium", rec.Confidence)
		}
		if !containsSubstring(rec.Actions, "建议该模型勾选 OpenAI Responses 协议端点") {
			t.Fatalf("actions 未按排序结果给出勾选动作: %v", rec.Actions)
		}
	})

	t.Run("no_same_family_refuses_to_pick", func(t *testing.T) {
		t.Parallel()
		// 共识口径：证据不足时写「无法区分」，不点名协议。
		inputs := []Input{
			{
				Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyAnthropic, Fields: fields([]string{"a", "b", "c"}, nil),
			},
			{
				Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyOpenAI, Fields: fields([]string{"a", "b", "c"}, nil),
			},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		if rec.Protocol != "" {
			t.Fatalf("protocol = %q, want 空（无法区分时不许点名）", rec.Protocol)
		}
		if rec.Confidence != ConfidenceLow {
			t.Fatalf("confidence = %q, want low", rec.Confidence)
		}
		if !containsSubstring([]string{rec.Summary}, "无法区分") {
			t.Fatalf("summary = %q, 需明写无法区分", rec.Summary)
		}
		if !containsSubstring(rec.Actions, "保持当前主类型") {
			t.Fatalf("actions 未给弱建议: %v", rec.Actions)
		}
		// 分不出谁更原生时，仍要给出「这个模型可勾哪些端点」的候选（附实测提示），
		// 而不是一句「无法区分」就把人卡住。
		if !containsSubstring(rec.Actions, "可勾选这些协议端点") {
			t.Fatalf("actions 未列出可勾选端点: %v", rec.Actions)
		}
	})

	t.Run("unknown_family_not_treated_as_same", func(t *testing.T) {
		t.Parallel()
		// 指纹没采到（FamilyUnknown）不能算「同族」——否则会把「没测到」读成「测过且一致」。
		inputs := []Input{
			{Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed, Family: FamilyUnknown},
			{Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed, Family: FamilyUnknown},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		if rec.Protocol != "" {
			t.Fatalf("protocol = %q, want 空（两条都未采到指纹 → 无法区分）", rec.Protocol)
		}
	})
}

// 耗时只在证据完全打平时用于打破平局，不得单独决定名次：本例字段命中相同、
// 两条都同族，耗时短的 anthropic 胜出。
func TestRecommend_TieBreakByLatencyOnlyWhenEvidenceEqual(t *testing.T) {
	t.Parallel()
	inputs := []Input{
		{
			Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyOpenAI, NonStreamMS: 900,
			Fields: fields([]string{"a", "b"}, nil),
		},
		{
			Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyAnthropic, NonStreamMS: 100,
			Fields: fields([]string{"a", "b"}, nil),
		},
	}
	rec := Recommend(consts.ProtocolAnthropic, inputs)
	if rec.Protocol != string(consts.ProtocolAnthropic) {
		t.Fatalf("protocol = %q, want anthropic（字段同分时耗时短者胜）", rec.Protocol)
	}
}

// 主类型在字段同分时优先于耗时：避免一次偶发快就把「当前配置」判成次优。
func TestRecommend_MainTypePreferredOverLatency(t *testing.T) {
	t.Parallel()
	inputs := []Input{
		{
			Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyOpenAI, NonStreamMS: 100,
			Fields: fields([]string{"a", "b"}, nil),
		},
		{
			Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyAnthropic, NonStreamMS: 900,
			Fields: fields([]string{"a", "b"}, nil),
		},
	}
	rec := Recommend(consts.ProtocolOpenAI, inputs)
	if rec.Protocol != string(consts.ProtocolOpenAI) {
		t.Fatalf("protocol = %q, want openai（主类型优先）", rec.Protocol)
	}
}

func TestRecommend_AlwaysCarriesFixedNotes(t *testing.T) {
	t.Parallel()
	rec := Recommend(consts.ProtocolOpenAI, []Input{{
		Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
	}})
	if len(rec.Notes) != 3 {
		t.Fatalf("notes = %v, want 3 条固定免责", rec.Notes)
	}
	joined := strings.Join(rec.Notes, "|")
	for _, must := range []string{"单次样本", "供应商级"} {
		if !strings.Contains(joined, must) {
			t.Fatalf("notes 缺少 %q: %v", must, rec.Notes)
		}
	}
}

func containsSubstring(items []string, sub string) bool {
	for _, item := range items {
		if strings.Contains(item, sub) {
			return true
		}
	}
	return false
}
