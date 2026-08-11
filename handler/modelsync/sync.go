package modelsync

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service"
)

// SyncProviderModels 同步单个提供商的上游模型
func SyncProviderModels(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}

	syncService := service.NewModelSyncService(models.DB, nil)
	syncLog, err := syncService.SyncProviderModels(c.Request.Context(), id)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to sync models: "+err.Error())
		return
	}

	if syncLog == nil {
		httpresp.Success(c, map[string]interface{}{
			"message": "No changes detected",
		})
		return
	}

	httpresp.Success(c, syncLog)
}

// SyncAllProviders 同步所有启用模型端点的提供商
func SyncAllProviders(c *gin.Context) {
	ctx := c.Request.Context()
	syncService := service.NewModelSyncService(models.DB, nil)

	logs, err := syncService.SyncAllProviders(ctx)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to sync models: "+err.Error())
		return
	}

	if len(logs) == 0 {
		httpresp.Success(c, map[string]interface{}{
			"message": "No changes detected",
		})
		return
	}

	addedTotal := 0
	removedTotal := 0
	for _, log := range logs {
		addedTotal += log.AddedCount
		removedTotal += log.RemovedCount
	}

	httpresp.Success(c, map[string]interface{}{
		"message":          "Sync completed",
		"logs":             logs,
		"synced_providers": len(logs),
		"added_total":      addedTotal,
		"removed_total":    removedTotal,
	})
}

// ModelSyncStatsResponse 模型同步统计响应结构
type ModelSyncStatsResponse struct {
	LastSyncAt           *time.Time `json:"last_sync_at"`           // 上次同步时间
	NextSyncAt           *time.Time `json:"next_sync_at"`           // 下次自动同步时间
	SyncEnabled          bool       `json:"sync_enabled"`           // 是否启用自动同步
	SyncInterval         int        `json:"sync_interval"`          // 同步间隔（小时）
	TotalProviders       int        `json:"total_providers"`        // 总提供商数量
	ProvidersWithUpdates int        `json:"providers_with_updates"` // 最新一次同步为 success 的提供商数量
	ProvidersUnchanged   int        `json:"providers_unchanged"`    // 最新一次同步为 unchanged 的提供商数量
	ProvidersWithErrors  int        `json:"providers_with_errors"`  // 最新一次同步为 error 的提供商数量
	ProvidersNeverSynced int        `json:"providers_never_synced"` // 从未同步的提供商数量
}

// GetModelSyncStats 获取模型同步统计信息
func GetModelSyncStats(c *gin.Context) {
	syncService := service.NewModelSyncService(models.DB, nil)
	stats, err := syncService.GetSyncStats(c.Request.Context())
	if err != nil {
		httpresp.InternalServerError(c, "Failed to get sync stats: "+err.Error())
		return
	}

	httpresp.Success(c, ModelSyncStatsResponse{
		LastSyncAt:           stats.LastSyncAt,
		NextSyncAt:           stats.NextSyncAt,
		SyncEnabled:          stats.SyncEnabled,
		SyncInterval:         stats.SyncInterval,
		TotalProviders:       int(stats.TotalProviders),
		ProvidersWithUpdates: stats.ProvidersWithUpdates,
		ProvidersUnchanged:   stats.ProvidersUnchanged,
		ProvidersWithErrors:  stats.ProvidersWithErrors,
		ProvidersNeverSynced: stats.ProvidersNeverSynced,
	})
}

// GetRecentAddedModels 获取最近新增的模型
func GetRecentAddedModels(c *gin.Context) {
	ctx := c.Request.Context()

	syncService := service.NewModelSyncService(models.DB, nil)
	result, err := syncService.GetRecentAddedModels(ctx)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to get recent added models: "+err.Error())
		return
	}

	httpresp.Success(c, result)
}
