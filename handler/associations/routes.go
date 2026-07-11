package associations

import "github.com/gin-gonic/gin"

// Register 挂接模型-供应商关联叶子路由。
// 注意：batch 路径先于 /:id。
func Register(rg gin.IRoutes) {
	rg.GET("/model-providers", GetModelProviders)
	rg.GET("/model-providers/status", GetModelProviderStatus)
	rg.GET("/model-providers/health-status", GetModelProviderHealthStatus)
	rg.POST("/model-providers", CreateModelProvider)
	rg.PUT("/model-providers/:id", UpdateModelProvider)
	rg.PATCH("/model-providers/:id/status", UpdateModelProviderStatus)
	rg.PATCH("/model-providers/batch/status", BatchUpdateModelProvidersStatus)
	rg.PATCH("/model-providers/batch/capabilities", BatchUpdateModelProvidersCapabilities)
	rg.DELETE("/model-providers/batch", BatchDeleteModelProviders)
	rg.DELETE("/model-providers/:id", DeleteModelProvider)
}