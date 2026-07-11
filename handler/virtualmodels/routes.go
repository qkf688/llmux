package virtualmodels

import "github.com/gin-gonic/gin"

// Register 挂接虚拟模型叶子路由。
// 注意：mappings/batch 先于 mappings/:mapping_id。
func Register(rg gin.IRoutes) {
	rg.GET("/virtual-models", GetVirtualModels)
	rg.POST("/virtual-models", CreateVirtualModel)
	rg.PUT("/virtual-models/:id", UpdateVirtualModel)
	rg.DELETE("/virtual-models/:id", DeleteVirtualModel)
	rg.GET("/virtual-models/:id/mappings", GetVirtualModelMappings)
	rg.POST("/virtual-models/:id/mappings", CreateVirtualModelMapping)
	rg.POST("/virtual-models/:id/mappings/batch", BatchCreateVirtualModelMapping)
	rg.DELETE("/virtual-models/:id/mappings/batch", BatchDeleteVirtualModelMapping)
	rg.PUT("/virtual-models/:id/mappings/:mapping_id", UpdateVirtualModelMapping)
	rg.DELETE("/virtual-models/:id/mappings/:mapping_id", DeleteVirtualModelMapping)
	rg.GET("/virtual-models/:id/stats", GetVirtualModelStats)
}