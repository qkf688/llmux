package associations

import (
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// GetModelProviderStatus 获取提供商状态信息（最近 10 次**有成败结论**的请求）。
//
// 口径是白名单「只统计 success 与 error」，而非黑名单逐个排除：在途（processing）、
// 用户主动撤走（canceled）、进程先走了（interrupted）三行既非成也非败，画成失败会
// 给出「这家供应商在失败」的假结论，而供应商没做错任何事；它们同时还会占掉 10 格的
// 名额、把真实成败挤出窗口。白名单写法让状态机再加新态时自动落到「不统计」一侧，
// 不必回来补一行排除（状态常量见 models.ChatLogStatus*）。
//
// 布尔返回形状是既有对外契约（前端 status-bars 只认成败两格），故非结论态是**排除**
// 而非新增第三格——这与 HealthCheckLog 侧的三态字符串（见 GetModelProviderHealthStatus）
// 是两条不同的路，不要合并。ChatLog 只读聚合：日志 repo 在 T3.0b，此处暂留 query 辅助。
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
		httpresp.InternalErrorMessage(c, "Failed to retrieve provider", err)
		return
	}

	logs, err := gorm.G[models.ChatLog](models.DB).
		Where("provider_name = ?", provider.Name).
		Where("provider_model = ?", providerModel).
		Where("name = ?", modelName).
		Where("status IN ?", []string{models.ChatLogStatusSuccess, models.ChatLogStatusError}).
		Limit(10).
		Order("created_at DESC").
		Find(ctx)
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to retrieve chat log", err)
		return
	}

	status := make([]bool, 0)
	for _, log := range logs {
		status = append(status, log.Status == models.ChatLogStatusSuccess)
	}
	slices.Reverse(status)
	httpresp.Success(c, status)
}

// GetModelProviderHealthStatus 获取模型提供商的健康检测结果。
// 返回最近 limit 条检测的**状态字符串**（success / error / not_measured），按时间升序。
//
// 为什么不再是 bool 列表（Stage 36）：三态里「本机没测成」既不是成功也不是失败，
// 压成真假会让前端把本机问题画成上游失败，与判停口径自相矛盾。
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
		httpresp.InternalErrorMessage(c, "Failed to retrieve health check logs", err)
		return
	}

	status := make([]string, 0, len(logs))
	for _, log := range logs {
		status = append(status, log.Status)
	}
	slices.Reverse(status)
	httpresp.Success(c, status)
}
