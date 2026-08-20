package shared

import (
	"bytes"
	"encoding/json"
)

// 本文件提供「一个字段可能是标量、也可能是对象/数组」这类多态入站字段的形状守卫。
//
// 为什么不直接 json.Unmarshal 到目标 struct：协议里大量字段是 string｜object 或
// string｜array 二选一（anthropic 的 system / tool_choice、openai 的 stop），而
// 标准库对形状不符只会返回错误，调用方若据此报错就把「客户端写法不标准」升级成
// 400——与 optional_fields.go 里那套宽容语义相悖。先守卫形状、不符就当没传，才能
// 让「解析不出来 = 视为未设置」这一条贯穿整个入站层。
//
// 放在 shared 而非各协议子包：anthropic 与 openai 入站都需要同一套分派语义，复制
// 两份必然漂移（AGENTS.md 4.4 DRY）。只依赖标准库，不反向依赖业务包。

// DecodeJSONObject 仅当 raw 是 JSON 对象时才解进 target，成功返回 true。
//
// 先判首字节而不是直接试 Unmarshal：`"abc"` 解进 struct 会失败、但 `123` 解进
// 某些目标类型可能意外成功，靠错误判形状不可靠。守卫住 '{' 让「不是对象」这件事
// 在类型层面就被拒掉。
func DecodeJSONObject(raw json.RawMessage, target any) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	return json.Unmarshal(raw, target) == nil
}

// RawString 在 raw 是 JSON 字符串时返回其值，否则返回 ("", false)。
//
// 用于 string｜object / string｜array 的第一分支试探：命中就走标量语义，未命中
// 交给后续的对象/数组分支，两者都不命中即视为未设置。
func RawString(raw json.RawMessage) (string, bool) {
	var value string
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return value, true
}
