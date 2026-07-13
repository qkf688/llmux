package metrics

import "github.com/gin-gonic/gin"

// Register 挂接 /api/metrics/* 叶子路由。
func Register(rg gin.IRoutes) {
	rg.GET("/metrics/use/:days", Metrics)
	rg.GET("/metrics/dailies/:days", MetricsDailies)
	rg.GET("/metrics/hourlies/today", MetricsHourliesToday)
	rg.GET("/metrics/counts", Counts)
	rg.GET("/metrics/real-model-counts", RealModelCounts)
	rg.GET("/metrics/total", MetricsTotal)
	rg.GET("/metrics/providers", ProviderMetrics)
}
