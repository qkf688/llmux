package settings

import (
	"context"
	"strconv"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetHealthCheckSettings 获取健康检测设置
func GetHealthCheckSettings(c *gin.Context) {
	ctx := c.Request.Context()
	enabled, interval, failureThreshold, failureDisableEnabled, autoEnable, logRetentionCount, countAsSuccess, countAsFailure, checkDisabledOnly := service.GetHealthCheckSettings(ctx)

	response := HealthCheckSettingsResponse{
		Enabled:                 enabled,
		Interval:                interval,
		FailureThreshold:        failureThreshold,
		FailureDisableEnabled:   failureDisableEnabled,
		AutoEnable:              autoEnable,
		LogRetentionCount:       logRetentionCount,
		CountHealthCheckSuccess: countAsSuccess,
		CountHealthCheckFailure: countAsFailure,
		CheckDisabledOnly:       checkDisabledOnly,
	}

	httpresp.Success(c, response)
}

// UpdateHealthCheckSettings 更新健康检测设置
func UpdateHealthCheckSettings(c *gin.Context) {
	var req UpdateHealthCheckSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// 更新启用状态
	enabledValue := "false"
	if req.Enabled {
		enabledValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckEnabled).
		Update(ctx, "value", enabledValue); err != nil {
		httpresp.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新检测间隔
	if req.Interval < 1 {
		req.Interval = 60
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckInterval).
		Update(ctx, "value", strconv.Itoa(req.Interval)); err != nil {
		httpresp.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新失败次数阈值
	if req.FailureThreshold < 1 {
		req.FailureThreshold = 3
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckFailureThreshold).
		Update(ctx, "value", strconv.Itoa(req.FailureThreshold)); err != nil {
		httpresp.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新失败自动禁用开关
	failureDisableEnabledValue := "false"
	if req.FailureDisableEnabled {
		failureDisableEnabledValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckFailureDisableEnabled).
		Update(ctx, "value", failureDisableEnabledValue); err != nil {
		httpresp.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动启用
	autoEnableValue := "false"
	if req.AutoEnable {
		autoEnableValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckAutoEnable).
		Update(ctx, "value", autoEnableValue); err != nil {
		httpresp.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新健康检测日志保留条数
	if req.LogRetentionCount < 0 {
		req.LogRetentionCount = 0
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckLogRetentionCount).
		Update(ctx, "value", strconv.Itoa(req.LogRetentionCount)); err != nil {
		httpresp.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	countHealthCheckSuccess := "false"
	if req.CountHealthCheckSuccess {
		countHealthCheckSuccess = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsSuccess).
		Update(ctx, "value", countHealthCheckSuccess); err != nil {
		httpresp.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	countHealthCheckFailure := "false"
	if req.CountHealthCheckFailure {
		countHealthCheckFailure = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsFailure).
		Update(ctx, "value", countHealthCheckFailure); err != nil {
		httpresp.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新只检测停用的模型设置
	checkDisabledOnly := "false"
	if req.CheckDisabledOnly {
		checkDisabledOnly = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCheckDisabledOnly).
		Update(ctx, "value", checkDisabledOnly); err != nil {
		httpresp.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 重启健康检测服务
	go service.GetHealthChecker().Restart(context.Background())

	// 执行日志清理以满足新的保留策略
	go service.EnforceHealthCheckLogRetention(context.Background())

	// 返回更新后的设置
	GetHealthCheckSettings(c)
}
