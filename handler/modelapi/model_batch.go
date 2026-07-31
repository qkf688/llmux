package modelapi

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/repository"
)

// BatchDeleteModels 批量删除模型。
func BatchDeleteModels(c *gin.Context) {
	var req BatchDeleteModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}
	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}

	ctx := c.Request.Context()
	r := repos()
	var result int64
	err := r.RunInTx(ctx, func(tx *repository.Repositories) error {
		for _, id := range req.IDs {
			if err := deleteModelAssociations(ctx, tx, id); err != nil {
				return err
			}
		}
		affected, err := tx.Model.DeleteByIDs(ctx, req.IDs)
		if err != nil {
			return err
		}
		result = affected
		return nil
	})
	if err != nil {
		httpresp.InternalServerError(c, "Failed to delete models: "+err.Error())
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"deleted": result,
	})
}

// BatchUpdateModels 批量更新模型参数。
func BatchUpdateModels(c *gin.Context) {
	var req BatchUpdateModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "参数验证失败: "+err.Error())
		return
	}

	if req.MaxRetry == nil && req.TimeOut == nil && req.AutoAssociate == nil {
		httpresp.BadRequest(c, "至少需要更新一个字段")
		return
	}
	if req.MaxRetry != nil && *req.MaxRetry < 0 {
		httpresp.BadRequest(c, "重试次数不能为负数")
		return
	}
	if req.TimeOut != nil && *req.TimeOut < 0 {
		httpresp.BadRequest(c, "超时时间不能为负数")
		return
	}

	updates := make(map[string]any)
	if req.MaxRetry != nil {
		updates["max_retry"] = *req.MaxRetry
	}
	if req.TimeOut != nil {
		updates["time_out"] = *req.TimeOut
	}
	if req.AutoAssociate != nil {
		updates["auto_associate"] = req.AutoAssociate
	}

	updated, err := repos().Model.BatchUpdate(c.Request.Context(), req.IDs, updates)
	if err != nil {
		httpresp.InternalServerError(c, "更新失败: "+err.Error())
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"updated": updated,
	})
}
