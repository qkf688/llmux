package logs

import "github.com/gin-gonic/gin"

// Register 挂接请求日志与 user-agents 叶子路由（不含 vacuum）。
// 注意：clear* / batch 先于 /logs/:id。
func Register(rg gin.IRoutes) {
	rg.GET("/logs", GetRequestLogs)
	rg.GET("/logs/:id", GetRequestLogDetail)
	rg.GET("/logs/:id/chat-io", GetChatIO)
	rg.DELETE("/logs/batch", BatchDeleteLogs)
	rg.DELETE("/logs/clear", ClearAllLogs)
	rg.DELETE("/logs/clear-filtered", ClearFilteredLogs)
	rg.DELETE("/logs/:id", DeleteLog)
	rg.GET("/user-agents", GetUserAgents)
}