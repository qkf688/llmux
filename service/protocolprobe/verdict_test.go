package protocolprobe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/qkf688/llmux/consts"
)

func fields(hit, miss []string) FieldReport { return FieldReport{Hit: hit, Miss: miss} }

// strongFields 造一份带「不可伪造的原生特征」命中的字段报告（强证据即证据本体）。
func strongFields(strong []string) FieldReport {
	return FieldReport{Hit: strong, Miss: nil, StrongHit: strong}
}

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
				Fields: strongFields([]string{"content[].signature"}),
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
			t.Fatalf("confidence = %q, want high（其余路径已证明是转译）", rec.Confidence)
		}
		if !containsSubstring(rec.Actions, "转译路径") {
			t.Fatalf("actions 未提示转译路径不建议勾选: %v", rec.Actions)
		}
		if !containsSubstring(rec.Actions, "勾选 Anthropic，主格式 Anthropic") {
			t.Fatalf("actions 未给出按模型勾选协议的动作: %v", rec.Actions)
		}
	})

	t.Run("lone_same_family_without_strong_evidence_is_medium", func(t *testing.T) {
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
			t.Fatalf("confidence = %q, want medium（只有弱证据，不许给 high）", rec.Confidence)
		}
		if rec.Primary != string(consts.ProtocolAnthropic) {
			t.Fatalf("primary = %q, want anthropic（同为弱证据时信封方言更难的胜出）", rec.Primary)
		}
	})

	t.Run("multiple_strong_candidates_rank_by_strong_count", func(t *testing.T) {
		t.Parallel()
		inputs := []Input{
			{
				Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyOpenAI, Fields: strongFields([]string{"system_fingerprint"}),
			},
			{
				Protocol: consts.ProtocolResponses, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyOpenAI, Fields: strongFields([]string{"encrypted_content", "response.output.encrypted"}),
			},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		// 两条都有强证据（网关在每条路都做了原生封装），按强证据数量取第一，仅作弱推荐。
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

	t.Run("no_strong_evidence_is_indistinguishable_not_field_count", func(t *testing.T) {
		t.Parallel()
		// 回归：此前按「命中字段总数」跨协议排序，Chat 的骨架比 Responses 少一项就凭空落后，
		// 于是推荐了一条转译路径。三家字段集本就不同，数量差不是证据。
		inputs := []Input{
			{
				Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyOpenAI, Fields: fields([]string{"a", "b"}, []string{"c"}),
			},
			{
				Protocol: consts.ProtocolResponses, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyOpenAI, Fields: fields([]string{"a", "b", "c"}, nil),
			},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		// 证据打平（同为 openai 信封、都无强证据、都无原生头）→ 不点名谁更原生。
		if rec.Confidence != ConfidenceLow {
			t.Fatalf("confidence = %q, want low（字段数量差不算证据）", rec.Confidence)
		}
		if !containsSubstring([]string{rec.Summary}, "无法区分") {
			t.Fatalf("summary = %q, 需明写无法区分", rec.Summary)
		}
		if rec.Primary != string(consts.ProtocolOpenAI) {
			t.Fatalf("primary = %q, want openai（打平时保持当前主类型）", rec.Primary)
		}
	})

	t.Run("id_prefix_conflict_excludes_translated_path", func(t *testing.T) {
		t.Parallel()
		// 回归：Chat 路径回 object=chat.completion 但 id 是 msg_ 前缀（Anthropic 形状）——
		// 这是翻译层拼出来的直接证据，哪怕字段完整、双模式全过也不能勾。
		inputs := []Input{
			{
				Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyUnknown, IDPrefix: "msg",
				Fields: fields([]string{"object", "finish_reason"}, []string{"service_tier"}),
			},
			{
				Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed,
				Family: FamilyUnknown, IDPrefix: "msg",
				Fields: fields([]string{"type", "content[].type"}, nil),
			},
		}
		rec := Recommend(consts.ProtocolOpenAI, inputs)
		assertProtocolSet(t, rec, []consts.Protocol{consts.ProtocolAnthropic}, consts.ProtocolAnthropic)
		if !containsSubstring(rec.Actions, "响应 id 是 msg 前缀") {
			t.Fatalf("actions 未说明 Chat 路径为何被排除: %v", rec.Actions)
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
		// 但它也不是转译证据：两条都该进集合，主格式保持当前主类型。
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

	t.Run("unforgeable_native_feature_beats_translating_gateway", func(t *testing.T) {
		t.Parallel()
		// 用户实测场景（glm-5.3-flash）：三条协议都可用。Chat 与 Responses 的错误信封被
		// 上游网关包成通用 OpenAI 信封（Responses 因此「同族」），Anthropic 的指纹没采到；
		// 只有 Anthropic 回了 content[].signature——翻译层造不出来的签名串。
		// 旧逻辑按「唯一同族」把 Responses 点成主协议，是错的：OpenAI 信封近乎恒真。
		chat := Input{
			Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyUnknown, IDPrefix: "msg",
			Fields: fields(
				[]string{"object", "finish_reason", "usage.prompt_tokens", "usage.completion_tokens", "usage.prompt_tokens_details"},
				[]string{"usage.completion_tokens_details", "system_fingerprint", "service_tier"},
			),
		}
		responses := Input{
			Protocol: consts.ProtocolResponses, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyOpenAI, IDPrefix: "resp",
			Fields: fields(
				[]string{"object", "status", "output[].type", "usage.input_tokens", "usage.output_tokens", "usage.output_tokens_details"},
				[]string{"usage.input_tokens_details", "encrypted_content"},
			),
		}
		anthropic := Input{
			Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyUnknown, IDPrefix: "msg",
			Fields: FieldReport{
				Hit:       []string{"type", "content[].type", "stop_reason", "usage.input_tokens", "usage.output_tokens", "usage.cache_read_input_tokens", "content[].signature"},
				Miss:      []string{"stop_sequence", "usage.cache_creation_input_tokens"},
				StrongHit: []string{"content[].signature"},
			},
		}
		rec := Recommend(consts.ProtocolOpenAI, []Input{chat, responses, anthropic})
		// Chat 被 id 前缀矛盾判为转译（回了 chat.completion 骨架却用 msg_ 的 id），不进集合；
		// Responses 只有「回 OpenAI 信封」这条弱证据，Anthropic 有签名串这条强证据 → 后者主格式。
		assertProtocolSet(t, rec,
			[]consts.Protocol{consts.ProtocolResponses, consts.ProtocolAnthropic},
			consts.ProtocolAnthropic)
		if rec.Confidence != ConfidenceHigh {
			t.Fatalf("confidence = %q, want high（唯一强证据）", rec.Confidence)
		}
		if !containsSubstring([]string{rec.Reason}, "翻译层造不出来") {
			t.Fatalf("reason 未说明强证据为何可信: %q", rec.Reason)
		}
		if !containsSubstring(rec.Actions, "响应 id 是 msg 前缀") {
			t.Fatalf("actions 未说明 Chat 路径为何被排除: %v", rec.Actions)
		}
	})
}

// 耗时只在证据完全打平时用于打破平局，不得单独决定名次：本例两条都无强证据、
// 都无原生头、指纹都没采到，耗时短的 anthropic 胜出。
func TestRecommend_TieBreakByLatencyOnlyWhenEvidenceEqual(t *testing.T) {
	t.Parallel()
	inputs := []Input{
		{
			Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyUnknown, NonStreamMS: 900,
			Fields: fields([]string{"a", "b"}, nil),
		},
		{
			Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyUnknown, NonStreamMS: 100,
			Fields: fields([]string{"a", "b"}, nil),
		},
	}
	rec := Recommend(consts.ProtocolAnthropic, inputs)
	if rec.Primary != string(consts.ProtocolAnthropic) {
		t.Fatalf("primary = %q, want anthropic（字段同分时耗时短者胜）", rec.Primary)
	}
}

// 主类型在证据打平时优先于耗时：避免一次偶发快就把「当前配置」判成次优。
func TestRecommend_MainTypePreferredOverLatency(t *testing.T) {
	t.Parallel()
	inputs := []Input{
		{
			Protocol: consts.ProtocolOpenAI, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyUnknown, NonStreamMS: 100,
			Fields: fields([]string{"a", "b"}, nil),
		},
		{
			Protocol: consts.ProtocolAnthropic, NonStream: LegPassed, Stream: LegPassed,
			Family: FamilyUnknown, NonStreamMS: 900,
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

// 回包形状回归：Protocols 必须是空数组而不是 null。协议全挂时走的正是这条分支，
// 而前端拿到 null 再取 .length 会把整页带崩——曾经真实发生过（上游全挂时白屏）。
func TestRecommend_ProtocolsIsEmptyArrayNotNull(t *testing.T) {
	t.Parallel()
	inputs := []Input{
		{Protocol: consts.ProtocolOpenAI, NonStream: LegFailed, Stream: LegFailed},
	}
	for _, rec := range []Recommendation{Recommend(consts.ProtocolOpenAI, inputs)} {
		raw, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(raw), `"protocols":[]`) {
			t.Fatalf("protocols 序列化成 %s, want 空数组（null 会让前端 .length 崩）", raw)
		}
	}
}
