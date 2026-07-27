package database

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

type tableCountDefinition struct {
	name        string
	displayName string
	model       any
}

type tableStat struct {
	Name               string `json:"name"`
	DisplayName        string `json:"display_name"`
	Count              int64  `json:"count"`
	EstimatedSizeHuman string `json:"estimated_size_human"`
}

func formatByteSize(size int64) string {
	if size <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(size)
	unitIndex := 0

	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}

	if value >= 10 {
		return fmt.Sprintf("%.1f %s", value, units[unitIndex])
	}
	return fmt.Sprintf("%.2f %s", value, units[unitIndex])
}

func estimateTableSize(fileSize int64, tableRows int64, totalRows int64) string {
	if fileSize <= 0 || tableRows <= 0 || totalRows <= 0 {
		return "0 B"
	}
	estimatedSize := (fileSize * tableRows) / totalRows
	if estimatedSize <= 0 {
		estimatedSize = 1
	}
	return formatByteSize(estimatedSize)
}

func getDatabaseTableStats() ([]tableStat, int64) {
	definitions := []tableCountDefinition{
		{name: "providers", displayName: "提供商", model: &models.Provider{}},
		{name: "models", displayName: "模型", model: &models.Model{}},
		{name: "model_with_providers", displayName: "模型关联", model: &models.ModelWithProvider{}},
		{name: "model_template_items", displayName: "模型模板", model: &models.ModelTemplateItem{}},
		{name: "virtual_models", displayName: "虚拟模型", model: &models.VirtualModel{}},
		{name: "virtual_model_mappings", displayName: "虚拟模型映射", model: &models.VirtualModelMapping{}},
		{name: "chat_logs", displayName: "聊天日志", model: &models.ChatLog{}},
		{name: "chat_ios", displayName: "聊天IO", model: &models.ChatIO{}},
		{name: "stats_totals", displayName: "统计总计", model: &models.StatsTotal{}},
		{name: "stats_dailies", displayName: "每日统计", model: &models.StatsDaily{}},
		{name: "stats_hourlies", displayName: "小时统计", model: &models.StatsHourly{}},
		{name: "stats_model_totals", displayName: "模型统计", model: &models.StatsModelTotal{}},
		{name: "stats_real_model_totals", displayName: "真实模型统计", model: &models.StatsRealModelTotal{}},
		{name: "stats_provider_totals", displayName: "供应商统计", model: &models.StatsProviderTotal{}},
		{name: "settings", displayName: "系统设置", model: &models.Setting{}},
		{name: "health_check_logs", displayName: "健康检测日志", model: &models.HealthCheckLog{}},
		{name: "model_sync_logs", displayName: "模型同步日志", model: &models.ModelSyncLog{}},
	}

	stats := make([]tableStat, 0, len(definitions))
	var totalRows int64

	for _, definition := range definitions {
		var count int64
		if err := models.DB.Model(definition.model).Count(&count).Error; err != nil {
			slog.Error("统计表记录数失败", "table", definition.name, "error", err)
			count = 0
		}
		totalRows += count

		stats = append(stats, tableStat{
			Name:               definition.name,
			DisplayName:        definition.displayName,
			Count:              count,
			EstimatedSizeHuman: "0 B",
		})
	}

	return stats, totalRows
}

// GetDatabaseStats 获取数据库统计信息。
func GetDatabaseStats(c *gin.Context) {
	dbPath := models.GetDBPath()

	fileInfo, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		httpresp.NotFound(c, "数据库文件不存在")
		return
	}
	if err != nil {
		slog.Error("无法访问数据库文件", "error", err, "path", dbPath)
		httpresp.InternalServerError(c, "无法访问数据库文件")
		return
	}

	tableStats, totalRows := getDatabaseTableStats()

	providerCount := int64(0)
	modelCount := int64(0)
	modelProviderCount := int64(0)
	for _, stat := range tableStats {
		switch stat.Name {
		case "providers":
			providerCount = stat.Count
		case "models":
			modelCount = stat.Count
		case "model_with_providers":
			modelProviderCount = stat.Count
		}
	}

	var pageCount int64
	if err := models.DB.Raw("PRAGMA page_count").Scan(&pageCount).Error; err != nil {
		slog.Error("获取 page_count 失败", "error", err)
	}

	var pageSize int64
	if err := models.DB.Raw("PRAGMA page_size").Scan(&pageSize).Error; err != nil {
		slog.Error("获取 page_size 失败", "error", err)
	}

	var freePages int64
	if err := models.DB.Raw("PRAGMA freelist_count").Scan(&freePages).Error; err != nil {
		slog.Error("获取 freelist_count 失败", "error", err)
	}

	sqliteVersion := "-"
	if err := models.DB.Raw("SELECT sqlite_version()").Scan(&sqliteVersion).Error; err != nil {
		slog.Error("获取 sqlite 版本失败", "error", err)
	}

	encoding := "-"
	if err := models.DB.Raw("PRAGMA encoding").Scan(&encoding).Error; err != nil {
		slog.Error("获取数据库编码失败", "error", err)
	}

	for i := range tableStats {
		tableStats[i].EstimatedSizeHuman = estimateTableSize(fileInfo.Size(), tableStats[i].Count, totalRows)
	}

	stats := map[string]interface{}{
		"file_path":            dbPath,
		"file_size":            fileInfo.Size(),
		"file_size_human":      formatByteSize(fileInfo.Size()),
		"table_stats":          tableStats,
		"page_count":           pageCount,
		"page_size":            pageSize,
		"free_pages":           freePages,
		"last_vacuum_at":       nil,
		"can_vacuum":           true,
		"db_path":              dbPath,
		"sqlite_version":       sqliteVersion,
		"encoding":             encoding,
		"last_modified":        fileInfo.ModTime(),
		"database_path":        dbPath,
		"database_size":        fileInfo.Size(),
		"provider_count":       providerCount,
		"model_count":          modelCount,
		"model_provider_count": modelProviderCount,
		"timestamp":            time.Now(),
	}

	httpresp.Success(c, stats)
}