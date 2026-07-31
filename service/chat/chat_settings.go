package chat

import (
	"context"
	"encoding/json"

	"github.com/qkf688/llmux/models"
)

// getStrictCapabilityMatch 读取严格能力匹配设置。
// 默认值与 setting schema 一致（true），避免与 handler/settings.GetStrictCapabilityMatch 反向默认导致行为分叉。
func getStrictCapabilityMatch(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyStrictCapabilityMatch, true)
}

// getLogRawRequestResponse 获取原始请求响应记录选项
func getLogRawRequestResponse(ctx context.Context) models.RawLogOptions {
	value := settingsReader.String(ctx, models.SettingKeyLogRawRequestResponse, "")
	if value == "" {
		return models.RawLogOptions{} // 默认全部关闭
	}

	var options models.RawLogOptions
	if err := json.Unmarshal([]byte(value), &options); err != nil {
		return models.RawLogOptions{} // 解析失败，默认全部关闭
	}
	return options
}

func getLogRawRequestResponseErrorsOnly(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyLogRawRequestResponseErrorsOnly, false)
}

// getDisableAllLogs 获取是否完全关闭日志记录
func getDisableAllLogs(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyDisableAllLogs, false)
}

// getDisablePerformanceTracking 获取是否关闭性能追踪
func getDisablePerformanceTracking(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyDisablePerformanceTracking, false)
}

// getDisableTokenCounting 获取是否关闭 token 统计
func getDisableTokenCounting(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyDisableTokenCounting, false)
}

// getEnableRequestTrace 获取是否启用请求追踪
func getEnableRequestTrace(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyEnableRequestTrace, true)
}

// GetStripResponseHeaders 获取是否移除不必要的响应头（导出供 handler 使用）
func GetStripResponseHeaders(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyStripResponseHeaders, false)
}

// getEnableFormatConversion 获取是否启用格式转换
func getEnableFormatConversion(ctx context.Context) bool {
	return settingsReader.Bool(ctx, models.SettingKeyEnableFormatConversion, true)
}
