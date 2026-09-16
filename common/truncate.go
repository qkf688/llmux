package common

import "unicode/utf8"

// TruncatedSuffix 截断标记单源：被字节上限截过的人读文本必须带它——消费方
// （日志页展示 / 排障）必须能区分「完整文本」与「尾部被切」。
const TruncatedSuffix = "…(truncated)"

// TruncateUTF8 按字节上限截断人读字符串，并回退到 UTF-8 rune 边界：
// s[:n] 直接按字节切会把多字节字符劈半，残字节进 JSON 后变 U+FFFD——
// 非 ASCII 必乱码（上游中文错误文案在 512B 处稳定出乱码尾的历史教训）。
// 超限才追加截断标记；maxBytes ≤ 0 时返回纯标记（调用方保证下限，属防御分支）。
func TruncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + TruncatedSuffix
}
