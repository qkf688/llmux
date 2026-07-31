package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/modelsync"
)

// RegisterModelSync 聚合模型同步触发与日志路由。
func RegisterModelSync(rg gin.IRoutes) {
	modelsync.Register(rg)
}
