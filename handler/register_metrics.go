package handler

import (
	"github.com/atopos31/llmio/handler/metrics"
	"github.com/gin-gonic/gin"
)

// RegisterMetrics 注册 /api/metrics/* 路由。
func RegisterMetrics(rg gin.IRoutes) {
	metrics.Register(rg)
}
