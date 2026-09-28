package credentials

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
)

// BatchUpdateCredentialStatus 批量启停凭据（按 ID，不设归属限界）。
// 内联凭据被判停后的人工恢复走这里（或单条 PATCH）——这是它唯一的状态写入口。
// PATCH /api/credentials/status
func BatchUpdateCredentialStatus(c *gin.Context) {
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
	// 恢复 active 时按状态机约定连带重置鉴权失败计数（单一来源 models.CredentialRecoveryFields）
	fields := map[string]any{"status": req.Status}
	for k, v := range models.CredentialRecoveryFields(req.Status) {
		fields[k] = v
	}
	updated, err := repos().Credential.UpdateFieldsByIDs(c.Request.Context(), req.IDs, fields)
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to update status", err)
		return
	}
	httpresp.Success(c, map[string]any{"updated": updated})
}

// BatchDeleteCredentials 批量删除凭据（按 ID 软删）。
// DELETE /api/credentials/batch
func BatchDeleteCredentials(c *gin.Context) {
	var req BatchCredentialDeleteRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}
	deleted, err := repos().Credential.DeleteByIDs(c.Request.Context(), req.IDs)
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to delete credentials", err)
		return
	}
	httpresp.Success(c, map[string]any{"deleted": deleted})
}
