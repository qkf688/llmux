package healthcheck

import (
	"context"
	"strconv"
	"time"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func (h *HealthChecker) isEnabled(ctx context.Context) bool {
	return getSettingBool(ctx, models.SettingKeyHealthCheckEnabled, false)
}

func (h *HealthChecker) getInterval(ctx context.Context) time.Duration {
	minutes := getSettingInt(ctx, models.SettingKeyHealthCheckInterval, 60, 1)
	return time.Duration(minutes) * time.Minute
}

func (h *HealthChecker) getFailureThreshold(ctx context.Context) int {
	return getSettingInt(ctx, models.SettingKeyHealthCheckFailureThreshold, 3, 1)
}

func (h *HealthChecker) getAutoEnable(ctx context.Context) bool {
	return getSettingBool(ctx, models.SettingKeyHealthCheckAutoEnable, false)
}

func (h *HealthChecker) getFailureDisableEnabled(ctx context.Context) bool {
	return getSettingBool(ctx, models.SettingKeyHealthCheckFailureDisableEnabled, true)
}

func (h *HealthChecker) getCheckDisabledOnly(ctx context.Context) bool {
	return getSettingBool(ctx, models.SettingKeyHealthCheckCheckDisabledOnly, false)
}

func (h *HealthChecker) getLogRetentionCount(ctx context.Context) int {
	return getSettingInt(ctx, models.SettingKeyHealthCheckLogRetentionCount, 0, 0)
}

// GetHealthCheckSettings 获取健康检测设置。
func GetHealthCheckSettings(ctx context.Context) (enabled bool, interval int, failureThreshold int, failureDisableEnabled bool, autoEnable bool, logRetentionCount int, countAsSuccess bool, countAsFailure bool, checkDisabledOnly bool) {
	checker := GetHealthChecker()

	enabled = checker.isEnabled(ctx)
	interval = int(checker.getInterval(ctx) / time.Minute)
	failureThreshold = checker.getFailureThreshold(ctx)
	failureDisableEnabled = checker.getFailureDisableEnabled(ctx)
	autoEnable = checker.getAutoEnable(ctx)
	logRetentionCount = checker.getLogRetentionCount(ctx)
	countAsSuccess = shouldCountHealthCheckSuccess(ctx)
	countAsFailure = shouldCountHealthCheckFailure(ctx)
	checkDisabledOnly = checker.getCheckDisabledOnly(ctx)

	return
}

func getSettingBool(ctx context.Context, key string, defaultValue bool) bool {
	setting, err := gorm.G[models.Setting](models.DB).Where("key = ?", key).First(ctx)
	if err != nil {
		return defaultValue
	}
	return setting.Value == "true"
}

func getSettingInt(ctx context.Context, key string, defaultValue, minValue int) int {
	setting, err := gorm.G[models.Setting](models.DB).Where("key = ?", key).First(ctx)
	if err != nil {
		return defaultValue
	}

	value, err := strconv.Atoi(setting.Value)
	if err != nil || value < minValue {
		return defaultValue
	}
	return value
}
