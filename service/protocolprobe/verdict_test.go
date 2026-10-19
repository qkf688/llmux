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
		if len(rec.Protocols) != 0 || rec.Primary != "" {
			t.Fatalf("protocols=%v primary=%q, want 空（无可用协议时不得给出可勾选集合）", rec.Protocols, rec.Primary)
		}
		if rec.Confidence != ConfidenceLow {
			t.Fatalf("confidence = %q, want low", rec.Confidence)
		}
		if !containsSubstring(rec.Actions, "部分支持") {
			t.Fatalf("actions 未提示部分支持: %v", rec.Actions)
		}
		// 没有可用协议时也要把「无从配置协议」说清，别让用户以为漏了建议。
		if !containsSubstring(rec.Actions, "没有可用的协议") {
			t.Fatalf("actions 未说明无可用协议: %v", rec.Actions)
		}
	})

	t.Run("single_usable_is_best", func(t *testing.T) {
		t.Parallel()
		inputs := []Input{
			usable(consts.ProtocolAnthropic),
			{Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegFailed},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		assertProtocolSet(t, rec, []consts.Protocol{consts.ProtocolAnthropic}, consts.ProtocolAnthropic)
		if rec.Confidence != ConfidenceHigh {
			t.Fatalf("confidence = %q, want high（唯一可用无需再比谁原生）", rec.Confidence)
		}
		if !containsSubstring(rec.Actions, "改为 Anthropic") {
			t.Fatalf("actions 未给出改主类型的建议: %v", rec.Actions)
		}
		if !containsSubstring(rec.Actions, "勾选 Anthropic，主格式 Anthropic") {
			t.Fatalf("actions 未给出按模型勾选协议的动作: %v", rec.Actions)
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
		// openai 的错误信封是 Anthropic 形状 = 转译路径，按「不勾转译协议」的规矩被排除；
		// 只剩 anthropic 一条进集合。这条断言就是「勾选集合排除转译」的锁。
		assertProtocolSet(t, rec, []consts.Protocol{consts.ProtocolAnthropic}, consts.ProtocolAnthropic)
		if rec.Confidence != ConfidenceHigh {
			t.Fatalf("confidence = %q, want high（唯一同族且字段全命中）", rec.Confidence)
		}
		if !containsSubstring(rec.Actions, "转译路径") {
			t.Fatalf("actions 未提示转译路径不建议勾选: %v", rec.Actions)
		}
		if !containsSubstring(rec.Actions, "勾选 Anthropic，主格式 Anthropic") {
			t.Fatalf("actions 未给出按模型勾选协议的动作: %v", rec.Actions)
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
		// 两条同族都在集合里（多条允许可少做格式转换），主格式按字段命中数取 responses。
		assertProtocolSet(t, rec,
			[]consts.Protocol{consts.ProtocolOpenAI, consts.ProtocolResponses},
			consts.ProtocolResponses)
		if rec.Confidence != ConfidenceMedium {
			t.Fatalf("confidence = %q, want medium", rec.Confidence)
		}
		if !containsSubstring(rec.Actions, "勾选 OpenAI Chat、OpenAI Responses，主格式 OpenAI Responses") {
			t.Fatalf("actions 未按排序结果给出勾选动作: %v", rec.Actions)
		}
	})

	t.Run("all_usable_translated_yields_no_set", func(t *testing.T) {
		t.Parallel()
		// 两条都可用、但错误信封都指向对方的家族 → 都是转译路径。按「不勾转译协议」的规矩
		// 集合为空：勾哪条都等于把请求交给上游网关转，宁可不给，也不给一个自相矛盾的按钮。
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
		if len(rec.Protocols) != 0 || rec.Primary != "" {
			t.Fatalf("protocols=%v primary=%q, want 空（全部像转译路径时无可放心勾选的集合）", rec.Protocols, rec.Primary)
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
		if !containsSubstring(rec.Actions, "不建议勾选这条协议") {
			t.Fatalf("actions 未说明每条为何不建议勾选: %v", rec.Actions)
		}
	})

	t.Run("unknown_family_not_treated_as_same_but_still_checkable", func(t *testing.T) {
		t.Parallel()
		// 指纹没采到（FamilyUnknown）不能算「同族」——否则会把「没测到」读成「测过且一致」。
		// 但它也不是转译证据：两条都该进集合，主格式保持当前主类型。这正是用户截图里的那种结论。
		inputs := []Input{
			{Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed, Family: FamilyUnknown},
			{Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed, Family: FamilyUnknown},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		assertProtocolSet(t, rec,
			[]consts.Protocol{consts.ProtocolOpenAI, consts.ProtocolAnthropic},
			consts.ProtocolOpenAI)
		if rec.Confidence != ConfidenceLow {
			t.Fatalf("confidence = %q, want low（分不出谁更原生）", rec.Confidence)
		}
		if !containsSubstring([]string{rec.Summary}, "无法区分") {
			t.Fatalf("summary = %q, 需明写无法区分", rec.Summary)
		}
	})

	t.Run("complete_fields_outrank_current_main_for_primary", func(t *testing.T) {
		t.Parallel()
		// 用户实测场景：三条都可用、指纹都未采到 → 集合是三条；OpenAI Chat 缺 2 项、
		// Responses 一项不缺、Anthropic 缺 1 项。主格式必须跟着字段最完整的那条走——
		// 若仍无条件保留当前主类型，报告会一边说「Chat 缺 2 项」一边推荐它。
		inputs := []Input{
			{
				Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed, Family: FamilyUnknown,
				Fields: fields(
					[]string{"object", "finish_reason", "usage.prompt_tokens", "usage.completion_tokens",
						"usage.prompt_tokens_details", "usage.completion_tokens_details"},
					[]string{"system_fingerprint", "service_tier"},
				),
			},
			{
				Protocol: consts.ProtocolResponses, NonStream: LegPassed, Stream: LegPassed, Family: FamilyUnknown,
				Fields: fields(
					[]string{"object", "status", "output[].type", "usage.input_tokens", "usage.output_tokens",
						"usage.input_tokens_details", "usage.output_tokens_details", "error"},
					nil,
				),
			},
			{
				Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed, Family: FamilyUnknown,
				Fields: fields(
					[]string{"type", "content[].type", "stop_reason", "usage.input_tokens", "usage.output_tokens",
						"usage.cache_creation_input_tokens", "usage.cache_read_input_tokens"},
					[]string{"stop_sequence"},
				),
			},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		assertProtocolSet(t, rec,
			[]consts.Protocol{consts.ProtocolOpenAI, consts.ProtocolResponses, consts.ProtocolAnthropic},
			consts.ProtocolResponses)
		if !containsSubstring(rec.Actions, "主格式建议取 OpenAI Responses（原生字段最完整）") {
			t.Fatalf("actions 未说明主格式为何改取 Responses: %v", rec.Actions)
		}
		if !containsSubstring(rec.Actions, "OpenAI Chat 也可用") {
			t.Fatalf("actions 把「当前主类型可用但字段不全」误述成不可用: %v", rec.Actions)
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
	if rec.Primary != string(consts.ProtocolAnthropic) {
		t.Fatalf("primary = %q, want anthropic（字段同分时耗时短者胜）", rec.Primary)
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
	if rec.Primary != string(consts.ProtocolOpenAI) {
		t.Fatalf("primary = %q, want openai（主类型优先）", rec.Primary)
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
	for _, must := range []string{"单次样本", "只对这个模型有效"} {
		if !strings.Contains(joined, must) {
			t.Fatalf("notes 缺少 %q: %v", must, rec.Notes)
		}
	}
}

// assertProtocolSet 断「勾选集合逐字相等（含顺序）+ 主格式落在集合内」。
// 集合顺序固定按 consts.AllProtocols，所以可以逐字比；主格式不在集合内会被保存侧拒绝，
// 所以这条不变量必须在单测里锁死。
func assertProtocolSet(t *testing.T, rec Recommendation, wantSet []consts.Protocol, wantPrimary consts.Protocol) {
	t.Helper()
	want := make([]string, 0, len(wantSet))
	for _, p := range wantSet {
		want = append(want, string(p))
	}
	if len(rec.Protocols) != len(want) {
		t.Fatalf("protocols = %v, want %v", rec.Protocols, want)
	}
	for i := range want {
		if rec.Protocols[i] != want[i] {
			t.Fatalf("protocols = %v, want %v", rec.Protocols, want)
		}
	}
	if rec.Primary != string(wantPrimary) {
		t.Fatalf("primary = %q, want %q", rec.Primary, wantPrimary)
	}
	for _, p := range rec.Protocols {
		if p == rec.Primary {
			return
		}
	}
	t.Fatalf("primary %q 不在集合 %v 内（保存侧会拒绝）", rec.Primary, rec.Protocols)
}

func containsSubstring(items []string, sub string) bool {
	for _, item := range items {
		if strings.Contains(item, sub) {
			return true
		}
	}
	return false
}
