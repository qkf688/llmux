package testapi

import (
	"fmt"
	"strings"
)

func BuildDetailedError(errorType string, summary string, detail string, context map[string]string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s\n\n", getErrorTypeLabel(errorType), summary))
	sb.WriteString(fmt.Sprintf("详细信息: %s\n", detail))

	if len(context) > 0 {
		sb.WriteString("\n上下文信息:\n")
		for key, value := range context {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", key, value))
		}
	}

	return sb.String()
}

// 状态码 → 错误类型映射见 common.ErrorTypeFromStatus（诊断测试 error_type 的单一来源，
// 前端 webui/src/lib/errors.ts 镜像该值域）；network/timeout 由 credprobe.RunCompletionProbe
// 在 Do 错误分支用 errors.Is 区分后产出。

func getErrorTypeLabel(errorType string) string {
	labels := map[string]string{
		"network":    "网络错误",
		"auth":       "认证错误",
		"provider":   "提供商错误",
		"timeout":    "超时错误",
		"validation": "验证错误",
		"unknown":    "未知错误",
	}
	if label, ok := labels[errorType]; ok {
		return label
	}
	return "未知错误"
}
