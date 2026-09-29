package healthcheck

import (
	"context"
	"time"

	"github.com/qkf688/llmux/models"
)

func (h *HealthChecker) isEnabled(ctx context.Context) bool {
	return getSettingBool(ctx, models.SettingKeyHealthCheckEnabled, false)
}

func (h *HealthChecker) getInterval(ctx context.Context) time.Duration {
	minutes := getSettingInt(ctx, models.SettingKeyHealthCheckInterval, 60, 1)
	return time.Duration(minutes) * time.Minute //gate:diag-timeout-allow settings 分钟值换算，非诊断超时参数
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
	interval = int(checker.getInterval(ctx) / time.Minute) //gate:diag-timeout-allow Duration→分钟单位换算，非诊断超时参数
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
	return models.GetSettingBool(ctx, key, defaultValue)
}

func getSettingInt(ctx context.Context, key string, defaultValue, minValue int) int {
	return models.GetSettingInt(ctx, key, defaultValue, minValue)
}

// getSettingBoolE / getSettingIntE 是「读失败可辨」变体（Stage 36，与 Stage 33 结算侧同口径）：
// key 不存在 → (default, nil)（未设置是有效状态）；查询失败 / 存值非法 → (default, err)。
// **「读失败会引发写动作或改变巡检范围」的决策点必须用这一对**并按 err 显式降级。
func getSettingBoolE(ctx context.Context, key string, defaultValue bool) (bool, error) {
	return models.GetSettingBoolE(ctx, key, defaultValue)
}

func getSettingIntE(ctx context.Context, key string, defaultValue, minValue int) (int, error) {
	return models.GetSettingIntE(ctx, key, defaultValue, minValue)
}

// isEnabledE 与 isEnabled 同取值语义，区分读失败（Stage 36 Ruling 4）：
// 读失败退默认 false 会让巡检循环把自己**永久**停掉，而没有任何机制把它拉回来。
func (h *HealthChecker) isEnabledE(ctx context.Context) (bool, error) {
	return getSettingBoolE(ctx, models.SettingKeyHealthCheckEnabled, false)
}

// getCheckDisabledOnlyE 与 getCheckDisabledOnly 同取值语义，区分读失败（Stage 36）。
// 读失败退默认 false 会把「只测已停用的」翻成「全量测」——探测是真实计费请求，
// 读不出来时不得扩大开销。
func (h *HealthChecker) getCheckDisabledOnlyE(ctx context.Context) (bool, error) {
	return getSettingBoolE(ctx, models.SettingKeyHealthCheckCheckDisabledOnly, false)
}
