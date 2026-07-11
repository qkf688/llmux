package handler

import (
	"github.com/atopos31/llmio/handler/modelsynclogs"
	"github.com/gin-gonic/gin"
)

// RegisterModelSync 聚合模型同步触发与日志路由。
func RegisterModelSync(rg gin.IRoutes) {
	rg.POST("/model-sync/:id", SyncProviderModels)
	rg.POST("/model-sync/all", SyncAllProviders)
	rg.GET("/model-sync/stats", GetModelSyncStats)
	rg.GET("/model-sync/logs", modelsynclogs.GetModelSyncLogs)
	rg.GET("/model-sync/recent-added-models", GetRecentAddedModels)
	rg.DELETE("/model-sync/logs", modelsynclogs.DeleteModelSyncLogs)
	rg.DELETE("/model-sync/logs/clear", modelsynclogs.ClearModelSyncLogs)
	rg.DELETE("/model-sync/logs/clear-errors", modelsynclogs.ClearModelSyncErrorLogs)
}