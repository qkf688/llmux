package handler

import "github.com/atopos31/llmio/handler/virtualmodels"

// VirtualModelRequest 虚拟模型请求结构（兼容层）。
type VirtualModelRequest = virtualmodels.VirtualModelRequest

// VirtualModelMappingRequest 虚拟模型映射请求结构（兼容层）。
type VirtualModelMappingRequest = virtualmodels.VirtualModelMappingRequest

// BatchVirtualModelMappingRequest 批量创建虚拟模型映射请求结构（兼容层）。
type BatchVirtualModelMappingRequest = virtualmodels.BatchVirtualModelMappingRequest

// BatchDeleteVirtualModelMappingRequest 批量删除虚拟模型映射请求结构（兼容层）。
type BatchDeleteVirtualModelMappingRequest = virtualmodels.BatchDeleteVirtualModelMappingRequest

// BatchFailedItem 批量创建失败项（兼容层）。
type BatchFailedItem = virtualmodels.BatchFailedItem

// BatchCreateResult 批量创建结果（兼容层）。
type BatchCreateResult = virtualmodels.BatchCreateResult

// GetVirtualModels 获取虚拟模型列表（兼容层）。
var GetVirtualModels = virtualmodels.GetVirtualModels

// CreateVirtualModel 创建虚拟模型（兼容层）。
var CreateVirtualModel = virtualmodels.CreateVirtualModel

// UpdateVirtualModel 更新虚拟模型（兼容层）。
var UpdateVirtualModel = virtualmodels.UpdateVirtualModel

// DeleteVirtualModel 删除虚拟模型（兼容层）。
var DeleteVirtualModel = virtualmodels.DeleteVirtualModel

// GetVirtualModelMappings 获取虚拟模型映射（兼容层）。
var GetVirtualModelMappings = virtualmodels.GetVirtualModelMappings

// CreateVirtualModelMapping 创建虚拟模型映射（兼容层）。
var CreateVirtualModelMapping = virtualmodels.CreateVirtualModelMapping

// UpdateVirtualModelMapping 更新虚拟模型映射（兼容层）。
var UpdateVirtualModelMapping = virtualmodels.UpdateVirtualModelMapping

// DeleteVirtualModelMapping 删除虚拟模型映射（兼容层）。
var DeleteVirtualModelMapping = virtualmodels.DeleteVirtualModelMapping

// BatchDeleteVirtualModelMapping 批量删除虚拟模型映射（兼容层）。
var BatchDeleteVirtualModelMapping = virtualmodels.BatchDeleteVirtualModelMapping

// BatchCreateVirtualModelMapping 批量创建虚拟模型映射（兼容层）。
var BatchCreateVirtualModelMapping = virtualmodels.BatchCreateVirtualModelMapping

// GetVirtualModelStats 获取虚拟模型统计（兼容层）。
var GetVirtualModelStats = virtualmodels.GetVirtualModelStats