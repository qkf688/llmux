package handler

import (
	"strconv"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ==================== 虚拟模型相关 API ====================

// VirtualModelRequest 虚拟模型请求结构
type VirtualModelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Strategy    string `json:"strategy"`
	MaxRetry    int    `json:"max_retry"`
	TimeOut     int    `json:"time_out"`
	IOLog       bool   `json:"io_log"`
	Enabled     bool   `json:"enabled"`
}

// VirtualModelMappingRequest 虚拟模型映射请求结构
type VirtualModelMappingRequest struct {
	RealModelID uint `json:"real_model_id"`
	Priority    int  `json:"priority"`
	Weight      int  `json:"weight"`
	Enabled     bool `json:"enabled"`
}

// GetVirtualModels 获取虚拟模型列表
func GetVirtualModels(c *gin.Context) {
	virtualModels, err := gorm.G[models.VirtualModel](models.DB).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, err.Error())
		return
	}
	common.Success(c, virtualModels)
}

// CreateVirtualModel 创建虚拟模型
func CreateVirtualModel(c *gin.Context) {
	var req VirtualModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// 检查名称是否已存在
	count, err := gorm.G[models.VirtualModel](models.DB).Where("name = ?", req.Name).Count(c.Request.Context(), "id")
	if err != nil {
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if count > 0 {
		common.BadRequest(c, "Virtual model name already exists")
		return
	}

	// 检查是否与真实模型名称冲突
	realModelCount, err := gorm.G[models.Model](models.DB).Where("name = ?", req.Name).Count(c.Request.Context(), "id")
	if err != nil {
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if realModelCount > 0 {
		common.BadRequest(c, "Virtual model name conflicts with existing real model")
		return
	}

	enabled := req.Enabled
	ioLog := req.IOLog

	virtualModel := models.VirtualModel{
		Name:        req.Name,
		Description: req.Description,
		Strategy:    req.Strategy,
		MaxRetry:    req.MaxRetry,
		TimeOut:     req.TimeOut,
		IOLog:       &ioLog,
		Enabled:     &enabled,
	}

	if err := gorm.G[models.VirtualModel](models.DB).Create(c.Request.Context(), &virtualModel); err != nil {
		common.InternalServerError(c, "Failed to create virtual model: "+err.Error())
		return
	}

	common.Success(c, virtualModel)
}

// UpdateVirtualModel 更新虚拟模型
func UpdateVirtualModel(c *gin.Context) {
	id := c.Param("id")
	var req VirtualModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	virtualModel, err := gorm.G[models.VirtualModel](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Virtual model not found")
		return
	}

	// 如果修改了名称，检查新名称是否已存在
	if req.Name != virtualModel.Name {
		count, err := gorm.G[models.VirtualModel](models.DB).Where("name = ? AND id != ?", req.Name, id).Count(c.Request.Context(), "id")
		if err != nil {
			common.InternalServerError(c, "Database error: "+err.Error())
			return
		}
		if count > 0 {
			common.BadRequest(c, "Virtual model name already exists")
			return
		}

		// 检查是否与真实模型名称冲突
		realModelCount, err := gorm.G[models.Model](models.DB).Where("name = ?", req.Name).Count(c.Request.Context(), "id")
		if err != nil {
			common.InternalServerError(c, "Database error: "+err.Error())
			return
		}
		if realModelCount > 0 {
			common.BadRequest(c, "Virtual model name conflicts with existing real model")
			return
		}
	}

	enabled := req.Enabled
	ioLog := req.IOLog

	virtualModel.Name = req.Name
	virtualModel.Description = req.Description
	virtualModel.Strategy = req.Strategy
	virtualModel.MaxRetry = req.MaxRetry
	virtualModel.TimeOut = req.TimeOut
	virtualModel.IOLog = &ioLog
	virtualModel.Enabled = &enabled

	if _, err := gorm.G[models.VirtualModel](models.DB).Where("id = ?", id).Updates(c.Request.Context(), virtualModel); err != nil {
		common.InternalServerError(c, "Failed to update virtual model: "+err.Error())
		return
	}

	common.Success(c, virtualModel)
}

// DeleteVirtualModel 删除虚拟模型
func DeleteVirtualModel(c *gin.Context) {
	id := c.Param("id")

	// 检查虚拟模型是否存在
	_, err := gorm.G[models.VirtualModel](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Virtual model not found")
		return
	}

	// 删除所有关联的映射
	if _, err := gorm.G[models.VirtualModelMapping](models.DB).Where("virtual_model_id = ?", id).Delete(c.Request.Context()); err != nil {
		common.InternalServerError(c, "Failed to delete virtual model mappings: "+err.Error())
		return
	}

	// 删除虚拟模型
	if _, err := gorm.G[models.VirtualModel](models.DB).Where("id = ?", id).Delete(c.Request.Context()); err != nil {
		common.InternalServerError(c, "Failed to delete virtual model: "+err.Error())
		return
	}

	common.Success(c, gin.H{"message": "Virtual model deleted successfully"})
}

// GetVirtualModelMappings 获取虚拟模型的映射关系
func GetVirtualModelMappings(c *gin.Context) {
	id := c.Param("id")

	// 检查虚拟模型是否存在
	_, err := gorm.G[models.VirtualModel](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Virtual model not found")
		return
	}

	mappings, err := gorm.G[models.VirtualModelMapping](models.DB).Where("virtual_model_id = ?", id).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, err.Error())
		return
	}

	common.Success(c, mappings)
}

// CreateVirtualModelMapping 创建虚拟模型映射
func CreateVirtualModelMapping(c *gin.Context) {
	id := c.Param("id")
	var req VirtualModelMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// 检查虚拟模型是否存在
	virtualModel, err := gorm.G[models.VirtualModel](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Virtual model not found")
		return
	}

	// 检查真实模型是否存在
	_, err = gorm.G[models.Model](models.DB).Where("id = ?", req.RealModelID).First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Real model not found")
		return
	}

	// 检查是否已存在相同的映射
	count, err := gorm.G[models.VirtualModelMapping](models.DB).
		Where("virtual_model_id = ? AND real_model_id = ?", id, req.RealModelID).
		Count(c.Request.Context(), "id")
	if err != nil {
		common.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if count > 0 {
		common.BadRequest(c, "Mapping already exists")
		return
	}

	// 验证没有循环依赖
	virtualModelService := service.NewVirtualModelService(models.DB)
	if err := virtualModelService.ValidateNoCircularDependency(c.Request.Context(), virtualModel.ID, req.RealModelID); err != nil {
		common.BadRequest(c, err.Error())
		return
	}

	enabled := req.Enabled

	mapping := models.VirtualModelMapping{
		VirtualModelID: virtualModel.ID,
		RealModelID:    req.RealModelID,
		Priority:       req.Priority,
		Weight:         req.Weight,
		Enabled:        &enabled,
	}

	if err := gorm.G[models.VirtualModelMapping](models.DB).Create(c.Request.Context(), &mapping); err != nil {
		common.InternalServerError(c, "Failed to create mapping: "+err.Error())
		return
	}

	common.Success(c, mapping)
}

// UpdateVirtualModelMapping 更新虚拟模型映射
func UpdateVirtualModelMapping(c *gin.Context) {
	id := c.Param("id")
	mappingID := c.Param("mapping_id")

	var req VirtualModelMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// 检查虚拟模型是否存在
	_, err := gorm.G[models.VirtualModel](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Virtual model not found")
		return
	}

	// 检查映射是否存在
	mapping, err := gorm.G[models.VirtualModelMapping](models.DB).
		Where("id = ? AND virtual_model_id = ?", mappingID, id).
		First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Mapping not found")
		return
	}

	enabled := req.Enabled

	mapping.Priority = req.Priority
	mapping.Weight = req.Weight
	mapping.Enabled = &enabled

	if _, err := gorm.G[models.VirtualModelMapping](models.DB).
		Where("id = ?", mappingID).
		Updates(c.Request.Context(), mapping); err != nil {
		common.InternalServerError(c, "Failed to update mapping: "+err.Error())
		return
	}

	common.Success(c, mapping)
}

// DeleteVirtualModelMapping 删除虚拟模型映射
func DeleteVirtualModelMapping(c *gin.Context) {
	id := c.Param("id")
	mappingID := c.Param("mapping_id")

	// 检查虚拟模型是否存在
	_, err := gorm.G[models.VirtualModel](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Virtual model not found")
		return
	}

	// 检查映射是否存在
	_, err = gorm.G[models.VirtualModelMapping](models.DB).
		Where("id = ? AND virtual_model_id = ?", mappingID, id).
		First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Mapping not found")
		return
	}

	// 删除映射
	if _, err := gorm.G[models.VirtualModelMapping](models.DB).
		Where("id = ?", mappingID).
		Delete(c.Request.Context()); err != nil {
		common.InternalServerError(c, "Failed to delete mapping: "+err.Error())
		return
	}

	common.Success(c, gin.H{"message": "Mapping deleted successfully"})
}

// GetVirtualModelStats 获取虚拟模型统计信息
func GetVirtualModelStats(c *gin.Context) {
	id := c.Param("id")

	// 检查虚拟模型是否存在
	_, err := gorm.G[models.VirtualModel](models.DB).Where("id = ?", id).First(c.Request.Context())
	if err != nil {
		common.NotFound(c, "Virtual model not found")
		return
	}

	virtualModelService := service.NewVirtualModelService(models.DB)
	idUint, _ := strconv.ParseUint(id, 10, 32)
	stats, err := virtualModelService.GetVirtualModelStats(c.Request.Context(), uint(idUint))
	if err != nil {
		common.InternalServerError(c, "Failed to get stats: "+err.Error())
		return
	}

	common.Success(c, stats)
}
