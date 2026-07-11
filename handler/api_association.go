package handler

import "github.com/atopos31/llmio/handler/associations"

// ModelWithProviderRequest represents the request body for creating/updating a model-provider association（兼容层）。
type ModelWithProviderRequest = associations.ModelWithProviderRequest

// ModelProviderStatusRequest represents the request body for updating provider status（兼容层）。
type ModelProviderStatusRequest = associations.ModelProviderStatusRequest

// BatchDeleteModelProvidersRequest represents the request body for batch deleting（兼容层）。
type BatchDeleteModelProvidersRequest = associations.BatchDeleteModelProvidersRequest

// BatchUpdateModelProvidersStatusRequest represents the request body for batch status update（兼容层）。
type BatchUpdateModelProvidersStatusRequest = associations.BatchUpdateModelProvidersStatusRequest

// BatchUpdateModelProvidersCapabilitiesRequest represents the request body for batch capabilities（兼容层）。
type BatchUpdateModelProvidersCapabilitiesRequest = associations.BatchUpdateModelProvidersCapabilitiesRequest

// GetModelProviders 获取模型的提供商关联列表（兼容层）。
var GetModelProviders = associations.GetModelProviders

// GetModelProviderStatus 获取提供商状态信息（兼容层）。
var GetModelProviderStatus = associations.GetModelProviderStatus

// GetModelProviderHealthStatus 获取健康检测结果（兼容层）。
var GetModelProviderHealthStatus = associations.GetModelProviderHealthStatus

// CreateModelProvider 创建关联（兼容层）。
var CreateModelProvider = associations.CreateModelProvider

// UpdateModelProvider 更新关联（兼容层）。
var UpdateModelProvider = associations.UpdateModelProvider

// UpdateModelProviderStatus 更新关联状态（兼容层）。
var UpdateModelProviderStatus = associations.UpdateModelProviderStatus

// DeleteModelProvider 删除关联（兼容层）。
var DeleteModelProvider = associations.DeleteModelProvider

// BatchDeleteModelProviders 批量删除（兼容层）。
var BatchDeleteModelProviders = associations.BatchDeleteModelProviders

// BatchUpdateModelProvidersStatus 批量更新状态（兼容层）。
var BatchUpdateModelProvidersStatus = associations.BatchUpdateModelProvidersStatus

// BatchUpdateModelProvidersCapabilities 批量更新能力（兼容层）。
var BatchUpdateModelProvidersCapabilities = associations.BatchUpdateModelProvidersCapabilities