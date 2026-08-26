package virtualmodels

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
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
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
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

	updates := map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"strategy":    req.Strategy,
		"io_log":      req.IOLog,
		"enabled":     req.Enabled,
	}
	if _, err := repos().VirtualModel.UpdateFields(ctx, id, updates); err != nil {
		httpresp.InternalServerError(c, "Failed to update virtual model: "+err.Error())
		return
	}

	// 重读返回最新 DB 值，避免本地 struct 与 DB 偏离
	updatedVM, err := repos().VirtualModel.Get(ctx, id)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to retrieve updated virtual model: "+err.Error())
		return
	}
	httpresp.Success(c, updatedVM)
}

// DeleteVirtualModel 删除虚拟模型（映射硬删，VM 本体软删）。
func DeleteVirtualModel(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
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
