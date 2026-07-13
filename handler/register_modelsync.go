package handler

import (
	"github.com/atopos31/llmio/handler/modelsync"
	"github.com/gin-gonic/gin"
)

// RegisterModelSync 聚合模型同步触发与日志路由。
func RegisterModelSync(rg gin.IRoutes) {
	modelsync.Register(rg)
}
