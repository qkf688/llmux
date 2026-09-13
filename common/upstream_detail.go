package common

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// upstreamDetailMaxBytes 诊断细节回显的截断上限：上游错误响应偶有整页 HTML/超长回显，
// 不截断会灌进管理端 JSON（testapi 旧实现无上限，pools 旧实现有——收敛取保守口径）。
const upstreamDetailMaxBytes = 512

// UpstreamDiagnosticReadLimitBytes 探针 / 健康检查 / 连通测试端点读取上游响应体的字节上限（1 MiB）。
// 这些路径读响应体只用于：非 2xx 时提取 ≤512B 的人读错误细节，或解析极小的 completion 探测/
// 测试结果（请求侧 max_tokens 通常 1024 封顶，合法响应远小于此值）。上限只挡上游异常/被篡改时
// 返回的超大响应整块进内存，不影响任何正常结论——避免「先无界全量读、再截到 512B」的纯浪费与大响应内存风险。
const UpstreamDiagnosticReadLimitBytes = 1 << 20

// ReadCappedBody 按字节上限读取响应体，是所有诊断/探针/测试路径读取上游 Body 的单一入口。
// 超过上限的部分被丢弃（LimitReader 语义），读取超时/网络错误仍原样向上返回，与调用方的
// ErrorTypeTimeout 归类逻辑兼容。
func ReadCappedBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, UpstreamDiagnosticReadLimitBytes))
}

// upstreamUnwrapMaxDepth 错误信封递归剥套的层数上限：网关会把上游 provider 的原始错误体
// JSON 编码成字符串再嵌进自己的 message（opencode go 即此形状），实际观测 2~3 层封顶，
// 该上限只挡病态自嵌套，不构成对真实层数的约束。
const upstreamUnwrapMaxDepth = 5

// ExtractUpstreamErrorDetail 上游错误体的细节提取单一来源：沿 JSON 信封的
// error / message 字段递归追到最内层人读文本（{"error":{"message":…}}、
// {"error":"<内嵌JSON字符串>"}、{"type":…,"message":…} 等形状及其任意嵌套），
// 解析不出则回退原文，最终统一截断。completion 探针（testapi / #24-B）与
// react/structured-output 测试端点共用，消灭「JSON 解析」与「原文截断」两套实现各说各话的漂移。
func ExtractUpstreamErrorDetail(bodyBytes []byte) string {
	if len(bodyBytes) == 0 {
		return ""
	}

	detail := ""
	var envelope map[string]any
	if json.Unmarshal(bodyBytes, &envelope) == nil {
		detail = upstreamMessageFromObject(envelope, upstreamUnwrapMaxDepth)
	}
	if detail == "" {
		detail = string(bodyBytes)
	}

	return truncateUpstreamDetail(detail)
}

// UpstreamErrorDetail 非 2xx 结论的完整细节：提取内容 + 状态码行。状态码必须出现在回显
// 文本里（改造前它由 ErrorWithHttpStatus 的 code 承载，收敛进 data.error 后不能丢这个事实）。
func UpstreamErrorDetail(statusCode int, bodyBytes []byte) string {
	return ExtractUpstreamErrorDetail(bodyBytes) + "\nHTTP 状态码: " + strconv.Itoa(statusCode)
}

// upstreamMessageFromObject 从一个 JSON 错误对象追人读文本：
// error 字段是对象→继续向内剥；是字符串→按文本解套；都没有再看本层 message。
func upstreamMessageFromObject(envelope map[string]any, depthBudget int) string {
	if depthBudget <= 0 {
		return ""
	}
	if errField, ok := envelope["error"]; ok {
		switch node := errField.(type) {
		case map[string]any:
			if message := upstreamMessageFromObject(node, depthBudget-1); message != "" {
				return message
			}
		case string:
			if message := upstreamMessageFromText(node, depthBudget-1); message != "" {
				return message
			}
		}
	}
	if message, ok := envelope["message"].(string); ok {
		return upstreamMessageFromText(message, depthBudget-1)
	}
	return ""
}

// upstreamMessageFromText 处理字符串形态的错误载荷：本身是 JSON 对象则递归解套
// （网关把 provider 错误原文塞进 message 的场景），否则它就是人读文本。
func upstreamMessageFromText(text string, depthBudget int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{") {
		var nested map[string]any
		if json.Unmarshal([]byte(trimmed), &nested) == nil {
			if message := upstreamMessageFromObject(nested, depthBudget); message != "" {
				return message
			}
		}
	}
	return trimmed
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
