package transform

import (
	"errors"
	"fmt"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/service/transform/openai"
	"github.com/qkf688/llmux/service/transform/responses"
	"github.com/qkf688/llmux/service/transform/shared"
)

// 本文件把 shared 的键集合原语接成「给一个入站 style 和它的原始 body，答出哪些
// 顶层键本网关根本没解析」的可消费入口。消费方是低频诊断路径（日志详情接口），
// 见 handler/logs。
//
// 为什么不把认领键做成 FormatAdapter 的第五个方法（ISP）：Anthropic 入站没有请求
// DTO 可反射，进接口就得逼它空实现或返 nil，调用方反而要靠约定判断「返回空到底
// 是没有未知字段还是这个协议答不了」。做成独立的可选注册表后，「未注册 = 不支持」
// 是编译期事实——想支持 anthropic 只需在它有了请求 DTO 之后加一行注册（OCP），
// 本文件与 FormatAdapter 都不用动。

// ErrClaimedKeysUnsupported 表示该 style 没有可反射的入站请求 DTO，无法计算未知字段。
//
// 刻意用哨兵错误而非返回空切片：空切片与「确实没有未知字段」不可区分，消费方会
// 把「查不了」显示成「已检查、无问题」，即假阴性。
var ErrClaimedKeysUnsupported = errors.New("transform: inbound unknown-field detection is not supported for this style")

// claimedRequestKeysByStyle 是 style → 入站认领键集合的分派表。
//
// 刻意**不**像 getAdapterOrDefault 那样在未命中时回退到默认协议：拿 openai 的键
// 集合去查 anthropic 的 body，会把 system / stop_sequences 这类正常键整片误报，
// 正是 TestUnknownTopLevelKeys_OutboundRenameIsNotUnknown 钉死要防的退化。
var claimedRequestKeysByStyle = map[string]func() map[string]struct{}{}

func registerClaimedRequestKeys(style string, keys func() map[string]struct{}) {
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

func init() {
	registerClaimedRequestKeys(consts.StyleOpenAI, openai.ClaimedRequestKeys)
	registerClaimedRequestKeys(consts.StyleOpenAIRes, responses.ClaimedRequestKeys)
	// consts.StyleAnthropic 刻意缺席，理由见 service/transform/anthropic/adapter.go。
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
func UnknownRequestKeys(style string, rawBody []byte) ([]string, error) {
	claimed, ok := claimedRequestKeysByStyle[style]
	if !ok {
		return nil, fmt.Errorf("style %q: %w", style, ErrClaimedKeysUnsupported)
	}
	return shared.UnknownTopLevelKeys(rawBody, claimed())
}
