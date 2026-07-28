package testapi

import "github.com/gin-gonic/gin"

// RegisterProviderTest 注册 POST /providers/:id/test（main 中夹在 provider 段内）。
func RegisterProviderTest(rg gin.IRoutes) {
	rg.POST("/providers/:id/test", ProviderModelTestHandler)
}

// Register 挂接连通性测试叶子路由（/test/*）。
func Register(rg gin.IRoutes) {
	rg.GET("/test/:id", ProviderTestHandler)
	rg.GET("/test/react/:id", TestReactHandler)
	rg.GET("/test/structured-output/:id", TestStructuredOutputHandler)
}
