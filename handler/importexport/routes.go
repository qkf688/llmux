package importexport

import "github.com/gin-gonic/gin"

// RegisterExportConfig 仅 export-config（main：export-database 之前）。
func RegisterExportConfig(rg gin.IRoutes) {
	rg.GET("/system/export-config", ExportConfig)
}

// RegisterImportConfig 仅 import-config（main：export-database 之后）。
func RegisterImportConfig(rg gin.IRoutes) {
	rg.POST("/system/import-config", ImportConfig)
}

// Register 挂接本包全部叶子路由。
// 若需严格对齐 main 现序，请用 RegisterExportConfig / RegisterImportConfig。
func Register(rg gin.IRoutes) {
	RegisterExportConfig(rg)
	RegisterImportConfig(rg)
}