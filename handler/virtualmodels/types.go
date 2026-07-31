package virtualmodels

import "github.com/qkf688/llmux/models"

// VirtualModelRequest 虚拟模型请求结构。
type VirtualModelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Strategy    string `json:"strategy"`
	MaxRetry    int    `json:"max_retry"`
	TimeOut     int    `json:"time_out"`
	IOLog       bool   `json:"io_log"`
	Enabled     bool   `json:"enabled"`
}

// VirtualModelMappingRequest 虚拟模型映射请求结构。
type VirtualModelMappingRequest struct {
	RealModelID uint `json:"real_model_id"`
	Priority    int  `json:"priority"`
	Weight      int  `json:"weight"`
	Enabled     bool `json:"enabled"`
}

// BatchVirtualModelMappingRequest 批量创建虚拟模型映射请求结构。
type BatchVirtualModelMappingRequest struct {
	Mappings []VirtualModelMappingRequest `json:"mappings"`
}

// BatchDeleteVirtualModelMappingRequest 批量删除虚拟模型映射请求结构。
type BatchDeleteVirtualModelMappingRequest struct {
	IDs []uint `json:"ids"`
}

// BatchFailedItem 批量创建失败项。
type BatchFailedItem struct {
	RealModelID uint   `json:"real_model_id"`
	Reason      string `json:"reason"`
}

// BatchCreateResult 批量创建结果。
type BatchCreateResult struct {
	SuccessCount int                          `json:"success_count"`
	FailedCount  int                          `json:"failed_count"`
	SuccessItems []models.VirtualModelMapping `json:"success_items"`
	FailedItems  []BatchFailedItem            `json:"failed_items"`
}
