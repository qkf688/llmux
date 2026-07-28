package database

import "github.com/gin-gonic/gin"

// RegisterVacuum 注册维护 vacuum（main 中紧随 logs）。
func RegisterVacuum(rg gin.IRoutes) {
	rg.POST("/maintenance/vacuum", VacuumDatabase)
}

// RegisterDatabaseStats 注册数据库统计。
func RegisterDatabaseStats(rg gin.IRoutes) {
	rg.GET("/system/database-stats", GetDatabaseStats)
}

// RegisterExportDatabase 注册数据库文件导出。
func RegisterExportDatabase(rg gin.IRoutes) {
	rg.GET("/system/export-database", ExportDatabase)
}

// Register 挂接本包全部叶子路由。
// 若需严格对齐 main 现序（export-config 夹在中间），请拆分调用。
func Register(rg gin.IRoutes) {
	RegisterVacuum(rg)
	RegisterDatabaseStats(rg)
	RegisterExportDatabase(rg)
}
