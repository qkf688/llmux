package pools

import "github.com/gin-gonic/gin"

// Register 挂接号池叶子路由。
func Register(rg gin.IRoutes) {
	rg.GET("/pools", GetPools)
	rg.POST("/pools", CreatePool)
	rg.PUT("/pools/:id", UpdatePool)
	rg.DELETE("/pools/:id", DeletePool)
	rg.GET("/pools/:id/credentials", ListCredentials)
}
