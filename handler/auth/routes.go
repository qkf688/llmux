package auth

import "github.com/gin-gonic/gin"

// RegisterLogin 挂接无需鉴权的登录路由（在 AuthJWT 中间件之前挂）。
// secret 通过闭包注入 Login handler，避免包级可变状态。
func RegisterLogin(rg gin.IRoutes, secret string) {
	rg.POST("/auth/login", Login(secret))
}

// Register 挂接需要 JWT 鉴权的 auth 路由。
func Register(rg gin.IRoutes) {
	rg.GET("/auth/me", Me)
	rg.POST("/auth/api-key/rotate", RotateAPIKey)
	rg.POST("/auth/password", ChangePassword)
}
