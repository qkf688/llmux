package service

import (
	"context"

	"github.com/qkf688/llmux/service/healthcheck"
)

// HealthChecker 健康检测服务（兼容层）。
type HealthChecker = healthcheck.HealthChecker

// HealthCheckError 健康检测错误（兼容层）。
type HealthCheckError = healthcheck.HealthCheckError

// HealthCheckSettingsJSON 健康检测设置 JSON 结构（兼容层）。
type HealthCheckSettingsJSON = healthcheck.HealthCheckSettingsJSON

// GetHealthChecker 获取健康检测单例。
func GetHealthChecker() *HealthChecker {
	return healthcheck.GetHealthChecker()
}

// GetHealthCheckSettings 获取健康检测设置。
func GetHealthCheckSettings(ctx context.Context) (enabled bool, interval int, failureThreshold int, failureDisableEnabled bool, autoEnable bool, logRetentionCount int, countAsSuccess bool, countAsFailure bool, checkDisabledOnly bool) {
	return healthcheck.GetHealthCheckSettings(ctx)
}

// EnforceHealthCheckLogRetention 清理超出保留条数的健康检测日志。
func EnforceHealthCheckLogRetention(ctx context.Context) {
	healthcheck.EnforceHealthCheckLogRetention(ctx)
}
