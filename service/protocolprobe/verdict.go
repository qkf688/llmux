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
	// 指纹（连不上、401、或形状不认识）。**同族是弱证据**，异族才是否决证据。
	Family Family
	// NativeHeaders 该协议家族的原生响应头（只作佐证，不参与判定）。
	NativeHeaders []string
	Fields        FieldReport
	// IDPrefix 成功响应 id 的前缀。只用于否决：前缀指向另一条已知协议时，说明这条路径
	// 的响应是翻译层拼出来的（见 IDPrefixConflict）。
	IDPrefix string
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

// evidence 一条路径「像不像原生直通」的证据。字段顺序即强度顺序，比较用字典序。
//
// 分档的唯一判据是**翻译层能不能伪造**：
//   - strong：不可推导的原生特征（Anthropic 的 content[].signature、Responses 的
//     encrypted_content）。翻译层拿不到源数据，造不出来，命中即近似证明直通；
//   - heads：原生响应头。网关普遍剥头，留着的多半是原样透传，但也可以伪造，故次之；
//   - dialect：错误信封方言。**排最后**——OpenAI 信封是兼容网关的默认形状
//     （见 ClassifyError），任何转译路径都顺手回它。把它当主要证据，正是「三条路
//     都可用时推荐了一条转译路径」的成因。
type evidence struct {
	strong  int
	heads   int
	dialect int
}

func (e evidence) beats(o evidence) bool {
	if e.strong != o.strong {
		return e.strong > o.strong
	}
	if e.heads != o.heads {
		return e.heads > o.heads
	}
	return e.dialect > o.dialect
}

func evidenceOf(in Input) evidence {
	return evidence{
		strong:  len(in.Fields.StrongHit),
		heads:   len(in.NativeHeaders),
		dialect: sameFamilyWeight(in),
	}
}

// sameFamilyWeight 同族证据的强度取决于「这个信封有多难被伪造」。
//
// Anthropic 信封要求顶层 type=error + 闭集内的 error.type（见 ClassifyError），必须
// 专门实现；OpenAI 信封则是兼容网关的默认形状，任何转译都顺手回它，近乎恒真。
// 两者等权就会让「OpenAI Responses 路径回 OpenAI 错误」凭空赢过「Anthropic 路径
// 回了一份完整的签名块」——这正是要修的错。
func sameFamilyWeight(in Input) int {
	if in.Family == "" || in.Family == FamilyUnknown || in.Family != FamilyOfProtocol(in.Protocol) {
		return 0
	}
	if in.Family == FamilyAnthropic {
		return 2
	}
	return 1
}

// translated 这条路径是否已被**明确证据**判为转译（而非原生）。
//
// 两项判据都只认「指向另一条已知协议」的矛盾，不认识就放行——「没测到」不等于
// 「测过且不一致」，把未知读成转译会误杀第三方实现（它们的信封与 id 前缀并不统一）。
func (in Input) translated() bool {
	if in.Family != "" && in.Family != FamilyUnknown && in.Family != FamilyOfProtocol(in.Protocol) {
		return true
	}
	_, conflict := IDPrefixConflict(in.Protocol, in.IDPrefix)
	return conflict
}

// Recommend 给总评。判据（按序，先命中先返回）：
//
//  1. 无协议双模式全过 → low：「没有一条协议同时通过非流式与流式」；
//  2. 只有一条可用     → high：唯一可用，无需分辨谁更原生；
//  3. 排除被证据判为转译的路径后只剩一条 → high：其余路径已**被证明**是转译；
//  4. 有且只有一条路径握有不可伪造的原生特征 → high；
//  5. 有多条握有强证据、或唯一赢家只靠弱证据 → medium，按 evidence 字典序点名；
//  6. 证据完全打平 → low：**无法区分**，不点名协议，只给当前配置的弱参考。
func Recommend(main consts.Protocol, inputs []Input) Recommendation {
	// Protocols 必须是空数组而非 nil：nil 切片序列化成 JSON null，前端拿到 null 再取
	// .length 就整页崩（协议全挂时正是这个形状——最容易踩到的路径）。
	rec := Recommendation{Protocols: []string{}, Confidence: ConfidenceLow, Notes: fixedNotes(), Actions: []string{}}

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

	// 候选 = 可用且没有被证据判为转译的路径。被排除的照样进不了勾选集合（见 allowSet）。
	set := allowSet(usable)
	rec.Protocols = set
	if len(set) == 0 {
		// 可用但全是转译路径：勾哪条都等于让上游网关转，宁可不给集合。
		rec.Reason = "所有可用协议都被证据判为转译路径（错误信封指向别的家族，或成功响应的 id 前缀是别的协议的）；" +
			"这里不给可勾选集合——勾哪条都等于把请求交给上游网关转。"
		rec.Summary = "无法区分：各可用路径都是转译出来的"
		rec.Actions = append(rec.Actions, translatedActions(usable, "")...)
		rec.Actions = append(rec.Actions, fmt.Sprintf("保持当前主类型 %s 不变，除非你另有把握", displayName(main)))
		return rec
	}

	candidates := make([]Input, 0, len(set))
	for _, in := range usable {
		if containsProtocol(set, in.Protocol) {
			candidates = append(candidates, in)
		}
	}
	ranked := rank(candidates, main)
	best := ranked[0]
	strictTop := len(ranked) == 1 || evidenceOf(ranked[0]).beats(evidenceOf(ranked[1]))

	switch {
	case !strictTop:
		// 证据打平：分不出谁更原生，只给当前配置的弱参考（不点名唯一主协议）。
		rec.Primary = pickPrimary(main, set, candidates)
		rec.Summary = "无法区分：各可用路径都拿不出指向自身的证据"
		rec.Reason = "各可用协议的证据强度相同（都没有不可伪造的原生特征，原生头与错误信封也打平），" +
			"无法判断谁更原生；这里不点名唯一主协议，但仍给出可勾选的集合。"
		rec.Actions = append(rec.Actions, translatedActions(usable, "")...)
		switch {
		case rec.Primary == string(main):
			rec.Actions = append(rec.Actions, fmt.Sprintf("保持当前主类型 %s 不变，除非你另有把握", displayName(main)))
		case containsProtocol(rec.Protocols, main):
			rec.Actions = append(rec.Actions, fmt.Sprintf("主格式建议取 %s；当前主类型 %s 也可用",
				displayName(consts.Protocol(rec.Primary)), displayName(main)))
		default:
			rec.Actions = append(rec.Actions, fmt.Sprintf("当前主类型 %s 本次不可用，主格式建议取 %s",
				displayName(main), displayName(consts.Protocol(rec.Primary))))
		}
		rec.Actions = append(rec.Actions, endpointAction(rec.Protocols, rec.Primary))
		return rec
	}

	rec.Primary = string(best.Protocol)
	strongCount := 0
	for _, in := range candidates {
		if len(in.Fields.StrongHit) > 0 {
			strongCount++
		}
	}

	switch {
	case len(candidates) == 1:
		// 其余可用路径都被证据判为转译，只剩这一条 → 它就是把请求按原协议送达的那条。
		rec.Confidence = ConfidenceHigh
		rec.Summary = fmt.Sprintf("建议主协议 %s", displayName(best.Protocol))
		rec.Reason = fmt.Sprintf("其余可用协议都被证据判为转译路径（错误信封是别的家族，或响应 id 前缀是别的协议的），" +
			"只剩它没有矛盾——它就是原样直通的那条。")
		rec.Actions = append(rec.Actions, mainTypeAction(best.Protocol, main), endpointAction(rec.Protocols, rec.Primary))
		rec.Actions = append(rec.Actions, translatedActions(usable, best.Protocol)...)
		return rec
	case strongCount == 1:
		rec.Confidence = ConfidenceHigh
		rec.Summary = fmt.Sprintf("建议主协议 %s", displayName(best.Protocol))
		rec.Reason = fmt.Sprintf("只有它的响应里出现了翻译层造不出来的原生特征（%s），其余可用路径一项都没有——"+
			"这类字段翻译层无法凭空生成，出现即说明这一层是原样直通。",
			strings.Join(best.Fields.StrongHit, "、"))
		rec.Actions = append(rec.Actions, mainTypeAction(best.Protocol, main), endpointAction(rec.Protocols, rec.Primary))
		rec.Actions = append(rec.Actions, translatedActions(usable, best.Protocol)...)
		return rec
	default:
		rec.Confidence = ConfidenceMedium
		rec.Summary = fmt.Sprintf("建议主协议 %s（区分度有限）", displayName(best.Protocol))
		if strongCount > 1 {
			rec.Reason = fmt.Sprintf("有多条协议都出现了不可伪造的原生特征，说明网关在每条路都做了原生封装；"+
				"按强证据数量排序取第一（%s），仅作弱推荐。", strings.Join(best.Fields.StrongHit, "、"))
		} else {
			rec.Reason = "没有任何一条路径出现不可伪造的原生特征（签名串 / 加密推理内容这类）；" +
				"只能按弱证据（原生响应头、错误信封方言）排出唯一第一名，仅作弱推荐。"
		}
		rec.Actions = append(rec.Actions, mainTypeAction(best.Protocol, main), endpointAction(rec.Protocols, rec.Primary))
		rec.Actions = append(rec.Actions, translatedActions(usable, best.Protocol)...)
		return rec
	}
}

// rank 按证据强度排序：字典序（强证据 → 原生头 → 信封方言）→ 是否当前主类型 → 单次耗时
// → 协议名。后三级只在证据完全打平时用：耗时是单次样本，绝不单独决定名次。
//
// 不再比较「命中字段总数」：三家协议该有的字段集本来就不同，跨协议比数量没有意义
// （Chat 的骨架比 Responses 少一项就能凭空落后），只能比各自命中的**强证据**。
func rank(candidates []Input, main consts.Protocol) []Input {
	out := append([]Input(nil), candidates...)
	sort.SliceStable(out, func(i, j int) bool {
		ei, ej := evidenceOf(out[i]), evidenceOf(out[j])
		if ei != ej {
			return ei.beats(ej)
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
// 为什么排除转译路径：错误信封是别的家族、或响应 id 前缀是别的协议的，说明请求其实是
// 上游网关在转；勾上等于让客户端把这条路交给上游转，协议特有字段更容易丢，与报告里
// 「不建议勾选这条」的建议互相矛盾。
// 指纹没采到（FamilyUnknown）与没有 id 不算转译证据，照常勾——「没测到」不等于
// 「测过且不一致」。顺序按 consts.AllProtocols 固定，保证同一组输入每次得到的集合
// 逐字一致（便于断言与回读）。
func allowSet(usable []Input) []string {
	out := make([]string, 0, len(usable))
	for _, p := range consts.AllProtocols() {
		for _, in := range usable {
			if in.Protocol != p {
				continue
			}
			if in.translated() {
				continue // 转译路径，不勾
			}
			out = append(out, string(in.Protocol))
			break
		}
	}
	return out
}

// pickPrimary 在「无法区分谁更原生」时挑主格式：在可用集合内按同一份证据口径排序
// （强证据 → 原生头 → 信封方言 → 当前主类型 → 耗时 → 协议名），集合外的不参与。
//
// 为什么不再「无条件保持当前主类型」：那样会出现「一条路径缺 2 项、另一条一项不缺，
// 却仍推荐前者」的自相矛盾。当前主类型只在证据打平时才作为偏好，与 rank 口径一致。
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

// translatedActions 对「不像直通」的协议给不开启建议——开了等于把转换交给上游。
func translatedActions(usable []Input, winner consts.Protocol) []string {
	var out []string
	for _, in := range usable {
		if in.Protocol == winner {
			continue
		}
		if target, conflict := IDPrefixConflict(in.Protocol, in.IDPrefix); conflict {
			out = append(out, fmt.Sprintf(
				"%s 可用但响应 id 是 %s 前缀（%s 的形状），声明与本机协议自相矛盾，像转译路径；"+
					"不建议勾选这条协议（要走这条路时由 LLMux 自己转更可控）",
				displayName(in.Protocol), in.IDPrefix, displayName(target)))
			continue
		}
		family := FamilyOfProtocol(in.Protocol)
		if in.Family != FamilyUnknown && in.Family != "" && in.Family != family {
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
