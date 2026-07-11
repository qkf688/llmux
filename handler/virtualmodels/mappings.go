package virtualmodels

import (
	"errors"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetVirtualModelMappings 获取虚拟模型的映射关系。
func GetVirtualModelMappings(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if _, err := repos().VirtualModel.Get(ctx, id); err != nil {
		httpresp.NotFound(c, "Virtual model not found")
		return
	}

	mappings, err := repos().VirtualModelMapping.ListByVirtualModel(ctx, id)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	httpresp.Success(c, mappings)
}

// CreateVirtualModelMapping 创建虚拟模型映射。
func CreateVirtualModelMapping(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req VirtualModelMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()
	virtualModel, err := repos().VirtualModel.Get(ctx, id)
	if err != nil {
		httpresp.NotFound(c, "Virtual model not found")
		return
	}

	if _, err := repos().Model.Get(ctx, req.RealModelID); err != nil {
		httpresp.NotFound(c, "Real model not found")
		return
	}

	count, err := repos().VirtualModelMapping.CountByPair(ctx, id, req.RealModelID)
	if err != nil {
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if count > 0 {
		httpresp.BadRequest(c, "Mapping already exists")
		return
	}

	// 循环依赖校验委托 service，禁止在 handler 重写
	virtualModelService := service.NewVirtualModelService(models.DB)
	if err := virtualModelService.ValidateNoCircularDependency(ctx, virtualModel.ID, req.RealModelID); err != nil {
		httpresp.BadRequest(c, err.Error())
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

	if err := repos().VirtualModelMapping.Create(ctx, &mapping); err != nil {
		httpresp.InternalServerError(c, "Failed to create mapping: "+err.Error())
		return
	}

	httpresp.Success(c, mapping)
}

// UpdateVirtualModelMapping 更新虚拟模型映射。
func UpdateVirtualModelMapping(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	mappingID, ok := parseIDParam(c, "mapping_id")
	if !ok {
		return
	}

	var req VirtualModelMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()
	if _, err := repos().VirtualModel.Get(ctx, id); err != nil {
		httpresp.NotFound(c, "Virtual model not found")
		return
	}

	mapping, err := repos().VirtualModelMapping.GetByVirtualModelAndID(ctx, id, mappingID)
	if err != nil {
		httpresp.NotFound(c, "Mapping not found")
		return
	}

	enabled := req.Enabled
	mapping.Priority = req.Priority
	mapping.Weight = req.Weight
	mapping.Enabled = &enabled

	if err := repos().VirtualModelMapping.Update(ctx, mappingID, mapping); err != nil {
		httpresp.InternalServerError(c, "Failed to update mapping: "+err.Error())
		return
	}

	httpresp.Success(c, mapping)
}

// DeleteVirtualModelMapping 删除虚拟模型映射（硬删）。
func DeleteVirtualModelMapping(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	mappingID, ok := parseIDParam(c, "mapping_id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if _, err := repos().VirtualModel.Get(ctx, id); err != nil {
		httpresp.NotFound(c, "Virtual model not found")
		return
	}

	if _, err := repos().VirtualModelMapping.GetByVirtualModelAndID(ctx, id, mappingID); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Mapping not found")
			return
		}
		httpresp.NotFound(c, "Mapping not found")
		return
	}

	if _, err := repos().VirtualModelMapping.DeleteByVirtualModelAndID(ctx, id, mappingID); err != nil {
		httpresp.InternalServerError(c, "Failed to delete mapping: "+err.Error())
		return
	}

	httpresp.Success(c, gin.H{"message": "Mapping deleted successfully"})
}