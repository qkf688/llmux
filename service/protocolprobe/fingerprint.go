// Package protocolprobe 是「协议支持检测」的**判定内核**：把上游实际回了什么，折算成
// 「这条协议能不能用 / 像不像直通 / 字段全不全」，再据此给出带置信度的建议。
//
// 分层纪律（与 credprobe / transform 同款口径）：
//   - 本包**不做任何 IO**，只吃「已经打回来的响应体与响应头」→ 判定规则全部可表驱动测试；
//   - 打上游的编排（并发两段、每协议三条探针、SSE 折叠）在 handler/testapi；
//   - 判「上游算不算成功」仍由 credprobe 走 common.IsPassingStatus，本包只判「回来了的东西像什么」。
//
// 为什么「原始协议」只能逼近、不能断言：协议互转网关对多条协议路径都能回合法响应，
// 通不通与是不是原生是两件事。故本包输出的是**证据**（错误信封家族 / 原生响应头 /
// 字段命中）+ **带置信度的建议**，证据不足时明说「无法区分」而不是硬给结论。
package protocolprobe

import (
	"net/http"
	"sort"
	"strings"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/service/transform/shared"
	"github.com/tidwall/gjson"
)

// Family 是「一份上游响应/错误出自哪一族 API」的归属判据。网关按自家方言重写错误时，
// 家族会与协议自身不符——这正是判定「这条路径有没有被转译」的核心证据。
type Family string

const (
	FamilyUnknown   Family = "unknown"
	FamilyOpenAI    Family = "openai"
	FamilyAnthropic Family = "anthropic"
	FamilyGoogle    Family = "google"
)

// FamilyOfProtocol 给协议期望的家族。Responses 与 Chat Completions 共用 OpenAI 家族
// （同一家的两套 API）——但**同族不等于直通**：OpenAI 信封是兼容网关的默认形状，
// 任何转译路径都顺手回它。故它能作否决（回的是 Anthropic/Google 信封 → 必是转译），
// 不能作正面证明；正面证明见 fieldSpec.strong 与 IDPrefixConflict。
func FamilyOfProtocol(protocol consts.Protocol) Family {
	switch protocol {
	case consts.ProtocolAnthropic:
		return FamilyAnthropic
	case consts.ProtocolOpenAI, consts.ProtocolResponses:
		return FamilyOpenAI
	default:
		return FamilyUnknown
	}
}

// anthropicErrorTypes 是 Anthropic 错误对象的 type 闭集。命中即判 Anthropic 家族——
// 网关若统一改写成 OpenAI 形状，这里就落空，正好把「被转译」暴露出来。
var anthropicErrorTypes = map[string]bool{
	"invalid_request_error": true,
	"authentication_error":  true,
	"permission_error":      true,
	"not_found_error":       true,
	"request_too_large":     true,
	"rate_limit_error":      true,
	"api_error":             true,
	"overloaded_error":      true,
	"timeout_error":         true,
}

// ClassifyError 把一份**错误响应**归家族：读 error 对象与顶层 type 判定，返回家族与
// 人读的错误类型/摘要。body 非 JSON 或形状不认识时返回 FamilyUnknown（不猜）。
//
// 判据与易踩点（都踩过）:
//
//  1. **两家共用 `invalid_request_error`**：OpenAI 的 error.type 与 Anthropic 同名，
//     只看 error.type 会把 OpenAI 错误判成 Anthropic。故 Anthropic 的判据是**信封形状**——
//     顶层带 `"type":"error"`（OpenAI/Google 信封都没有这个顶层键）；退化情形
//     （网关剥掉顶层 type）才退到「error.type 在 Anthropic 闭集内且不带 param/code」。
//  2. **Google 的签名是 `error.status` 那个大写串**，不是数字 `code`：大量兼容网关也往
//     error.code 里塞数字（500/400），只看数字会把自家方言判成 Google。
//  3. `{"error":{"message":...}}` 归 OpenAI 家族：它就是兼容网关的事实标准信封（含
//     new_api_error 这类自家命名）。这意味着「openai 同族」是**弱于原生**的证据——
//     是否真直通由 Recommend 里的「是否与自身协议同族 + 字段命中」一起判。
func ClassifyError(body []byte) (family Family, errorType string, message string) {
	if !gjson.ValidBytes(body) {
		return FamilyUnknown, "", ""
	}
	errObj := gjson.GetBytes(body, "error")
	message = gjson.GetBytes(body, "error.message").String()
	if !errObj.Exists() {
		if gjson.GetBytes(body, "type").String() == "error" && message != "" {
			return FamilyAnthropic, firstNonEmpty(
				gjson.GetBytes(body, "error.type").String(),
				gjson.GetBytes(body, "error.code").String(),
			), message
		}
		return FamilyUnknown, "", message
	}
	// Google：只认大写 status 串（判据 2）。
	if status := gjson.GetBytes(body, "error.status"); status.Type == gjson.String {
		return FamilyGoogle, status.String(), message
	}
	// 展示给用户的错误标识取**更具体**的那个：OpenAI 的 type 常是笼统的
	// invalid_request_error，具体原因在 code（model_not_found）；网关自命名
	// （new_api_error）时反过来 type 更具体，code 只是数字。故按此优先级取。
	errorType = displayErrorType(body)
	if gjson.GetBytes(body, "type").String() == "error" {
		return FamilyAnthropic, errorType, message
	}
	if anthropicErrorTypes[gjson.GetBytes(body, "error.type").String()] &&
		!gjson.GetBytes(body, "error.param").Exists() &&
		!gjson.GetBytes(body, "error.code").Exists() {
		return FamilyAnthropic, errorType, message
	}
	if message != "" {
		return FamilyOpenAI, errorType, message
	}
	return FamilyUnknown, "", message
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// displayErrorType 取给人看的错误标识：非笼统的 error.type 优先，否则退回 error.code
// （Anthropic 无 code、Google 已在归家族时早退，两条不受影响）。
func displayErrorType(body []byte) string {
	errType := gjson.GetBytes(body, "error.type").String()
	code := gjson.GetBytes(body, "error.code").String()
	if errType != "" && errType != "invalid_request_error" {
		return errType
	}
	return firstNonEmpty(code, errType)
}

// NativeHeaders 挑出「属于该家族厂商」的响应头。**有**它是直通的强证据（网关普遍剥头，
// 留着的多半是原样透传）；**没有**只是弱证据，故本函数只回名字、不给结论。
func NativeHeaders(header http.Header, family Family) []string {
	if header == nil || family == FamilyUnknown {
		return nil
	}
	var matches []string
	for name := range header {
		lower := strings.ToLower(name)
		switch family {
		case FamilyAnthropic:
			if strings.HasPrefix(lower, "anthropic-") || lower == "request-id" {
				matches = append(matches, lower)
			}
		case FamilyOpenAI:
			if strings.HasPrefix(lower, "openai-") || lower == "x-request-id" {
				matches = append(matches, lower)
			}
		case FamilyGoogle:
			if strings.HasPrefix(lower, "x-goog-") {
				matches = append(matches, lower)
			}
		}
	}
	// map 遍历无序：排序后输出，保证同一份响应每次给同样的列表（前端与测试都依赖稳定性）。
	sort.Strings(matches)
	return matches
}

// fieldSpec 是「这条协议正常实现时，响应里应当出现的原生特征」一项。
//
// 为什么按协议各列一份、而不是跨协议比字段多少：三条协议的关键字段集本就不同
// （cache_* 只有 Anthropic 有），跨协议比数量没有意义。本包比的是
// 「**它自己该有的，回来了几个**」，命中项逐条报给用户看。
//
// 这里列的**只有骨架字段**（object / type / stop_reason / usage.*）：任何翻译层都必须
// 亲手写出来，命中与否与是否原生无关，只能当完整度看，不能当直通证据。
// 能证明直通的字段另列在 strongSpecs，两者判据不同、不能混在一张表里。
type fieldSpec struct {
	label string
	path  string
}

// strongSpec 是「翻译层通常产不出来」的字段——唯一能正面证明直通的证据。
//
// 与 fieldSpecs 的分野不是重要程度，而是**缺席能否算证据**：
// 这类字段是否出现取决于模型与请求参数（要不要 thinking、要不要 include、上游版本），
// 所以只能「有才算证据，没有不算」——绝不能进 Miss 列表，否则每一份正常的原生响应
// 都会显示「缺 N 项原生字段」，把用户引去排查一个不存在的上游丢字段问题。
type strongSpec struct {
	label string
	path  string
}

var fieldSpecs = map[consts.WireFormat][]fieldSpec{
	consts.FormatOpenAIChat: {
		{label: "object", path: "object"},
		{label: "finish_reason", path: "choices.0.finish_reason"},
		{label: "usage.prompt_tokens", path: "usage.prompt_tokens"},
		{label: "usage.completion_tokens", path: "usage.completion_tokens"},
		{label: "usage.prompt_tokens_details", path: "usage.prompt_tokens_details"},
		{label: "usage.completion_tokens_details", path: "usage.completion_tokens_details"},
		{label: "service_tier", path: "service_tier"},
	},
	consts.FormatOpenAIResponses: {
		{label: "object", path: "object"},
		{label: "status", path: "status"},
		{label: "output[].type", path: "output.0.type"},
		{label: "usage.input_tokens", path: "usage.input_tokens"},
		{label: "usage.output_tokens", path: "usage.output_tokens"},
		{label: "usage.input_tokens_details", path: "usage.input_tokens_details"},
		{label: "usage.output_tokens_details", path: "usage.output_tokens_details"},
	},
	consts.FormatAnthropic: {
		{label: "type", path: "type"},
		{label: "content[].type", path: "content.0.type"},
		{label: "stop_reason", path: "stop_reason"},
		{label: "stop_sequence", path: "stop_sequence"},
		{label: "usage.input_tokens", path: "usage.input_tokens"},
		{label: "usage.output_tokens", path: "usage.output_tokens"},
		{label: "usage.cache_creation_input_tokens", path: "usage.cache_creation_input_tokens"},
		{label: "usage.cache_read_input_tokens", path: "usage.cache_read_input_tokens"},
	},
}

// strongSpecs 各协议的「翻译层通常产不出来」字段。缺席是常态，只有命中才有意义。
//
// 逐项说明为什么是它、以及为什么没把别的塞进来：
//   - Chat `system_fingerprint`：后端配置指纹，翻译层要凭空编一个既无意义也易被识破。
//     **注意它不是绝对的**——部分自建兼容服务（vLLM 一类）支持自定义该值，故它是
//     「转译层通常不产出」而非「物理上不可能」；但作为弱于签名的证据仍有区分力。
//   - Anthropic `content[].signature`：上游对 thinking 块的不透明签名，翻译层拿不到源
//     数据。**只在响应含 thinking 块时出现**（模型默认思考与否决定），故只算机会性证据。
//   - Responses `encrypted_content`：**已移除**。官方只在请求带
//     `include:["reasoning.encrypted_content"]`（或 `store:false`）时返回，而协议探针的
//     请求体不带这两个参数 → 结构上永不出现。留着等于给每条正常响应挂一条恒假的
//     「缺 encrypted_content」，且它作为强证据的分支线上永不触发。
var strongSpecs = map[consts.WireFormat][]strongSpec{
	consts.FormatOpenAIChat: {
		{label: "system_fingerprint", path: "system_fingerprint"},
	},
	consts.FormatOpenAIResponses: {},
	consts.FormatAnthropic: {
		{label: "content[].signature", path: "content.0.signature"},
	},
}

// FieldReport 一次响应按本协议「该有的原生特征」的命中情况。Miss 同样要报给用户：
// 「缺了什么」才是他判断该不该换上游的依据。
//
// StrongHit 与 Hit/Miss **来源不同、互不重叠**：
//   - Hit / Miss 来自 fieldSpecs（骨架字段，该有就有），用于展示完整度；
//   - StrongHit 来自 strongSpecs（机会性字段，缺席是常态），**只记命中不记缺失**。
//
// 判定只看 StrongHit——把骨架字段当直通证据，正是本包此前把转译路径推成主协议的成因。
type FieldReport struct {
	Hit       []string `json:"hit"`
	Miss      []string `json:"miss"`
	StrongHit []string `json:"strong_hit"`
}

// Total 该协议「该有」的骨架特征总数（0 = 未注册的 wire format，调用方据此判「不可检」）。
func Total(format consts.WireFormat) int {
	return len(fieldSpecs[format])
}

// InspectFields 按协议核对原生特征。用 Exists() 判「键在不在」而非「值非空」——
// cache_read_input_tokens 为 0 是合法响应，但它出现本身就说明这一层没被转译抹掉。
//
// strongSpecs 单独一轮扫描：缺席**不进 Miss**（见 strongSpec 注释——那类字段出不出现
// 取决于模型与请求参数，把常态缺席报成「缺字段」会让每份正常响应都挂一条假缺陷）。
func InspectFields(format consts.WireFormat, body []byte) FieldReport {
	report := FieldReport{Hit: []string{}, Miss: []string{}, StrongHit: []string{}}
	if !gjson.ValidBytes(body) {
		return report
	}
	for _, spec := range fieldSpecs[format] {
		if gjson.GetBytes(body, spec.path).Exists() {
			report.Hit = append(report.Hit, spec.label)
			continue
		}
		report.Miss = append(report.Miss, spec.label)
	}
	for _, spec := range strongSpecs[format] {
		if gjson.GetBytes(body, spec.path).Exists() {
			report.StrongHit = append(report.StrongHit, spec.label)
		}
	}
	return report
}

// IDPrefix 取响应 id 的家族前缀（msg / chatcmpl / resp 之类）。原生实现各有自己的 id
// 形状，转译层往往照着目标协议造同样的前缀，故它**只作佐证与否决**，不单独作正面证据。
func IDPrefix(body []byte) string {
	if !gjson.ValidBytes(body) {
		return ""
	}
	id := gjson.GetBytes(body, "id").String()
	if id == "" {
		return ""
	}
	if i := strings.IndexAny(id, "_-"); i > 0 {
		return id[:i]
	}
	return id
}

// protocolIDPrefix 各协议原生响应的 id 前缀（身份词，不含分隔符）。
//
// **单一来源取自 transform/shared**：出站重铸与「前缀矛盾」判定必须用同一张表。
// 两处各写一份时，改了重铸口径却忘了判定侧，就会出现「LLMux 自己产出的 id 被自己的
// 检测判成转译证据」这类自伤（2026-10-24 修的第一版正是这个状态）。
var protocolIDPrefix = map[consts.Protocol]string{
	consts.ProtocolOpenAI:    shared.ResponseIDPrefixToken(consts.FormatOpenAIChat),
	consts.ProtocolResponses: shared.ResponseIDPrefixToken(consts.FormatOpenAIResponses),
	consts.ProtocolAnthropic: shared.ResponseIDPrefixToken(consts.FormatAnthropic),
}

// IDPrefixConflict 判「响应 id 前缀是否指向**另一条**已知协议」。
//
// 这是比字段命中更硬的证据：`{"object":"chat.completion"}` 配 `msg_...` 这种组合
// 只可能是翻译层拼出来的——它按目标协议写了骨架，却漏了源协议的 id 形状。
// 反过来，前缀不认识（"" / 自定义）一律不算冲突：第三方实现的前缀并不统一，
// 拿未知当转译会误杀。
//
// **判定用身份词等值，不做前缀包含**：`prefix` 已是 IDPrefix 切段后的整段身份词，
// 用 HasPrefix 会把「身份词恰好是另一个身份词的超串」误判成已知协议——
// `msgpack-1` 会被当成 Anthropic 形状、`response-9c1d` 会被当成 Responses 形状。
// 三个身份词（chatcmpl / resp / msg）两两互不为前缀，故等值判定不丢召回：
// 真实的 `msg_…` / `resp_…` / `chatcmpl-…` 仍各自命中，而第三方自定义前缀全部放行。
//
// **适用范围仅限上游返回的原始响应**：本仓 transform 出站已按目标协议重铸 id
// （见 shared.ResponseIDFor），故 LLMux 自产响应不会命中本判据。若将来把这个函数
// 复用到自产响应上，必须先剥掉重铸痕迹，否则会误判自己的输出。
func IDPrefixConflict(protocol consts.Protocol, prefix string) (consts.Protocol, bool) {
	if prefix == "" {
		return "", false
	}
	for _, p := range consts.AllProtocols() {
		if p == protocol {
			continue
		}
		if want := protocolIDPrefix[p]; want != "" && prefix == want {
			return p, true
		}
	}
	return "", false
}
