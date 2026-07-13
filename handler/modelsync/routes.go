package modelsync

import "github.com/gin-gonic/gin"

// Register 挂接模型同步触发与日志叶子路由。
func Register(rg gin.IRoutes) {
	rg.POST("/model-sync/:id", SyncProviderModels)
	rg.POST("/model-sync/all", SyncAllProviders)
	rg.GET("/model-sync/stats", GetModelSyncStats)
	rg.GET("/model-sync/logs", GetModelSyncLogs)
	rg.GET("/model-sync/recent-added-models", GetRecentAddedModels)
	rg.DELETE("/model-sync/logs", DeleteModelSyncLogs)
	rg.DELETE("/model-sync/logs/clear", ClearModelSyncLogs)
	rg.DELETE("/model-sync/logs/clear-errors", ClearModelSyncErrorLogs)
}
