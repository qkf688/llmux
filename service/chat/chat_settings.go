package chat

import (
	"context"
	"encoding/json"

	"github.com/atopos31/llmio/models"
)

func getStrictCapabilityMatch(ctx context.Context) bool {
	return models.GetSettingBool(ctx, models.SettingKeyStrictCapabilityMatch, false)
}

// getLogRawRequestResponse 获取原始请求响应记录选项
func getLogRawRequestResponse(ctx context.Context) models.RawLogOptions {
	value := models.GetSettingString(ctx, models.SettingKeyLogRawRequestResponse, "")
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
	return models.GetSettingBool(ctx, models.SettingKeyLogRawRequestResponseErrorsOnly, false)
}

// getDisableAllLogs 获取是否完全关闭日志记录
func getDisableAllLogs(ctx context.Context) bool {
	return models.GetSettingBool(ctx, models.SettingKeyDisableAllLogs, false)
}

// getDisablePerformanceTracking 获取是否关闭性能追踪
func getDisablePerformanceTracking(ctx context.Context) bool {
	return models.GetSettingBool(ctx, models.SettingKeyDisablePerformanceTracking, false)
}

// getDisableTokenCounting 获取是否关闭 token 统计
func getDisableTokenCounting(ctx context.Context) bool {
	return models.GetSettingBool(ctx, models.SettingKeyDisableTokenCounting, false)
}

// getEnableRequestTrace 获取是否启用请求追踪
func getEnableRequestTrace(ctx context.Context) bool {
	return models.GetSettingBool(ctx, models.SettingKeyEnableRequestTrace, true)
}

// GetStripResponseHeaders 获取是否移除不必要的响应头（导出供 handler 使用）
func GetStripResponseHeaders(ctx context.Context) bool {
	return models.GetSettingBool(ctx, models.SettingKeyStripResponseHeaders, false)
}

// getEnableFormatConversion 获取是否启用格式转换
func getEnableFormatConversion(ctx context.Context) bool {
	return models.GetSettingBool(ctx, models.SettingKeyEnableFormatConversion, true)
}
