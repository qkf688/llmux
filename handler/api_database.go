package handler

import (
	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"log/slog"
	"os"
	"time"
)

// GetDatabaseStats 获取数据库统计信息
func GetDatabaseStats(c *gin.Context) {
	// 获取数据库文件路径
	dbPath := models.GetDBPath()

	// 检查文件是否存在并获取文件信息
	fileInfo, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		common.NotFound(c, "数据库文件不存在")
		return
	}
	if err != nil {
		slog.Error("无法访问数据库文件", "error", err, "path", dbPath)
		common.InternalServerError(c, "无法访问数据库文件")
		return
	}

	// 统计数据库表信息
	var providerCount, modelCount, modelProviderCount int64

	// 统计提供商数量
	if err := models.DB.Model(&models.Provider{}).Count(&providerCount).Error; err != nil {
		slog.Error("统计提供商数量失败", "error", err)
	}

	// 统计模型数量
	if err := models.DB.Model(&models.Model{}).Count(&modelCount).Error; err != nil {
		slog.Error("统计模型数量失败", "error", err)
	}

	// 统计模型-提供商关联数量
	if err := models.DB.Model(&models.ModelWithProvider{}).Count(&modelProviderCount).Error; err != nil {
		slog.Error("统计模型-提供商关联数量失败", "error", err)
	}

	// 构建响应
	stats := map[string]interface{}{
		"database_path":            dbPath,
		"database_size":            fileInfo.Size(),
		"last_modified":           fileInfo.ModTime(),
		"provider_count":          providerCount,
		"model_count":             modelCount,
		"model_provider_count":    modelProviderCount,
		"timestamp":              time.Now(),
	}

	common.Success(c, stats)
}

// VacuumDatabase 执行数据库真空操作
func VacuumDatabase(c *gin.Context) {
	if err := models.DB.Exec("VACUUM").Error; err != nil {
		slog.Error("执行数据库真空操作失败", "error", err)
		common.InternalServerError(c, "执行数据库真空操作失败: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"message": "数据库真空操作执行成功",
		"timestamp": time.Now(),
	})
}
