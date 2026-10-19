package protocolprobe

import (
	"fmt"
	"sort"
	"strings"

	"github.com/qkf688/llmux/consts"
)

// LegStatus 一条探针腿的判定结果。刻意分出 noncompliant：上游**回了 SSE 但不合规**
// （缺终止事件 / 无内容增量）与**压根不按流式回**是两件事，前者是实现质量问题、
// 后者是能力缺失，管理端的处置完全不同。
type LegStatus string

const (
	LegPassed       LegStatus = "passed"
	LegFailed       LegStatus = "failed"
	LegNonCompliant LegStatus = "noncompliant"
	LegUnknown      LegStatus = "unknown"
)

// Verdict 一条协议的可用性判定（只回答「能不能用」，不回答「是不是最好」）。
type Verdict string

const (
	VerdictUsable   Verdict = "usable"   // 非流式与流式都过
	VerdictPartial  Verdict = "partial"  // 只过一种模式
	VerdictUnusable Verdict = "unusable" // 两种模式都没过
)

// Confidence 建议的置信度。三档都是机械可判的（见 Recommend），不允许出现第四档。
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Input 一条协议的检测事实（由 handler/testapi 采集后喂进来；本包只判不采）。
type Input struct {
	Protocol  consts.Protocol
	NonStream LegStatus
	Stream    LegStatus
	// Family 是无效模型请求采到的错误信封家族；FamilyUnknown 表示没采到可判的
	// 指纹（连不上、401、或形状不认识）——此时「是否直通」无从谈起。
	Family Family
	// NativeHeaders 该协议家族的原生响应头（只作佐证，不参与判定）。
	NativeHeaders []string
	Fields        FieldReport
	// NonStreamMS 单次非流式耗时，只在证据完全打平时用于打破平局；正常路径不参与排名。
	NonStreamMS int64
	// EndpointConfigured 该协议是否已配端点（影响建议话术，不影响判定）。
	EndpointConfigured bool
}

// VerdictOf 判单条协议的可用性。
func VerdictOf(in Input) Verdict {
	switch {
	case in.NonStream == LegPassed && in.Stream == LegPassed:
		return VerdictUsable
	case in.NonStream == LegPassed || in.Stream == LegPassed:
		return VerdictPartial
	default:
		return VerdictUnusable
	}
}

// Recommendation 一次检测的总评。Protocols 为空 = 一条可勾选的协议都没有。
//
// 一键写入的是「允许集合 + 主格式」，不是「只走一条」：多勾几条能让客户端用哪种协议
// 进来就走哪条，省掉没必要的格式转换；主格式只在「客户端发来的协议不在集合里」时
// 决定转成哪种格式。两项必须同时给——只给集合而不给主格式，集合外的协议就没有转换目标。
type Recommendation struct {
	// Protocols 建议勾选的协议集合（可用且未被判为转译），顺序与 consts.AllProtocols 一致。
	Protocols []string `json:"protocols"`
	// Primary 建议的主格式，必须落在 Protocols 内（保存侧按同一判据校验）。
	Primary    string     `json:"primary"`
	Confidence Confidence `json:"confidence"`
	Summary    string     `json:"summary"`
	Reason     string     `json:"reason"`
	Actions    []string   `json:"actions"`
	Notes      []string   `json:"notes"`
}

// displayName 仅用于服务端句子里的协议称呼；前端展示名另有单源（webui 的 PROTOCOL_LABEL）。
func displayName(p consts.Protocol) string {
	switch p {
	case consts.ProtocolOpenAI:
		return "OpenAI Chat"
	case consts.ProtocolResponses:
		return "OpenAI Responses"
	case consts.ProtocolAnthropic:
		return "Anthropic"
	default:
		return string(p)
	}
}

// fixedNotes 每次都带的三句免责：缺了任一条，结论都会被读成比实际更硬。
func fixedNotes() []string {
	return []string{
		"字段「有」才算证据，「没有」只算弱证据——很多字段是否出现取决于模型与请求",
		"耗时是单次样本，只作展示，不参与排名",
		"协议结论只对这个模型有效：一键写入写的是该模型的关联设置；改供应商主类型则会影响该供应商下所有模型",
	}
}

// Recommend 给总评。判据（按序，先命中先返回）：
//
//  1. 无协议双模式全过 → low：「没有一条协议同时通过非流式与流式」；
//  2. 只有一条可用     → high：唯一可用，无需区分谁更原生；
//  3. 多条可用且**有且只有一条**的错误信封与自身协议同族 → high/medium（字段全命中才是 high）；
//  4. 多条可用且多条同族 → medium：按字段命中数排（网关在每条路都做了原生封装，分不出后端家族）；
//  5. 多条可用且**没有一条**同族 → low：**无法区分**，不点名协议，只给当前配置的弱参考。
func Recommend(main consts.Protocol, inputs []Input) Recommendation {
	rec := Recommendation{Confidence: ConfidenceLow, Notes: fixedNotes(), Actions: []string{}}

	var usable, partial, unusable []Input
	for _, in := range inputs {
		switch VerdictOf(in) {
		case VerdictUsable:
			usable = append(usable, in)
		case VerdictPartial:
			partial = append(partial, in)
		default:
			unusable = append(unusable, in)
		}
	}

	rec.Actions = append(rec.Actions, unusableActions(unusable)...)
	rec.Actions = append(rec.Actions, partialActions(partial)...)

	switch {
	case len(usable) == 0:
		rec.Summary = "没有一条协议同时通过非流式与流式"
		rec.Reason = "本模型当前不能通过任何一条已配置协议提供完整服务；先修凭据或端点 URL 再复测。"
		if len(partial) > 0 {
			rec.Reason = "只有协议处于「部分支持」状态（单模式可用、双模式不通），不建议勾选这些协议。"
		}
		rec.Actions = append(rec.Actions, "本轮没有可用的协议（两种模式都没通过）")
		return rec
	case len(usable) == 1:
		only := usable[0]
		rec.Protocols = []string{string(only.Protocol)}
		rec.Primary = string(only.Protocol)
		rec.Confidence = ConfidenceHigh
		rec.Summary = fmt.Sprintf("只有 %s 这条协议可用", displayName(only.Protocol))
		rec.Reason = "其余协议至少有一种模式不通；唯一可用的协议就是最优解，无需再比谁更原生。"
		rec.Actions = append(rec.Actions, mainTypeAction(only.Protocol, main), endpointAction(rec.Protocols, rec.Primary))
		return rec
	}

	var native []Input
	for _, in := range usable {
		if in.Family != FamilyUnknown && in.Family == FamilyOfProtocol(in.Protocol) {
			native = append(native, in)
		}
	}

	switch {
	case len(native) == 1:
		best := rank(native, main)[0]
		rec.Protocols = allowSet(usable)
		rec.Primary = string(best.Protocol)
		rec.Confidence = ConfidenceMedium
		if len(best.Fields.Miss) == 0 {
			rec.Confidence = ConfidenceHigh
		}
		rec.Summary = fmt.Sprintf("建议主协议 %s", displayName(best.Protocol))
		rec.Reason = fmt.Sprintf("只有它的错误信封是自身协议形状（%s 同族），其余可用协议回的是别的家族——"+
			"说明其余路径被网关转译，协议特有字段最容易在那里丢；它的原生字段命中 %d/%d。",
			FamilyOfProtocol(best.Protocol), len(best.Fields.Hit), len(best.Fields.Hit)+len(best.Fields.Miss))
		rec.Actions = append(rec.Actions, mainTypeAction(best.Protocol, main), endpointAction(rec.Protocols, rec.Primary))
		rec.Actions = append(rec.Actions, translatedActions(usable, best.Protocol)...)
		return rec
	case len(native) > 1:
		best := rank(native, main)[0]
		rec.Protocols = allowSet(usable)
		rec.Primary = string(best.Protocol)
		rec.Confidence = ConfidenceMedium
		rec.Summary = fmt.Sprintf("建议主协议 %s（区分度有限）", displayName(best.Protocol))
		rec.Reason = fmt.Sprintf("有多条协议的错误信封都与自身协议同族，说明网关在每条路都做了原生封装，"+
			"分不出后端家族；按原生字段命中数（%d/%d）排序取第一，仅作弱推荐。",
			len(best.Fields.Hit), len(best.Fields.Hit)+len(best.Fields.Miss))
		rec.Actions = append(rec.Actions, mainTypeAction(best.Protocol, main), endpointAction(rec.Protocols, rec.Primary))
		return rec
	default:
		rec.Summary = "无法区分：各可用协议的响应都没有指向自身协议"
		rec.Protocols = allowSet(usable)
		rec.Primary = pickPrimary(main, rec.Protocols, usable)
		// 逐条说明为什么没进集合（转译路径 / 缺原生字段）——排除掉的不给理由，用户无从判断。
		rec.Actions = append(rec.Actions, translatedActions(usable, "")...)
		if len(rec.Protocols) == 0 {
			// 可用但全是转译路径：勾哪条都等于让上游网关转，宁可不给集合。
			rec.Reason = "所有可用协议的错误信封都指向别的家族（像被网关统一改写），没有一条能当作原生路径；" +
				"这里不给可勾选集合——勾哪条都等于把请求交给上游网关转。"
			rec.Actions = append(rec.Actions, fmt.Sprintf("保持当前主类型 %s 不变，除非你另有把握", displayName(main)))
			return rec
		}
		rec.Reason = "所有可用协议的错误信封都没有采到自身家族的形状（可能都被网关统一改写，" +
			"也可能指纹请求未采到），无法判断谁更原生；这里不点名唯一主协议，但仍给出可勾选的集合。"
		switch {
		case rec.Primary == string(main):
			rec.Actions = append(rec.Actions, fmt.Sprintf("保持当前主类型 %s 不变，除非你另有把握", displayName(main)))
		case containsProtocol(rec.Protocols, main):
			// 当前主类型本身可用，只是原生字段没别人完整——说清是「换更完整的」而不是「当前不可用」。
			rec.Actions = append(rec.Actions, fmt.Sprintf("主格式建议取 %s（原生字段最完整）；当前主类型 %s 也可用",
				displayName(consts.Protocol(rec.Primary)), displayName(main)))
		default:
			rec.Actions = append(rec.Actions, fmt.Sprintf("当前主类型 %s 本次不可用，主格式建议取 %s",
				displayName(main), displayName(consts.Protocol(rec.Primary))))
		}
		rec.Actions = append(rec.Actions, endpointAction(rec.Protocols, rec.Primary))
		return rec
	}
}

// rank 在同族候选里排序：原生字段命中数 → 是否当前主类型 → 单次耗时 → 协议名。
// 后两级只在证据完全打平时用：耗时是单次样本，绝不单独决定名次。
func rank(candidates []Input, main consts.Protocol) []Input {
	out := append([]Input(nil), candidates...)
	sort.SliceStable(out, func(i, j int) bool {
		hi, hj := len(out[i].Fields.Hit), len(out[j].Fields.Hit)
		if hi != hj {
			return hi > hj
		}
		if out[i].Protocol == main && out[j].Protocol != main {
			return true
		}
		if out[j].Protocol == main && out[i].Protocol != main {
			return false
		}
		if out[i].NonStreamMS != out[j].NonStreamMS {
			return out[i].NonStreamMS < out[j].NonStreamMS
		}
		return out[i].Protocol < out[j].Protocol
	})
	return out
}

func mainTypeAction(p, main consts.Protocol) string {
	if p == main {
		return fmt.Sprintf("主类型保持 %s 即可", displayName(p))
	}
	return fmt.Sprintf("建议把主类型从 %s 改为 %s", displayName(main), displayName(p))
}

// endpointAction 给出「这个模型该勾哪些协议、主格式取谁」的落地动作。可一键写入该关联的
// 协议设置（next-do #1 的能力），只影响这一个模型；与 mainTypeAction 刻意分开，是因为
// 「供应商主类型该设成谁」与「这个模型该走哪些协议」是两个粒度，前者改的是全部模型的兜底路径。
func endpointAction(set []string, primary string) string {
	return fmt.Sprintf("建议把该模型的协议设置为勾选 %s，主格式 %s（可一键写入，只影响这个模型）",
		displayNames(set), displayName(consts.Protocol(primary)))
}

// displayNames 把协议集合转成句子里的称呼，顺序沿用入参（已是 consts.AllProtocols 序）。
func displayNames(set []string) string {
	names := make([]string, 0, len(set))
	for _, p := range set {
		names = append(names, displayName(consts.Protocol(p)))
	}
	return strings.Join(names, "、")
}

// allowSet 一键写入的勾选集合：双模式全过、且**没有被判为转译**的协议。
//
// 为什么排除转译路径：错误信封是别的家族形状，说明请求其实是上游网关在转；勾上等于让
// 客户端把这条路交给上游转，协议特有字段更容易丢，与报告里「不建议勾选这条」的建议互相矛盾。
// 指纹没采到（FamilyUnknown）不算转译证据，照常勾——「没测到」不等于「测过且不一致」。
// 顺序按 consts.AllProtocols 固定，保证同一组输入每次得到的集合逐字一致（便于断言与回读）。
func allowSet(usable []Input) []string {
	out := make([]string, 0, len(usable))
	for _, p := range consts.AllProtocols() {
		for _, in := range usable {
			if in.Protocol != p {
				continue
			}
			// Family 空串（调用方未填）与 FamilyUnknown 一样当作「没有转译证据」，照勾。
			if in.Family != "" && in.Family != FamilyUnknown && in.Family != FamilyOfProtocol(in.Protocol) {
				continue // 转译路径，不勾
			}
			out = append(out, string(in.Protocol))
			break
		}
	}
	return out
}

// pickPrimary 在「无法区分谁更原生」时挑主格式：在可用集合内按同一份证据口径排序
// （自己该有的原生字段回来了几个 → 当前主类型 → 耗时 → 协议名），集合外的不参与。
//
// 为什么不再「无条件保持当前主类型」：那样会出现「OpenAI Chat 缺 2 项、Responses 一项不缺，
// 却仍推荐 OpenAI Chat」的自相矛盾——同一张报告里两段话打架。字段完整度只是弱证据
// （各协议的原生字段集不同，缺一项也可能天生就不发），所以它排第一，但当前主类型仍是
// 打平时的偏好——与 rank 在别处的口径一致。
func pickPrimary(main consts.Protocol, set []string, usable []Input) string {
	if len(set) == 0 {
		return ""
	}
	candidates := make([]Input, 0, len(set))
	for _, in := range usable {
		if containsProtocol(set, in.Protocol) {
			candidates = append(candidates, in)
		}
	}
	if len(candidates) == 0 {
		// 理论上不会发生（set 由 usable 算出）；兜底取集合首条，保证主格式落在集合内。
		return set[0]
	}
	return string(rank(candidates, main)[0].Protocol)
}

// containsProtocol 判断协议是否在集合里（集合是字符串切片，与 JSON 回包同形状）。
func containsProtocol(set []string, p consts.Protocol) bool {
	for _, item := range set {
		if item == string(p) {
			return true
		}
	}
	return false
}

// translatedActions 对「可用但不像直通」的协议给不开启建议——开了等于把转换交给上游。
func translatedActions(usable []Input, winner consts.Protocol) []string {
	var out []string
	for _, in := range usable {
		if in.Protocol == winner {
			continue
		}
		family := FamilyOfProtocol(in.Protocol)
		if in.Family != FamilyUnknown && in.Family != family {
			out = append(out, fmt.Sprintf(
				"%s 可用但错误信封是 %s 家族的形状，像转译路径；不建议勾选这条协议（要走这条路时由 LLMux 自己转更可控）",
				displayName(in.Protocol), in.Family))
			continue
		}
		if len(in.Fields.Miss) > 0 {
			out = append(out, fmt.Sprintf(
				"%s 可用但缺 %d 项原生字段（%v），不确定字段是否被上游丢弃；配置前建议先按该协议实测一次真实请求",
				displayName(in.Protocol), len(in.Fields.Miss), in.Fields.Miss))
		}
	}
	return out
}

func partialActions(partial []Input) []string {
	var out []string
	for _, in := range partial {
		mode := "非流式"
		if in.NonStream != LegPassed {
			mode = "流式"
		}
		reason := "另一种模式不通"
		if in.Stream == LegNonCompliant {
			reason = "流式回了 SSE 但不合规（缺终止事件或无内容增量）"
		}
		out = append(out, fmt.Sprintf("%s 只支持 %s（%s），属于部分支持，不建议勾选这条协议",
			displayName(in.Protocol), mode, reason))
	}
	return out
}

func unusableActions(unusable []Input) []string {
	var out []string
	for _, in := range unusable {
		out = append(out, fmt.Sprintf("%s 本次两种模式都不通，不要勾选这条协议", displayName(in.Protocol)))
	}
	return out
}
