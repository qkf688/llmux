package pools

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// BatchUpdateCredentialStatus 批量启停同一号池下的凭据（池内限界，不越池）。
// PATCH /api/pools/:id/credentials/batch/status
func BatchUpdateCredentialStatus(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	var req BatchCredentialStatusRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}
	if !models.IsValidCredentialStatus(req.Status) {
		httpresp.BadRequest(c, "Invalid status")
		return
	}
	ctx := c.Request.Context()
	if _, err := repos().Pool.Get(ctx, poolID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Pool not found")
			return
		}
		httpresp.InternalServerError(c, err.Error())
		return
	}
	updated, err := repos().Credential.UpdateStatusByIDs(ctx, poolID, req.IDs, req.Status)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to update status: "+err.Error())
		return
	}
	httpresp.Success(c, map[string]any{"updated": updated})
}

// BatchDeleteCredentials 批量删除同一号池下的凭据（池内限界，不越池）。
// DELETE /api/pools/:id/credentials/batch
func BatchDeleteCredentials(c *gin.Context) {
	poolID, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}
	var req BatchCredentialDeleteRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}
	ctx := c.Request.Context()
	if _, err := repos().Pool.Get(ctx, poolID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Pool not found")
			return
		}
		httpresp.InternalServerError(c, err.Error())
		return
	}
	deleted, err := repos().Credential.DeleteByIDs(ctx, poolID, req.IDs)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to delete credentials: "+err.Error())
		return
	}
	httpresp.Success(c, map[string]any{"deleted": deleted})
}
