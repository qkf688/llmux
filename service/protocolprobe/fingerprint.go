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
// （同一家的两套 API），故两者同族——这也是「Responses 路径回 OpenAI 错误信封」**不算**证据的原因。
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
type fieldSpec struct {
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
		{label: "system_fingerprint", path: "system_fingerprint"},
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
		{label: "error", path: "error"},
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

// FieldReport 一次响应按本协议「该有的原生特征」的命中情况。Miss 同样要报给用户：
// 「缺了什么」才是他判断该不该换上游的依据。
type FieldReport struct {
	Hit  []string `json:"hit"`
	Miss []string `json:"miss"`
}

// Total 该协议的原生特征总数（0 = 未注册的 wire format，调用方据此判「不可检」）。
func Total(format consts.WireFormat) int {
	return len(fieldSpecs[format])
}

// InspectFields 按协议核对原生特征。用 Exists() 判「键在不在」而非「值非空」——
// cache_read_input_tokens 为 0 是合法响应，但它出现本身就说明这一层没被转译抹掉。
func InspectFields(format consts.WireFormat, body []byte) FieldReport {
	report := FieldReport{Hit: []string{}, Miss: []string{}}
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
	return report
}

// IDPrefix 取响应 id 的家族前缀（msg / chatcmpl / resp 之类）。原生实现各有自己的 id
// 形状，转译层往往照着目标协议造同样的前缀，故它只是**佐证**，不参与判定。
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
