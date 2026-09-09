package testapi

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qkf688/llmux/common"
)

func parseProviderErrorDetail(bodyBytes []byte) string {
	bodyStr := string(bodyBytes)
	if len(bodyBytes) == 0 {
		return bodyStr
	}

	var errorDetail string
	var jsonErr map[string]interface{}
	if json.Unmarshal(bodyBytes, &jsonErr) == nil {
		if errMsg, ok := jsonErr["error"].(map[string]interface{}); ok {
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

	return errorDetail
}

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

// 状态码 → 错误类型映射委托给 common.ErrorTypeFromStatus（诊断测试 error_type 的单一
// 来源，前端 webui/src/lib/errors.ts 镜像该值域）。network 由调用点硬编码、timeout 为
// 预留值——当前 60s 客户端超时仍归类 network，若需细分在 client.Do 错误分支用 errors.Is
// 区分，届时值域与 common 常量、getErrorTypeLabel 同步补充。
func getErrorTypeFromStatus(statusCode int) string {
	return common.ErrorTypeFromStatus(statusCode)
}

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
