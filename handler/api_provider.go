package handler

import "github.com/atopos31/llmio/handler/providerapi"

// ProviderTemplate 供应商配置模板（兼容层）。
type ProviderTemplate = providerapi.ProviderTemplate

// ProviderRequest represents the request body for creating/updating a provider（兼容层）。
type ProviderRequest = providerapi.ProviderRequest

// GetProviders 获取所有提供商列表（兼容层）。
var GetProviders = providerapi.GetProviders

// GetProviderModels 拉取供应商上游模型列表（兼容层）。
var GetProviderModels = providerapi.GetProviderModels

// CreateProvider 创建提供商（兼容层）。
var CreateProvider = providerapi.CreateProvider

// UpdateProvider 更新提供商（兼容层）。
var UpdateProvider = providerapi.UpdateProvider

// DeleteProvider 删除提供商（兼容层）。
var DeleteProvider = providerapi.DeleteProvider

// GetProviderTemplates 获取提供商模板（兼容层）。
var GetProviderTemplates = providerapi.GetProviderTemplates

// GetProviderBlacklist 获取拉黑的供应商 ID 列表（兼容层）。
var GetProviderBlacklist = providerapi.GetProviderBlacklist

// UpdateProviderBlacklist 更新供应商黑名单（兼容层）。
var UpdateProviderBlacklist = providerapi.UpdateProviderBlacklist

// ClearProviderAssociations 清除指定提供商的所有模型关联（兼容层）。
var ClearProviderAssociations = providerapi.ClearProviderAssociations