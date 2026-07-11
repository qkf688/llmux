package modelsynclogs

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/repository"
	"github.com/gin-gonic/gin"
)

func repos() *repository.Repositories {
	return repository.Default()
}

// GetModelSyncLogs 获取模型同步日志列表。
func GetModelSyncLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	list, err := repos().ModelSyncLog.List(c.Request.Context(), repository.ModelSyncLogListOptions{
		Filter: repository.ModelSyncLogFilter{
			ProviderID:    c.Query("provider_id"),
			Status:        c.Query("status"),
			ShowUnchanged: c.Query("show_unchanged") == "true",
		},
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		if strings.HasPrefix(err.Error(), "invalid status:") {
			common.BadRequest(c, "Invalid status parameter (must be success, error, or unchanged)")
			return
		}
		common.InternalServerError(c, "Failed to get logs: "+err.Error())
		return
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	common.Success(c, map[string]interface{}{
		"data": list.Logs,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total":       list.Total,
			"total_pages": (list.Total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// DeleteModelSyncLogs 批量删除模型同步日志。
func DeleteModelSyncLogs(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}
	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	deleted, err := repos().ModelSyncLog.HardDeleteByIDs(c.Request.Context(), req.IDs)
	if err != nil {
		common.InternalServerError(c, "Failed to delete logs: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": deleted,
	})
}

// ClearModelSyncLogs 清空所有模型同步日志。
func ClearModelSyncLogs(c *gin.Context) {
	deleted, err := repos().ModelSyncLog.HardDeleteAll(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to clear logs: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": deleted,
	})
}

// ClearModelSyncErrorLogs 清空错误模型同步日志。
func ClearModelSyncErrorLogs(c *gin.Context) {
	var req struct {
		ProviderIDs []uint `json:"provider_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	deleted, err := repos().ModelSyncLog.HardDeleteErrors(c.Request.Context(), req.ProviderIDs)
	if err != nil {
		common.InternalServerError(c, "Failed to clear error logs: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": deleted,
	})
}