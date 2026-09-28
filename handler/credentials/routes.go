package credentials

import "github.com/gin-gonic/gin"

// Register 挂接凭据域叶子路由。
//
// 固定段（status / batch）必须先于 :id 注册：gin 会把静态段优先匹配，但注册顺序仍是
// 「静态在前」的显式契约（同 pools 的 batch 与 :credId 关系，有测试锁定）。
func Register(rg gin.IRoutes) {
	rg.GET("/credentials", ListCredentials)
	rg.POST("/credentials", CreateCredential)
	rg.PATCH("/credentials/status", BatchUpdateCredentialStatus)
	rg.DELETE("/credentials/batch", BatchDeleteCredentials)
	rg.GET("/credentials/:id", GetCredential)
	rg.PATCH("/credentials/:id", UpdateCredential)
	rg.DELETE("/credentials/:id", DeleteCredential)
	// 明文查看（受控端点）：列表/详情恒为掩码，明文只在「显示」动作时单独拉取
	rg.GET("/credentials/:id/raw", GetCredentialRaw)
}
