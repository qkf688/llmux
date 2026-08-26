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

	if req.AutoAssociate == nil {
		httpresp.BadRequest(c, "至少需要更新一个字段")
		return
	}

	updates := map[string]any{
		"auto_associate": req.AutoAssociate,
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
