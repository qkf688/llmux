package modelsync

import (
	"strconv"
	"time"

	"github.com/atopos31/llmio/handler/httpx"
	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/repository"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
	ProvidersWithUpdates int        `json:"providers_with_updates"` // 有更新的提供商数量（status = "success"）
	ProvidersUnchanged   int        `json:"providers_unchanged"`    // 没变化的提供商数量（status = "unchanged"）
	ProvidersWithErrors  int        `json:"providers_with_errors"`  // 报错的提供商数量（status = "error"）
	ProvidersNeverSynced int        `json:"providers_never_synced"` // 从未同步的提供商数量
}

// GetModelSyncStats 获取模型同步统计信息
func GetModelSyncStats(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取所有提供商数量（启用模型端点的）
	var totalProviders int64
	enabledProviderIDs := repository.WhereModelEndpointEnabled(
		models.DB.WithContext(ctx).Model(&models.Provider{}).Select("id"),
	)

	if err := repository.WhereModelEndpointEnabled(
		models.DB.WithContext(ctx).Model(&models.Provider{}),
	).Count(&totalProviders).Error; err != nil {
		httpresp.InternalServerError(c, "Failed to count providers: "+err.Error())
		return
	}

	// 获取同步设置
	syncEnabled := false
	syncInterval := 12 // 默认12小时

	setting, err := gorm.G[models.Setting](models.DB).Where("key = ?", models.SettingKeyModelSyncEnabled).First(ctx)
	if err == nil && setting.Value == "true" {
		syncEnabled = true
	}

	setting, err = gorm.G[models.Setting](models.DB).Where("key = ?", models.SettingKeyModelSyncInterval).First(ctx)
	if err == nil {
		if val, err := strconv.Atoi(setting.Value); err == nil {
			syncInterval = val
		}
	}

	// 获取最近一次同步记录
	var lastLog models.ModelSyncLog
	err = models.DB.WithContext(ctx).
		Where("provider_id IN (?)", enabledProviderIDs).
		Order("synced_at DESC").
		First(&lastLog).Error

	// 处理零值时间：如果没有同步记录或时间为零值，返回 nil
	var lastSyncAt *time.Time
	if err == nil && !lastLog.SyncedAt.IsZero() {
		lastSyncAt = &lastLog.SyncedAt
	}

	// 计算下次同步时间
	var nextSyncAt *time.Time
	if syncEnabled && lastSyncAt != nil {
		next := lastSyncAt.Add(time.Duration(syncInterval) * time.Hour)
		nextSyncAt = &next
	}

	// 统计各状态的提供商数量
	// 获取所有提供商的最近一次同步状态
	var logs []models.ModelSyncLog
	if err := models.DB.WithContext(ctx).
		Where("provider_id IN (?)", enabledProviderIDs).
		Order("synced_at DESC").
		Find(&logs).Error; err != nil {
		httpresp.InternalServerError(c, "Failed to get sync logs: "+err.Error())
		return
	}

	// 按提供商分组，获取每个提供商最近一次同步的状态
	providerStats := make(map[uint]models.ModelSyncLog)
	for _, log := range logs {
		if _, exists := providerStats[log.ProviderID]; !exists {
			providerStats[log.ProviderID] = log
		}
	}

	// 使用 Status 字段进行统计
	providersWithUpdates := 0 // status = "success"
	providersUnchanged := 0   // status = "unchanged"
	providersWithErrors := 0  // status = "error"

	for _, log := range providerStats {
		// 使用 Status 字段（新逻辑）
		if log.Status != "" {
			switch log.Status {
			case "success":
				providersWithUpdates++
			case "unchanged":
				providersUnchanged++
			case "error":
				providersWithErrors++
			}
		} else {
			// 向后兼容：旧数据没有 Status 字段，根据 AddedCount 和 RemovedCount 判断
			if log.AddedCount > 0 || log.RemovedCount > 0 {
				providersWithUpdates++
			} else {
				providersUnchanged++
			}
		}
	}

	// 计算从未同步的提供商数量
	providersNeverSynced := int(totalProviders) - len(providerStats)
	if providersNeverSynced < 0 {
		providersNeverSynced = 0
	}

	response := ModelSyncStatsResponse{
		LastSyncAt:           lastSyncAt,
		NextSyncAt:           nextSyncAt,
		SyncEnabled:          syncEnabled,
		SyncInterval:         syncInterval,
		TotalProviders:       int(totalProviders),
		ProvidersWithUpdates: providersWithUpdates,
		ProvidersUnchanged:   providersUnchanged,
		ProvidersWithErrors:  providersWithErrors,
		ProvidersNeverSynced: providersNeverSynced,
	}

	httpresp.Success(c, response)
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
