package handler

import (
	"github.com/atopos31/llmio/handler/healthchecklogs"
	"github.com/atopos31/llmio/handler/settings"
	"github.com/gin-gonic/gin"
)

// RegisterHealthCheck 聚合健康检查域路由（settings + logs + run*）。
func RegisterHealthCheck(rg gin.IRoutes) {
	rg.GET("/health-check/settings", settings.GetHealthCheckSettings)
	rg.PUT("/health-check/settings", settings.UpdateHealthCheckSettings)
	rg.GET("/health-check/logs", healthchecklogs.GetHealthCheckLogs)
	rg.DELETE("/health-check/logs", healthchecklogs.ClearHealthCheckLogs)
	rg.POST("/health-check/run/:id", RunHealthCheck)
	rg.POST("/health-check/run-all", RunHealthCheckAll)
	rg.GET("/health-check/batch/:batchId", GetBatchHealthCheckStatus)
}