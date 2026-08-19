package openai

import "github.com/qkf688/llmux/service/transform/shared"

// ClaimedRequestKeys 返回本协议入站解析实际认领的顶层键集合。
//
// 直接反射入站 DTO 而不是手写清单：手写清单是 DTO 的第二份副本，加字段时必然
// 漏改成误报源。给 openAIChatCompletionRequest 加字段即自动纳入，无需改这里。
//
// 之所以做成包级函数而不是 FormatAdapter 的方法：各协议子包对外一律只暴露包级
// 函数，由 service/transform 的 funcAdapter 组装（见 adapters_register.go）。
func ClaimedRequestKeys() map[string]struct{} {
	return shared.ClaimedJSONKeys(openAIChatCompletionRequest{})
}
