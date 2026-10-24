package shared

import (
	"strings"

	"github.com/qkf688/llmux/consts"
)

// 本文件是「响应 id 按目标协议重铸」的单一实现。
//
// 为什么必须重铸而不是原样透传：id 前缀是协议身份的一部分（OpenAI Chat 是
// `chatcmpl-`、Responses 是 `resp_`、Anthropic 是 `msg_`）。上游说 Anthropic 方言时
// 它的 id 是 `msg_...`，若原样塞进 OpenAI Chat 的响应体，就产出
// `{"object":"chat.completion","id":"msg_..."}` 这种自相矛盾形状——按前缀判断协议来源的
// 客户端/网关会误判，按 `chatcmpl-` 精确解析的代码会失败。多跳转换（A→Responses→B）
// 还会把前缀一层层叠上去（`chatcmpl-msg_...`、`msg_msg_...`），故重铸必须先剥旧前缀。

// responseIDPrefixes 协议形状 → 原生响应 id 前缀。**唯一来源**：transform 出站重铸与
// protocolprobe 的「前缀矛盾」判据都取这里，两处各写一份必漂移。
var responseIDPrefixes = map[consts.WireFormat]string{
	consts.FormatOpenAIChat:      "chatcmpl-",
	consts.FormatOpenAIResponses: "resp_",
	consts.FormatAnthropic:       "msg_",
}

// strippableResponseIDPrefixes 剥离旧前缀时的候选集，含连字符与下划线两形：
// OpenAI 官方用 `chatcmpl-`，部分兼容实现（与本仓旧夹具）用 `chatcmpl_`；
// Responses 官方 `resp_`，也存在 `resp-` 写法。剥不干净就会叠成双前缀。
var strippableResponseIDPrefixes = []string{
	"chatcmpl-", "chatcmpl_", "resp_", "resp-", "msg_", "msg-",
}

// ResponseIDPrefix 返回该协议形状的原生 id 前缀；未注册形状返回空串（调用方据此不重铸）。
func ResponseIDPrefix(format consts.WireFormat) string {
	return responseIDPrefixes[format]
}

// ResponseIDPrefixToken 返回前缀的**身份词**（去掉 `-` / `_` 分隔符），供「按 id 前缀判
// 协议来源」的判定使用：`msg_2026...` 的身份词是 `msg`，`chatcmpl-abc` 是 `chatcmpl`。
// 判定侧（protocolprobe）只关心身份词，不关心分隔符，故两者都从同一张表派生。
func ResponseIDPrefixToken(format consts.WireFormat) string {
	return strings.TrimRight(ResponseIDPrefix(format), "-_")
}

// BareResponseID 剥掉任意一层已知协议前缀，返回不含协议身份的裸 id。
//
// 循环剥离而非剥一次：多跳转换后可能出现 `chatcmpl-msg_xxx` 这类叠加形态，
// 只剥一层会留下另一半。剥到无可剥为止即幂等——同一 id 反复经过本函数结果不变。
func BareResponseID(id string) string {
	for {
		stripped := false
		for _, prefix := range strippableResponseIDPrefixes {
			if strings.HasPrefix(id, prefix) && len(id) > len(prefix) {
				id = id[len(prefix):]
				stripped = true
				break
			}
		}
		if !stripped {
			return id
		}
	}
}

// ResponseIDFor 把上游 id 重铸成目标协议的原生形状：剥掉旧前缀，再盖目标前缀。
//
// 空串原样返回（上游没给 id 时不该凭空造一个）；裸化后为空的退化值也原样返回——
// 宁可前缀不整，也不要产出一个只有前缀的空 id。未注册的目标形状同样原样返回，
// 避免「新协议漏配前缀」被静默当成空前缀。
func ResponseIDFor(format consts.WireFormat, upstreamID string) string {
	prefix := ResponseIDPrefix(format)
	if prefix == "" || upstreamID == "" {
		return upstreamID
	}
	bare := BareResponseID(upstreamID)
	if bare == "" {
		return upstreamID
	}
	return prefix + bare
}

// ResponseItemIDFor 重铸「Responses output 数组内的 message 项 id」。
//
// 与 ResponseIDFor 分开：message 项 id 的前缀固定是 `msg_`（Responses 的 item 级标识），
// 与响应级 `resp_` 不是同一个东西。混用会产出 `resp_xxx` 形状的 item id，客户端把
// output 回灌成下一轮 input 时按 `msg_` 找 item 就找不着。
func ResponseItemIDFor(upstreamID string) string {
	bare := BareResponseID(upstreamID)
	if bare == "" {
		return upstreamID
	}
	return "msg_" + bare
}
