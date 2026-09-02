package providerapi

import "github.com/gin-gonic/gin"

// RegisterHead 注册 provider 列表类路由（main 中 test 之前）。
func RegisterHead(rg gin.IRoutes) {
	rg.GET("/providers/template", GetProviderTemplates)
	rg.GET("/providers", GetProviders)
	rg.GET("/providers/model-catalog", GetProviderModelCatalog)
	rg.GET("/providers/:id", GetProvider)
	rg.GET("/providers/models/:id", GetProviderModels)
}

// RegisterCRUD 注册 provider 写操作与清关联（main 中 test 之后）。
func RegisterCRUD(rg gin.IRoutes) {
	rg.POST("/providers", CreateProvider)
	rg.PUT("/providers/:id", UpdateProvider)
	rg.DELETE("/providers/:id", DeleteProvider)
	rg.DELETE("/providers/:id/associations", ClearProviderAssociations)
}

// RegisterBlacklist 注册供应商黑名单（main 中位于 auto-associate 与 clean-invalid 之间）。
func RegisterBlacklist(rg gin.IRoutes) {
	rg.GET("/providers/blacklist", GetProviderBlacklist)
	rg.PUT("/providers/blacklist", UpdateProviderBlacklist)
}

// Register 挂接本包全部叶子路由（不含 POST /providers/:id/test，见 testapi）。
// 若需严格对齐 main 现序，请用 RegisterHead / RegisterCRUD / RegisterBlacklist。
func Register(rg gin.IRoutes) {
	RegisterHead(rg)
	RegisterCRUD(rg)
	RegisterBlacklist(rg)
}
