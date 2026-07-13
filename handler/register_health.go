package handler

import (
	"github.com/atopos31/llmio/handler/healthcheck"
	"github.com/atopos31/llmio/handler/settings"
	"github.com/gin-gonic/gin"
)

// RegisterHealthCheck 聚合健康检查域路由（settings + run/logs）。
func RegisterHealthCheck(rg gin.IRoutes) {
	rg.GET("/health-check/settings", settings.GetHealthCheckSettings)
	rg.PUT("/health-check/settings", settings.UpdateHealthCheckSettings)
	rg.GET("/health-check/logs", healthcheck.GetHealthCheckLogs)
	rg.DELETE("/health-check/logs", healthcheck.ClearHealthCheckLogs)
	rg.POST("/health-check/run/:id", healthcheck.RunHealthCheck)
	rg.POST("/health-check/run-all", healthcheck.RunHealthCheckAll)
	rg.GET("/health-check/batch/:batchId", healthcheck.GetBatchHealthCheckStatus)
}
