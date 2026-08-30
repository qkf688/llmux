package pools

import "github.com/gin-gonic/gin"

// Register 挂接号池叶子路由。
func Register(rg gin.IRoutes) {
	rg.GET("/pools", GetPools)
	rg.POST("/pools", CreatePool)
	rg.PUT("/pools/:id", UpdatePool)
	rg.DELETE("/pools/:id", DeletePool)
	rg.GET("/pools/:id/credentials", ListCredentials)
	rg.POST("/pools/:id/credentials", CreateCredential)
	// batch 固定段优先于 :credId，避免 "batch" 被当成 credId 捕获
	rg.PATCH("/pools/:id/credentials/batch/status", BatchUpdateCredentialStatus)
	rg.DELETE("/pools/:id/credentials/batch", BatchDeleteCredentials)
	rg.GET("/pools/:id/credentials/:credId", GetCredential)
	rg.GET("/pools/:id/credentials/:credId/raw", GetCredentialRaw)
	rg.PATCH("/pools/:id/credentials/:credId", UpdateCredential)
	rg.DELETE("/pools/:id/credentials/:credId", DeleteCredential)
}
