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

// Recommendation 一次检测的总评。Protocol 为空 = 不点名任何协议（证据不足时的
// 「无法区分」，共识口径：宁可不给，也不给一个自信的错答案）。
type Recommendation struct {
	Protocol   string     `json:"protocol"`
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
		"结论按本模型测出，协议端点开关在供应商级，改动会影响该供应商下所有模型",
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
			rec.Reason = "只有协议处于「部分支持」状态（单模式可用、双模式不通），不建议开启端点。"
		}
		rec.Actions = append(rec.Actions, "本轮没有可勾选的协议端点（两种模式都没通过）")
		return rec
	case len(usable) == 1:
		only := usable[0]
		rec.Protocol = string(only.Protocol)
		rec.Confidence = ConfidenceHigh
		rec.Summary = fmt.Sprintf("只有 %s 这条协议可用", displayName(only.Protocol))
		rec.Reason = "其余协议至少有一种模式不通；唯一可用的协议就是最优解，无需再比谁更原生。"
		rec.Actions = append(rec.Actions, mainTypeAction(only.Protocol, main), endpointAction(only.Protocol))
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
		rec.Protocol = string(best.Protocol)
		rec.Confidence = ConfidenceMedium
		if len(best.Fields.Miss) == 0 {
			rec.Confidence = ConfidenceHigh
		}
		rec.Summary = fmt.Sprintf("建议主协议 %s", displayName(best.Protocol))
		rec.Reason = fmt.Sprintf("只有它的错误信封是自身协议形状（%s 同族），其余可用协议回的是别的家族——"+
			"说明其余路径被网关转译，协议特有字段最容易在那里丢；它的原生字段命中 %d/%d。",
			FamilyOfProtocol(best.Protocol), len(best.Fields.Hit), len(best.Fields.Hit)+len(best.Fields.Miss))
		rec.Actions = append(rec.Actions, mainTypeAction(best.Protocol, main), endpointAction(best.Protocol))
		rec.Actions = append(rec.Actions, translatedActions(usable, best.Protocol)...)
		return rec
	case len(native) > 1:
		best := rank(native, main)[0]
		rec.Protocol = string(best.Protocol)
		rec.Confidence = ConfidenceMedium
		rec.Summary = fmt.Sprintf("建议主协议 %s（区分度有限）", displayName(best.Protocol))
		rec.Reason = fmt.Sprintf("有多条协议的错误信封都与自身协议同族，说明网关在每条路都做了原生封装，"+
			"分不出后端家族；按原生字段命中数（%d/%d）排序取第一，仅作弱推荐。",
			len(best.Fields.Hit), len(best.Fields.Hit)+len(best.Fields.Miss))
		rec.Actions = append(rec.Actions, mainTypeAction(best.Protocol, main), endpointAction(best.Protocol))
		return rec
	default:
		rec.Summary = "无法区分：各可用协议的响应都没有指向自身协议"
		rec.Reason = "所有可用协议的错误信封都没有采到自身家族的形状（可能都被网关统一改写，" +
			"也可能指纹请求未采到），无法判断谁更原生；这里不给推荐协议。"
		rec.Actions = append(rec.Actions,
			fmt.Sprintf("保持当前主类型 %s 不变，除非你另有把握", displayName(main)),
			endpointListAction(usable))
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

// endpointAction 给出「给这个模型勾哪条协议端点」的落地动作。主类型是供应商级配置，
// 端点勾选才是按模型生效的那一层（next-do #1 的能力）；不写成同一句话，是因为
// 「主类型该设成谁」与「这个模型该勾哪条端点」是两件事，前者改的是全部模型的兜底路径。
func endpointAction(p consts.Protocol) string {
	return fmt.Sprintf("建议该模型勾选 %s 协议端点", displayName(p))
}

// endpointListAction 用于「无法区分谁更原生」的分支：可用协议都列出来，但不替用户拍板。
// 附实测提示，是因为「可用」不等于「字段没被转译」，勾选前值得按该协议真跑一次。
func endpointListAction(usable []Input) string {
	names := make([]string, 0, len(usable))
	for _, in := range usable {
		names = append(names, displayName(in.Protocol))
	}
	return fmt.Sprintf("该模型可勾选这些协议端点：%s（本次无法判断哪个最原生，勾选前建议先按该协议实测一次真实请求）",
		strings.Join(names, "、"))
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
				"%s 可用但错误信封是 %s 家族的形状，像转译路径；不建议为它开端点（要走这条路时由 LLMux 自己转更可控）",
				displayName(in.Protocol), in.Family))
			continue
		}
		if len(in.Fields.Miss) > 0 {
			out = append(out, fmt.Sprintf(
				"%s 可用但缺 %d 项原生字段（%v），不确定字段是否被上游丢弃；开端点前建议先按该协议实测一次真实请求",
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
		out = append(out, fmt.Sprintf("%s 只支持 %s（%s），属于部分支持，不建议开启端点",
			displayName(in.Protocol), mode, reason))
	}
	return out
}

func unusableActions(unusable []Input) []string {
	var out []string
	for _, in := range unusable {
		out = append(out, fmt.Sprintf("%s 本次两种模式都不通，不要开启端点", displayName(in.Protocol)))
	}
	return out
}
