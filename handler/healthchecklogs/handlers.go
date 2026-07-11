package healthchecklogs

import (
	"strconv"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/repository"
	"github.com/gin-gonic/gin"
)

func repos() *repository.Repositories {
	return repository.Default()
}

// GetHealthCheckLogs 获取健康检测日志（支持分页和筛选）。
func GetHealthCheckLogs(c *gin.Context) {
	pageStr := c.Query("page")
	page := 1
	if pageStr != "" {
		parsedPage, err := strconv.Atoi(pageStr)
		if err != nil || parsedPage < 1 {
			common.BadRequest(c, "Invalid page parameter")
			return
		}
		page = parsedPage
	}

	pageSizeStr := c.Query("page_size")
	pageSize := 20
	if pageSizeStr != "" {
		parsedPageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || parsedPageSize < 1 || parsedPageSize > 100 {
			common.BadRequest(c, "Invalid page_size parameter (must be between 1 and 100)")
			return
		}
		pageSize = parsedPageSize
	}

	list, err := repos().HealthCheckLog.List(c.Request.Context(), repository.HealthCheckLogListOptions{
		Filter: repository.HealthCheckLogFilter{
			ModelProviderID: c.Query("model_provider_id"),
			ModelName:       c.Query("model_name"),
			ProviderName:    c.Query("provider_name"),
			Status:          c.Query("status"),
		},
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		common.InternalServerError(c, "Failed to query health check logs: "+err.Error())
		return
	}

	common.Success(c, map[string]any{
		"data":      list.Logs,
		"total":     list.Total,
		"page":      page,
		"page_size": pageSize,
		"pages":     (list.Total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// ClearHealthCheckLogs 清空健康检测日志。
func ClearHealthCheckLogs(c *gin.Context) {
	deleted, err := repos().HealthCheckLog.HardDeleteAll(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to clear health check logs: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"deleted": deleted,
	})
}