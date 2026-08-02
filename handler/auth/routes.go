package auth

import "github.com/gin-gonic/gin"

// jwtSecret 由 SetJWTSecret 设置，供 login/sign 使用。
var jwtSecret string

// SetJWTSecret 设置包级 JWT secret（由 register.go 在挂接前调用）。
func SetJWTSecret(s string) {
	jwtSecret = s
}

// RegisterLogin 挂接无需鉴权的登录路由（在 AuthJWT 中间件之前挂）。
func RegisterLogin(rg gin.IRoutes) {
	rg.POST("/auth/login", Login)
}

// Register 挂接需要 JWT 鉴权的 auth 路由。
func Register(rg gin.IRoutes) {
	rg.GET("/auth/me", Me)
	rg.POST("/auth/api-key/rotate", RotateAPIKey)
	rg.POST("/auth/password", ChangePassword)
}
