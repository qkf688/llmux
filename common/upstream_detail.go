package common

import (
	"encoding/json"
	"strconv"
	"unicode/utf8"
)

// upstreamDetailMaxBytes 诊断细节回显的截断上限：上游错误响应偶有整页 HTML/超长回显，
// 不截断会灌进管理端 JSON（testapi 旧实现无上限，pools 旧实现有——收敛取保守口径）。
const upstreamDetailMaxBytes = 512

// ExtractUpstreamErrorDetail 上游错误体的细节提取单一来源：优先取常见错误信封的
// message（{"error":{"message":…}} 或 {"error":{"error":…}} 两种形状），解析不出则回退原文，
// 最终统一截断。completion 探针（testapi / #24-B）与 react/structured-output 测试端点共用，
// 消灭「JSON 解析」与「原文截断」两套实现各说各话的漂移。
func ExtractUpstreamErrorDetail(bodyBytes []byte) string {
	bodyStr := string(bodyBytes)
	if len(bodyBytes) == 0 {
		return bodyStr
	}

	errorDetail := ""
	var jsonErr map[string]any
	if json.Unmarshal(bodyBytes, &jsonErr) == nil {
		if errMsg, ok := jsonErr["error"].(map[string]any); ok {
			if message, ok := errMsg["message"].(string); ok {
				errorDetail = message
			} else if msg, ok := errMsg["error"].(string); ok {
				errorDetail = msg
			}
		}
	}
	if errorDetail == "" {
		errorDetail = bodyStr
	}

	return truncateUpstreamDetail(errorDetail)
}

// UpstreamErrorDetail 非 2xx 结论的完整细节：提取内容 + 状态码行。状态码必须出现在回显
// 文本里（改造前它由 ErrorWithHttpStatus 的 code 承载，收敛进 data.error 后不能丢这个事实）。
func UpstreamErrorDetail(statusCode int, bodyBytes []byte) string {
	return ExtractUpstreamErrorDetail(bodyBytes) + "\nHTTP 状态码: " + strconv.Itoa(statusCode)
}

func truncateUpstreamDetail(detail string) string {
	if len(detail) <= upstreamDetailMaxBytes {
		return detail
	}
	// 回退到 rune 边界：按字节切会把多字节字符劈半，json.Marshal 将残字节替换成
	// U+FFFD——上游中文错误文案在 512B 处稳定出乱码尾（pools 旧实现的存量毛病，
	// 收敛为单一来源时一并修）。
	cut := upstreamDetailMaxBytes
	for cut > 0 && !utf8.RuneStart(detail[cut]) {
		cut--
	}
	return detail[:cut] + "…(truncated)"
}
