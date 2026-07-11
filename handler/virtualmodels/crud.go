package virtualmodels

import (
	"errors"
	"strconv"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetVirtualModels 获取虚拟模型列表。
func GetVirtualModels(c *gin.Context) {
	list, err := repos().VirtualModel.List(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}
	httpresp.Success(c, list)
}

// CreateVirtualModel 创建虚拟模型。
func CreateVirtualModel(c *gin.Context) {
	var req VirtualModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()
	exists, err := repos().VirtualModel.ExistsByName(ctx, req.Name)
	if err != nil {
		httpresp.InternalServerError(c, "Database error: "+err.Error())
		return
	}
	if exists {
		httpresp.BadRequest(c, "Virtual model name already exists")
		return
	}

	if _, err := repos().Model.GetByName(ctx, req.Name); err == nil {
		httpresp.BadRequest(c, "Virtual model name conflicts with existing real model")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		httpresp.InternalServerError(c, "Database error: "+err.Error())
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

	if err := repos().VirtualModel.Create(ctx, &virtualModel); err != nil {
		httpresp.InternalServerError(c, "Failed to create virtual model: "+err.Error())
		return
	}

	httpresp.Success(c, virtualModel)
}

// UpdateVirtualModel 更新虚拟模型。
func UpdateVirtualModel(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var req VirtualModelRequest
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

	if req.Name != virtualModel.Name {
		exists, err := repos().VirtualModel.ExistsByNameExceptID(ctx, req.Name, id)
		if err != nil {
			httpresp.InternalServerError(c, "Database error: "+err.Error())
			return
		}
		if exists {
			httpresp.BadRequest(c, "Virtual model name already exists")
			return
		}

		if _, err := repos().Model.GetByName(ctx, req.Name); err == nil {
			httpresp.BadRequest(c, "Virtual model name conflicts with existing real model")
			return
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.InternalServerError(c, "Database error: "+err.Error())
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

	if err := repos().VirtualModel.Update(ctx, id, virtualModel); err != nil {
		httpresp.InternalServerError(c, "Failed to update virtual model: "+err.Error())
		return
	}

	httpresp.Success(c, virtualModel)
}

// DeleteVirtualModel 删除虚拟模型（映射硬删，VM 本体软删）。
func DeleteVirtualModel(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if _, err := repos().VirtualModel.Get(ctx, id); err != nil {
		httpresp.NotFound(c, "Virtual model not found")
		return
	}

	if _, err := repos().VirtualModelMapping.HardDeleteByVirtualModelID(ctx, id); err != nil {
		httpresp.InternalServerError(c, "Failed to delete virtual model mappings: "+err.Error())
		return
	}

	if _, err := repos().VirtualModel.Delete(ctx, id); err != nil {
		httpresp.InternalServerError(c, "Failed to delete virtual model: "+err.Error())
		return
	}

	httpresp.Success(c, gin.H{"message": "Virtual model deleted successfully"})
}

func parseIDParam(c *gin.Context, key string) (uint, bool) {
	raw := c.Param(key)
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		// 与旧 gorm Where("id = ?", string) 一致：非法 id 走 not found 路径
		return 0, true
	}
	return uint(v), true
}