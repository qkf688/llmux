package common

import (
	"fmt"
	"unicode/utf8"
)

// TruncatedSuffix 尾部截断标记：单字段只留头部的截断语义（上限小、只关心
// 「开头是什么」的场景）。头尾式标记见 TruncateHeadTail。
const TruncatedSuffix = "…(truncated)"

// TruncateHeadTail 头尾式截断标记单源：全项目一切「超限被切、头尾保留」的日志文本
// 统一带它，消费方（日志页展示 / 排障）必须能区分「完整文本」「尾部被切」（TruncatedSuffix）
// 「中间被切头尾保留」（本标记）。keptHead/keptTail 为两侧实际保留字节数，dropped 为省略字节数。
func TruncateHeadTail(keptHead, keptTail, dropped int) string {
	return fmt.Sprintf("……[内容过长已截断：显示开头 %d 字节、结尾 %d 字节，中间省略 %d 字节]……",
		keptHead, keptTail, dropped)
}

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

// TruncateHeadTailUTF8 头尾保留式截断：超限文本保留头部 headBytes 与尾部
// tailBytes（各自回退 rune 边界），中间替换为 TruncateHeadTail 标记。
// 返回值恒 ≤ headBytes+tailBytes+标记长度：标记字节计入两侧预算之外（调用方
// 需要严格 ≤ 上限时，把标记长度从预算里扣掉，见 truncateResponseBody）。
// 未超限原样返回。headBytes/tailBytes ≤ 0 时只保留对侧。
func TruncateHeadTailUTF8(s string, headBytes, tailBytes int) string {
	if len(s) <= headBytes+tailBytes {
		return s
	}
	head := cutAtRuneBoundary(s, headBytes)
	// 尾段从末尾往前找到完整 rune 起点
	tailStart := len(s) - tailBytes
	if tailStart < 0 {
		tailStart = 0
	}
	for tailStart < len(s) && !utf8.RuneStart(s[tailStart]) {
		tailStart++
	}
	dropped := len(s) - len(head) - (len(s) - tailStart)
	return head + TruncateHeadTail(len(head), len(s)-tailStart, dropped) + s[tailStart:]
}

// cutAtRuneBoundary 取 s 前 n 字节，回退到完整 rune 边界；n ≤ 0 或超长按防御处理。
func cutAtRuneBoundary(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if n >= len(s) {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
