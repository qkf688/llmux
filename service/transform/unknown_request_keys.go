package transform

import (
	"errors"
	"fmt"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/service/transform/anthropic"
	"github.com/qkf688/llmux/service/transform/openai"
	"github.com/qkf688/llmux/service/transform/responses"
	"github.com/qkf688/llmux/service/transform/shared"
)

// 本文件把 shared 的键集合原语接成「给一个入站 style 和它的原始 body，答出哪些
// 顶层键本网关根本没解析」的可消费入口。消费方是低频诊断路径（日志详情接口），
// 见 handler/logs。
//
// 为什么不把认领键做成 FormatAdapter 的第五个方法（ISP）：认领键是**入站**解析的
// 性质，而 FormatAdapter 同时管出站；塞进接口会让「这个协议答不答得了」变成运行时
// 约定而非编译期事实。做成独立的可选注册表后，某个 style 支持与否只看它在下面的
// init 里有没有那一行（OCP）。

// ErrClaimedKeysUnsupported 表示该 style 没有可反射的入站请求 DTO，无法计算未知字段。
//
// 刻意用哨兵错误而非返回空切片：空切片与「确实没有未知字段」不可区分，消费方会
// 把「查不了」显示成「已检查、无问题」，即假阴性。
var ErrClaimedKeysUnsupported = errors.New("transform: inbound unknown-field detection is not supported for this style")

// ErrMismatchedKeysUnsupported 表示该 style 的入站 DTO 不使用 shared 的三态容器，
// 无从判断「键被认领但类型不对而丢弃」。
//
// 与 ErrClaimedKeysUnsupported 分开而不复用：两种检测的支持面**不重合**——
// openai-res 的 DTO 能反射认领键（支持未认领检测），但它的顶层字段是裸类型/指针，
// 类型不匹配会让整条请求解析报错而非静默丢弃，本就没有可报告的静默。混用一个哨兵
// 会让消费方分不清是哪项能力缺失。
var ErrMismatchedKeysUnsupported = errors.New("transform: inbound type-mismatch detection is not supported for this style")

// claimedRequestKeysByStyle 是**入站客户端 style** → 入站认领键集合的分派表。
//
// 键刻意保持 consts.Style（而不是 consts.WireFormat）：本表只服务入站诊断，传出站协议
// 会把「转换时改名的字段」全部误报成未知。用 Style 做键之后，出站侧的 consts.WireFormat
// 值在编译期就传不进来——这条原本只写在注释里的告诫现在由类型系统兜住。
//
// 刻意**不**在未命中时回退到默认协议：拿 openai 的键集合去查 anthropic 的 body，
// 会把 system / stop_sequences 这类正常键整片误报，正是
// TestUnknownTopLevelKeys_OutboundRenameIsNotUnknown 钉死要防的退化。
var claimedRequestKeysByStyle = map[consts.Style]func() map[string]struct{}{}

// mismatchedRequestKeysByStyle 是 style → 类型不匹配键计算函数的分派表。
//
// 与 claimedRequestKeysByStyle 分成两张表而不是一张表两个字段：两者的支持面不同
// （见 ErrMismatchedKeysUnsupported），合成一张表就得允许其中一半为 nil，等于把
// 「未注册」和「注册了但为空」混在一起。
var mismatchedRequestKeysByStyle = map[consts.Style]func([]byte) ([]string, error){}

func registerClaimedRequestKeys(style consts.Style, keys func() map[string]struct{}) {
	if style == "" {
		panic("transform: claimed request keys style must not be empty")
	}
	if keys == nil {
		panic(fmt.Sprintf("transform: claimed request keys func for %q must not be nil", style))
	}
	if _, exists := claimedRequestKeysByStyle[style]; exists {
		panic(fmt.Sprintf("transform: claimed request keys for %q are already registered", style))
	}
	claimedRequestKeysByStyle[style] = keys
}

func registerMismatchedRequestKeys(style consts.Style, mismatched func([]byte) ([]string, error)) {
	if style == "" {
		panic("transform: mismatched request keys style must not be empty")
	}
	if mismatched == nil {
		panic(fmt.Sprintf("transform: mismatched request keys func for %q must not be nil", style))
	}
	if _, exists := mismatchedRequestKeysByStyle[style]; exists {
		panic(fmt.Sprintf("transform: mismatched request keys for %q are already registered", style))
	}
	mismatchedRequestKeysByStyle[style] = mismatched
}

func init() {
	registerClaimedRequestKeys(consts.StyleOpenAI, openai.ClaimedRequestKeys)
	registerClaimedRequestKeys(consts.StyleOpenAIRes, responses.ClaimedRequestKeys)
	registerClaimedRequestKeys(consts.StyleAnthropic, anthropic.ClaimedRequestKeys)

	// openai-res 刻意缺席：`ResponsesRequest` 用裸类型/指针字段，类型不匹配时整条
	// 请求解析失败（客户端拿到显式错误），不存在需要暴露的静默丢弃。
	registerMismatchedRequestKeys(consts.StyleOpenAI, openai.MismatchedRequestKeys)
	registerMismatchedRequestKeys(consts.StyleAnthropic, anthropic.MismatchedRequestKeys)
}

// UnknownRequestKeys 返回 rawBody 里出现、但 style 对应的入站 DTO 未认领的顶层键，已排序。
//
// style 必须是**入站**协议（即客户端打进来时用的 style，对应 ChatLog.Style）。传出站
// 协议会把「转换时改名的字段」全部误报成未知——比如 openai 的 max_tokens 出到
// Responses 叫 max_output_tokens，用 openai-res 的键集合查就会报 max_tokens 未知。
//
// 结果语义是「网关没解析这些键」，不等于「转换过程中丢了这些键」：passthrough 路径
// （style == provider type，见 service/chat/chat_attempt_request.go）请求根本不进
// transform，这些键实际会原样透传给上游。
//
// 未注册的 style 返回 ErrClaimedKeysUnsupported；rawBody 不是 JSON 对象则透传解析错误。
// 只看顶层，messages[].xxx 之类的嵌套键不在范围内。
func UnknownRequestKeys(style consts.Style, rawBody []byte) ([]string, error) {
	claimed, ok := claimedRequestKeysByStyle[style]
	if !ok {
		return nil, fmt.Errorf("style %q: %w", style, ErrClaimedKeysUnsupported)
	}
	return shared.UnknownTopLevelKeys(rawBody, claimed())
}

// MismatchedRequestKeys 返回 rawBody 里出现、被 style 对应的入站 DTO **认领了、但值
// 类型不匹配而被静默丢弃**的顶层键，已排序。
//
// 与 UnknownRequestKeys 是互补的两类静默：那个答「DTO 里根本没这个键」，这个答
// 「DTO 有这个键，但 shared 容器把类型不匹配当成没传」。后者对客户端更隐蔽——
// `{"temperature":"0.5"}` 会 200 通过、参数却没生效，而日志里的入站 body 明明写着
// 那个值。宽容度本身是刻意的（见 shared/optional_fields.go），所以这里的解法是让
// 静默可见，不是改成 400。
//
// style 语义与 UnknownRequestKeys 相同（必须传**入站** style）。未注册的 style 返回
// ErrMismatchedKeysUnsupported；只看顶层，且只覆盖带三态的容器字段
// （`json.RawMessage` / `RawArray` 类型的字段不在范围内，见 shared 侧注释）。
func MismatchedRequestKeys(style consts.Style, rawBody []byte) ([]string, error) {
	mismatched, ok := mismatchedRequestKeysByStyle[style]
	if !ok {
		return nil, fmt.Errorf("style %q: %w", style, ErrMismatchedKeysUnsupported)
	}
	return mismatched(rawBody)
}
