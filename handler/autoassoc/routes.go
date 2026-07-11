package autoassoc

import "github.com/gin-gonic/gin"

// RegisterAssociate 自动关联预览与执行（main 中 blacklist 之前）。
func RegisterAssociate(rg gin.IRoutes) {
	rg.GET("/model-providers/auto-associate/preview", PreviewAutoAssociate)
	rg.POST("/model-providers/auto-associate", AutoAssociateModels)
}

// RegisterClean 清理无效关联（main 中 blacklist 之后）。
func RegisterClean(rg gin.IRoutes) {
	rg.GET("/model-providers/clean-invalid/preview", PreviewCleanInvalid)
	rg.POST("/model-providers/clean-invalid", CleanInvalidAssociations)
}

// Register 挂接本包全部叶子路由。
// 若需严格对齐 main 现序（中间夹 blacklist），请用 RegisterAssociate / RegisterClean。
func Register(rg gin.IRoutes) {
	RegisterAssociate(rg)
	RegisterClean(rg)
}