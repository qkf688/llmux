package handler

import (
	"github.com/atopos31/llmio/handler/settings"
)

// 重新导出所有设置相关的函数，保持向后兼容性

// SystemConfigRequest represents the request body for updating system configuration
type SystemConfigRequest = settings.SystemConfigRequest

// SettingsResponse 设置响应结构
type SettingsResponse = settings.SettingsResponse

// UpdateSettingsRequest 更新设置请求结构
type UpdateSettingsRequest = settings.UpdateSettingsRequest

// HealthCheckSettingsResponse 健康检测设置响应结构
type HealthCheckSettingsResponse = settings.HealthCheckSettingsResponse

// UpdateHealthCheckSettingsRequest 更新健康检测设置请求结构
type UpdateHealthCheckSettingsRequest = settings.UpdateHealthCheckSettingsRequest

// GetSystemConfig 获取系统配置
var GetSystemConfig = settings.GetSystemConfig

// UpdateSystemConfig 更新系统配置
var UpdateSystemConfig = settings.UpdateSystemConfig

// GetSettings 获取所有设置
var GetSettings = settings.GetSettings

// UpdateSettings 更新设置
var UpdateSettings = settings.UpdateSettings

// GetHealthCheckSettings 获取健康检测设置
var GetHealthCheckSettings = settings.GetHealthCheckSettings

// UpdateHealthCheckSettings 更新健康检测设置
var UpdateHealthCheckSettings = settings.UpdateHealthCheckSettings

// GetStrictCapabilityMatch 获取严格能力匹配设置
var GetStrictCapabilityMatch = settings.GetStrictCapabilityMatch

// GetAutoPriorityDecayDefault 获取自动优先级衰减默认值
var GetAutoPriorityDecayDefault = settings.GetAutoPriorityDecayDefault

// GetSettingBool 获取设置的布尔值
var GetSettingBool = settings.GetSettingBool

// ResetModelWeights 重置所有模型权重
var ResetModelWeights = settings.ResetModelWeights

// ResetModelPriorities 重置所有模型优先级
var ResetModelPriorities = settings.ResetModelPriorities

// EnableAllAssociations 启用所有模型关联
var EnableAllAssociations = settings.EnableAllAssociations
