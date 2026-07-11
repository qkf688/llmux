package handler

import (
	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

type ProviderMetricRes struct {
	ProviderID      uint    `json:"provider_id"`
	ProviderName    string  `json:"provider_name"`
	SuccessRate     float64 `json:"success_rate"`
	AvgResponseTime int64   `json:"avg_response_time"`
	TotalRequests   int64   `json:"total_requests"`
	TotalTokens     int64   `json:"total_tokens"`
	SuccessCount    int64   `json:"success_count"`
	FailureCount    int64   `json:"failure_count"`
}

func ProviderMetrics(c *gin.Context) {
	// 从供应商统计表直接读取，不再依赖 chat_logs 聚合
	rows := make([]models.StatsProviderTotal, 0)
	if err := models.DB.WithContext(c.Request.Context()).
		Order("total_requests DESC").
		Find(&rows).Error; err != nil {
		httpresp.InternalServerError(c, "Failed to query provider metrics: "+err.Error())
		return
	}

	res := make([]ProviderMetricRes, 0, len(rows))
	for _, row := range rows {
		successRate := 0.0
		avgResponseTime := int64(0)
		if row.TotalRequests > 0 {
			successRate = float64(row.SuccessCount) / float64(row.TotalRequests)
			avgResponseTime = row.AvgResponseTime / row.TotalRequests
		}
		res = append(res, ProviderMetricRes{
			ProviderID:      0, // 统计表不存储 provider_id，前端通过名称匹配
			ProviderName:    row.ProviderName,
			SuccessRate:     successRate,
			AvgResponseTime: avgResponseTime,
			TotalRequests:   row.TotalRequests,
			TotalTokens:     row.TotalTokens,
			SuccessCount:    row.SuccessCount,
			FailureCount:    row.FailureCount,
		})
	}

	httpresp.Success(c, res)
}
