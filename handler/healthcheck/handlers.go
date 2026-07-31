package healthcheck

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/repository"
)

func repos() *repository.Repositories {
	return repository.Default()
}

// GetHealthCheckLogs 获取健康检测日志（支持分页和筛选）。
func GetHealthCheckLogs(c *gin.Context) {
	page, pageSize, ok := httpx.ParsePaginationStrict(c)
	if !ok {
		return
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
		httpresp.InternalServerError(c, "Failed to query health check logs: "+err.Error())
		return
	}

	httpresp.Success(c, map[string]any{
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
		httpresp.InternalServerError(c, "Failed to clear health check logs: "+err.Error())
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"deleted": deleted,
	})
}
