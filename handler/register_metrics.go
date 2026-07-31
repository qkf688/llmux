package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/metrics"
)

// RegisterMetrics 注册 /api/metrics/* 路由。
func RegisterMetrics(rg gin.IRoutes) {
	metrics.Register(rg)
}
