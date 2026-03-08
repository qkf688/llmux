package chat

import (
	"context"
	"encoding/json"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func getStrictCapabilityMatch(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyStrictCapabilityMatch).
		First(ctx)
	if err != nil {
		return false // 默认关闭
	}
	return setting.Value == "true"
}

// getLogRawRequestResponse 获取原始请求响应记录选项
func getLogRawRequestResponse(ctx context.Context) models.RawLogOptions {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyLogRawRequestResponse).
		First(ctx)
	if err != nil {
		return models.RawLogOptions{} // 默认全部关闭
	}

	var options models.RawLogOptions
	if err := json.Unmarshal([]byte(setting.Value), &options); err != nil {
		return models.RawLogOptions{} // 解析失败，默认全部关闭
	}
	return options
}

func getLogRawRequestResponseErrorsOnly(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyLogRawRequestResponseErrorsOnly).
		First(ctx)
	if err != nil {
		return false // 默认关闭
	}
	return setting.Value == "true"
}

// getDisableAllLogs 获取是否完全关闭日志记录
func getDisableAllLogs(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyDisableAllLogs).
		First(ctx)
	if err != nil {
		return false // 默认不关闭（即记录日志）
	}
	return setting.Value == "true"
}

// getDisablePerformanceTracking 获取是否关闭性能追踪
func getDisablePerformanceTracking(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyDisablePerformanceTracking).
		First(ctx)
	if err != nil {
		return false // 默认不关闭
	}
	return setting.Value == "true"
}

// getDisableTokenCounting 获取是否关闭 token 统计
func getDisableTokenCounting(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyDisableTokenCounting).
		First(ctx)
	if err != nil {
		return false // 默认不关闭
	}
	return setting.Value == "true"
}

// getEnableRequestTrace 获取是否启用请求追踪
func getEnableRequestTrace(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyEnableRequestTrace).
		First(ctx)
	if err != nil {
		return true // 默认启用
	}
	return setting.Value == "true"
}

// GetStripResponseHeaders 获取是否移除不必要的响应头（导出供 handler 使用）
func GetStripResponseHeaders(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyStripResponseHeaders).
		First(ctx)
	if err != nil {
		return false // 默认不移除
	}
	return setting.Value == "true"
}

// getEnableFormatConversion 获取是否启用格式转换
func getEnableFormatConversion(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyEnableFormatConversion).
		First(ctx)
	if err != nil {
		return true // 默认启用
	}
	return setting.Value == "true"
}
