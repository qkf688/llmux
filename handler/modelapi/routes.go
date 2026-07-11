package modelapi

import "github.com/gin-gonic/gin"

// Register 挂接模型管理叶子路由。
// 注意：/models/batch 必须先于 /models/:id。
func Register(rg gin.IRoutes) {
	rg.GET("/models", GetModels)
	rg.POST("/models", CreateModel)
	rg.PUT("/models/:id", UpdateModel)
	rg.GET("/models/:id/template", GetModelTemplate)
	rg.POST("/models/:id/template/items", AddModelTemplateItem)
	rg.DELETE("/models/:id/template/items", DeleteModelTemplateItem)
	rg.DELETE("/models/batch", BatchDeleteModels)
	rg.PUT("/models/batch", BatchUpdateModels)
	rg.DELETE("/models/:id", DeleteModel)
}