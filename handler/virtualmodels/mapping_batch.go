package virtualmodels

import (
	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
)

// BatchDeleteVirtualModelMapping 批量删除虚拟模型映射（硬删）。
func BatchDeleteVirtualModelMapping(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req BatchDeleteVirtualModelMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}
	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}

	ctx := c.Request.Context()
	if _, err := repos().VirtualModel.Get(ctx, id); err != nil {
		httpresp.NotFound(c, "Virtual model not found")
		return
	}

	deleted, err := repos().VirtualModelMapping.BatchDelete(ctx, id, req.IDs)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to delete mappings: "+err.Error())
		return
	}

	httpresp.Success(c, gin.H{"deleted": deleted})
}

// BatchCreateVirtualModelMapping 批量创建虚拟模型映射。
func BatchCreateVirtualModelMapping(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req BatchVirtualModelMappingRequest
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

	realModelIDs := make([]uint, 0, len(req.Mappings))
	for _, mapping := range req.Mappings {
		realModelIDs = append(realModelIDs, mapping.RealModelID)
	}

	existingModels, err := repos().Model.ListByIDs(ctx, realModelIDs)
	if err != nil {
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	existingModelMap := make(map[uint]bool)
	for _, model := range existingModels {
		existingModelMap[model.ID] = true
	}

	existingMappings, err := repos().VirtualModelMapping.ListByVirtualModelAndRealModelIDs(ctx, id, realModelIDs)
	if err != nil {
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}

	existingMappingMap := make(map[uint]bool)
	for _, mapping := range existingMappings {
		existingMappingMap[mapping.RealModelID] = true
	}

	result := BatchCreateResult{
		SuccessItems: make([]models.VirtualModelMapping, 0),
		FailedItems:  make([]BatchFailedItem, 0),
	}

	virtualModelService := service.NewVirtualModelService(models.DB)

	for _, mappingReq := range req.Mappings {
		if !existingModelMap[mappingReq.RealModelID] {
			result.FailedItems = append(result.FailedItems, BatchFailedItem{
				RealModelID: mappingReq.RealModelID,
				Reason:      "Real model not found",
			})
			result.FailedCount++
			continue
		}

		if existingMappingMap[mappingReq.RealModelID] {
			result.FailedItems = append(result.FailedItems, BatchFailedItem{
				RealModelID: mappingReq.RealModelID,
				Reason:      "Mapping already exists",
			})
			result.FailedCount++
			continue
		}

		if err := virtualModelService.ValidateNoCircularDependency(ctx, virtualModel.ID, mappingReq.RealModelID); err != nil {
			result.FailedItems = append(result.FailedItems, BatchFailedItem{
				RealModelID: mappingReq.RealModelID,
				Reason:      err.Error(),
			})
			result.FailedCount++
			continue
		}

		enabled := mappingReq.Enabled
		mapping := models.VirtualModelMapping{
			VirtualModelID: virtualModel.ID,
			RealModelID:    mappingReq.RealModelID,
			Priority:       mappingReq.Priority,
			Weight:         mappingReq.Weight,
			Enabled:        &enabled,
		}

		if err := repos().VirtualModelMapping.Create(ctx, &mapping); err != nil {
			result.FailedItems = append(result.FailedItems, BatchFailedItem{
				RealModelID: mappingReq.RealModelID,
				Reason:      "Failed to create mapping: " + err.Error(),
			})
			result.FailedCount++
			continue
		}

		result.SuccessItems = append(result.SuccessItems, mapping)
		result.SuccessCount++
		existingMappingMap[mappingReq.RealModelID] = true
	}

	httpresp.Success(c, result)
}