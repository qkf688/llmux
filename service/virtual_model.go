package service

import (
	"github.com/qkf688/llmux/service/virtualmodel"
	"gorm.io/gorm"
)

// VirtualModelService 虚拟模型服务（兼容层）。
type VirtualModelService = virtualmodel.Service

// OrderedRealModel 有序的真实模型（兼容层）。
type OrderedRealModel = virtualmodel.OrderedRealModel

// NewVirtualModelService 创建或获取虚拟模型服务单例。
func NewVirtualModelService(db *gorm.DB) *VirtualModelService {
	return virtualmodel.NewService(db)
}
