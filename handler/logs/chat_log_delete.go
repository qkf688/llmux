package logs

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/repository"
)

// BatchDeleteLogsRequest 批量删除日志请求结构。
type BatchDeleteLogsRequest struct {
	IDs []uint `json:"ids"`
}

// DeleteLog 删除单条日志（先 ChatIO 硬删，再 ChatLog 硬删）。
func DeleteLog(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if err := repos().ChatIO.HardDeleteByLogID(ctx, id); err != nil {
		slog.Warn("failed to delete chat io record", "log_id", id, "error", err)
	}

	affected, err := repos().ChatLog.HardDelete(ctx, id)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to delete log: "+err.Error())
		return
	}
	if affected == 0 {
		httpresp.NotFound(c, "Log not found")
		return
	}

	httpresp.Success(c, nil)
}

// BatchDeleteLogs 批量删除日志。
func BatchDeleteLogs(c *gin.Context) {
	var req BatchDeleteLogsRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}

	ctx := c.Request.Context()
	if err := repos().ChatIO.HardDeleteByLogIDs(ctx, req.IDs); err != nil {
		slog.Warn("failed to delete chat io records", "error", err)
	}

	affected, err := repos().ChatLog.HardDeleteByIDs(ctx, req.IDs)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to delete logs: "+err.Error())
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"deleted": affected,
	})
}

// ClearAllLogs 清空所有日志。
func ClearAllLogs(c *gin.Context) {
	ctx := c.Request.Context()
	if err := repos().ChatIO.HardDeleteAll(ctx); err != nil {
		slog.Warn("failed to delete all chat io records", "error", err)
	}

	affected, err := repos().ChatLog.HardDeleteAll(ctx)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to clear logs: "+err.Error())
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"deleted": affected,
	})
}

// ClearFilteredLogs 清空满足筛选条件的请求日志（跨分页，事务内 ChatIO→ChatLog）。
func ClearFilteredLogs(c *gin.Context) {
	filter := repository.ChatLogFilter{
		ProviderName: strings.TrimSpace(c.Query("provider_name")),
		Name:         strings.TrimSpace(c.Query("name")),
		Status:       strings.TrimSpace(c.Query("status")),
		Style:        strings.TrimSpace(c.Query("style")),
		UserAgent:    strings.TrimSpace(c.Query("user_agent")),
	}

	if filter.IsEmpty() {
		httpresp.BadRequest(c, "At least one filter parameter is required")
		return
	}

	deleted, err := repos().ChatLog.HardDeleteFiltered(c.Request.Context(), filter)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to clear logs: "+err.Error())
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"deleted": deleted,
	})
}
