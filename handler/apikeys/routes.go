// Package apikeys 提供 /v1 调用 key（api_keys 表）的管理端 CRUD。
// 多 api-key 方案切片 A：key 值创建后不可改（轮换 = 新建 + 停用/删除旧），
// 列表/创建/更新回显明文支撑管理端复制——全部响应走 no-store。
package apikeys

import "github.com/gin-gonic/gin"

// Register 把 apikeys 域挂到已受 AuthJWT 保护的 /api 组（handler.RegisterAll 调用）。
func Register(rg gin.IRoutes) {
	rg.POST("/api-keys", CreateApiKey)
	rg.GET("/api-keys", ListApiKeys)
	rg.PATCH("/api-keys/:id", UpdateApiKey)
	rg.DELETE("/api-keys/:id", DeleteApiKey)
}
