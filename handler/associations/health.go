package associations

import (
	"slices"
	"strconv"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetModelProviderStatus 获取提供商状态信息（最近 10 次请求成败）。
// ChatLog 只读聚合：日志 repo 在 T3.0b，此处暂留 query 辅助。
func GetModelProviderStatus(c *gin.Context) {
	providerIDStr := c.Query("provider_id")
	modelName := c.Query("model_name")
	providerModel := c.Query("provider_model")

	if providerIDStr == "" || modelName == "" || providerModel == "" {
		httpresp.BadRequest(c, "provider_id, model_name and provider_model query parameters are required")
		return
	}

	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		httpresp.BadRequest(c, "Invalid provider_id format")
		return
	}

	ctx := c.Request.Context()
	provider, err := repos().Provider.Get(ctx, uint(providerID))
	if err != nil {
		httpresp.InternalServerError(c, "Failed to retrieve provider: "+err.Error())
		return
	}

	logs, err := gorm.G[models.ChatLog](models.DB).
		Where("provider_name = ?", provider.Name).
		Where("provider_model = ?", providerModel).
		Where("name = ?", modelName).
		Limit(10).
		Order("created_at DESC").
		Find(ctx)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to retrieve chat log: "+err.Error())
		return
	}

	status := make([]bool, 0)
	for _, log := range logs {
		status = append(status, log.Status == "success")
	}
	slices.Reverse(status)
	httpresp.Success(c, status)
}

// GetModelProviderHealthStatus 获取模型提供商的健康检测结果。
// HealthCheckLog 只读聚合：日志 repo 在 T3.0b，此处暂留 query 辅助。
func GetModelProviderHealthStatus(c *gin.Context) {
	modelProviderIDStr := c.Query("model_provider_id")
	limitStr := c.Query("limit")

	if modelProviderIDStr == "" {
		httpresp.BadRequest(c, "model_provider_id query parameter is required")
		return
	}

	modelProviderID, err := strconv.ParseUint(modelProviderIDStr, 10, 64)
	if err != nil {
		httpresp.BadRequest(c, "Invalid model_provider_id format")
		return
	}

	limit := 10
	if limitStr != "" {
		parsed, parseErr := strconv.Atoi(limitStr)
		if parseErr != nil || parsed < 1 || parsed > 50 {
			httpresp.BadRequest(c, "Invalid limit parameter (must be between 1 and 50)")
			return
		}
		limit = parsed
	}

	logs, err := gorm.G[models.HealthCheckLog](models.DB).
		Where("model_provider_id = ?", modelProviderID).
		Order("checked_at DESC").
		Limit(limit).
		Find(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, "Failed to retrieve health check logs: "+err.Error())
		return
	}

	status := make([]bool, 0, len(logs))
	for _, log := range logs {
		status = append(status, log.Status == "success")
	}
	slices.Reverse(status)
	httpresp.Success(c, status)
}
