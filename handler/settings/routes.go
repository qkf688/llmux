package settings

import "github.com/gin-gonic/gin"

// RegisterSystemConfig 注册 /config 系统配置路由。
func RegisterSystemConfig(rg gin.IRoutes) {
	rg.GET("/config", GetSystemConfig)
	rg.PUT("/config", UpdateSystemConfig)
}

// Register 挂接通用 settings 叶子路由（不含 health-check settings）。
func Register(rg gin.IRoutes) {
	rg.GET("/settings", GetSettings)
	rg.PUT("/settings", UpdateSettings)
	rg.POST("/settings/reset-weights", ResetModelWeights)
	rg.POST("/settings/reset-priorities", ResetModelPriorities)
	rg.POST("/settings/enable-all-associations", EnableAllAssociations)
}
