package modelapi

import (
	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BatchDeleteModels 批量删除模型。
func BatchDeleteModels(c *gin.Context) {
	var req BatchDeleteModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}
	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	result, err := gorm.G[models.Model](models.DB).Where("id IN ?", req.IDs).Delete(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to delete models: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": result,
	})
}

// BatchUpdateModels 批量更新模型参数。
func BatchUpdateModels(c *gin.Context) {
	var req BatchUpdateModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "参数验证失败: "+err.Error())
		return
	}

	if req.MaxRetry == nil && req.TimeOut == nil && req.AutoAssociate == nil {
		common.BadRequest(c, "至少需要更新一个字段")
		return
	}
	if req.MaxRetry != nil && *req.MaxRetry < 0 {
		common.BadRequest(c, "重试次数不能为负数")
		return
	}
	if req.TimeOut != nil && *req.TimeOut < 0 {
		common.BadRequest(c, "超时时间不能为负数")
		return
	}

	updates := make(map[string]interface{})
	if req.MaxRetry != nil {
		updates["max_retry"] = *req.MaxRetry
	}
	if req.TimeOut != nil {
		updates["time_out"] = *req.TimeOut
	}
	if req.AutoAssociate != nil {
		updates["auto_associate"] = req.AutoAssociate
	}

	result := models.DB.Model(&models.Model{}).
		Where("id IN ?", req.IDs).
		Updates(updates)
	if result.Error != nil {
		common.InternalServerError(c, "更新失败: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"updated": result.RowsAffected,
	})
}
