package handler

import (
	"encoding/json"
	"fmt"
	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	"os"
	"time"
)

// ExportConfig 导出系统配置为 JSON
func ExportConfig(c *gin.Context) {
	// 获取所有配置数据
	var providers []models.Provider
	var modelsList []models.Model
	var modelProviders []models.ModelWithProvider
	var settings []models.Setting

	// 读取提供商数据
	if err := models.DB.Find(&providers).Error; err != nil {
		slog.Error("读取提供商数据失败", "error", err)
		common.InternalServerError(c, "读取提供商数据失败")
		return
	}

	// 读取模型数据
	if err := models.DB.Find(&modelsList).Error; err != nil {
		slog.Error("读取模型数据失败", "error", err)
		common.InternalServerError(c, "读取模型数据失败")
		return
	}

	// 读取模型-提供商关联数据
	if err := models.DB.Find(&modelProviders).Error; err != nil {
		slog.Error("读取模型-提供商关联数据失败", "error", err)
		common.InternalServerError(c, "读取模型-提供商关联数据失败")
		return
	}

	// 读取系统设置
	if err := models.DB.Find(&settings).Error; err != nil {
		slog.Error("读取系统设置失败", "error", err)
		common.InternalServerError(c, "读取系统设置失败")
		return
	}

	// 构建配置数据
	config := map[string]interface{}{
		"providers":       providers,
		"models":          modelsList,
		"model_providers": modelProviders,
		"settings":        settings,
		"exported_at":     time.Now(),
		"version":         "1.0",
	}

	// 生成带时间戳的文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("llmio_config_%s.json", timestamp)

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")

	// 记录导出操作
	slog.Info("配置导出", "filename", filename)

	// 发送 JSON 数据
	c.JSON(200, config)
}

// ImportConfig 导入系统配置
func ImportConfig(c *gin.Context) {
	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		slog.Error("读取请求体失败", "error", err)
		common.BadRequest(c, "读取请求体失败")
		return
	}
	defer c.Request.Body.Close()

	// 解析 JSON 数据
	var config map[string]interface{}
	if err := json.Unmarshal(body, &config); err != nil {
		slog.Error("解析配置数据失败", "error", err)
		common.BadRequest(c, "解析配置数据失败")
		return
	}

	// 开始事务
	tx := models.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 导入提供商数据
	if providersData, ok := config["providers"].([]interface{}); ok {
		// 清空现有提供商数据
		if err := tx.Exec("DELETE FROM providers").Error; err != nil {
			tx.Rollback()
			slog.Error("清空提供商数据失败", "error", err)
			common.InternalServerError(c, "清空提供商数据失败")
			return
		}

		// 导入新提供商数据
		for _, pData := range providersData {
			if _, ok := pData.(map[string]interface{}); ok {
				var provider models.Provider
				if err := tx.Create(&provider).Error; err != nil {
					tx.Rollback()
					slog.Error("导入提供商数据失败", "error", err)
					common.InternalServerError(c, "导入提供商数据失败")
					return
				}
			}
		}
	}

	// 导入模型数据
	if modelsData, ok := config["models"].([]interface{}); ok {
		// 清空现有模型数据
		if err := tx.Exec("DELETE FROM models").Error; err != nil {
			tx.Rollback()
			slog.Error("清空模型数据失败", "error", err)
			common.InternalServerError(c, "清空模型数据失败")
			return
		}

		// 导入新模型数据
		for _, mData := range modelsData {
			if _, ok := mData.(map[string]interface{}); ok {
				var model models.Model
				if err := tx.Create(&model).Error; err != nil {
					tx.Rollback()
					slog.Error("导入模型数据失败", "error", err)
					common.InternalServerError(c, "导入模型数据失败")
					return
				}
			}
		}
	}

	// 导入模型-提供商关联数据
	if modelProvidersData, ok := config["model_providers"].([]interface{}); ok {
		// 清空现有关联数据
		if err := tx.Exec("DELETE FROM model_with_providers").Error; err != nil {
			tx.Rollback()
			slog.Error("清空模型-提供商关联数据失败", "error", err)
			common.InternalServerError(c, "清空模型-提供商关联数据失败")
			return
		}

		// 导入新关联数据
		for _, mpData := range modelProvidersData {
			if _, ok := mpData.(map[string]interface{}); ok {
				var modelProvider models.ModelWithProvider
				if err := tx.Create(&modelProvider).Error; err != nil {
					tx.Rollback()
					slog.Error("导入模型-提供商关联数据失败", "error", err)
					common.InternalServerError(c, "导入模型-提供商关联数据失败")
					return
				}
			}
		}
	}

	// 导入系统设置
	if _, ok := config["settings"].([]interface{}); ok {
		// 清空现有设置
		if err := tx.Exec("DELETE FROM settings").Error; err != nil {
			tx.Rollback()
			slog.Error("清空系统设置失败", "error", err)
			common.InternalServerError(c, "清空系统设置失败")
			return
		}

		// 导入新设置
		// 这里需要根据实际的设置数据结构进行处理
		// 由于设置是键值对形式，需要单独处理每个设置项
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		slog.Error("提交事务失败", "error", err)
		common.InternalServerError(c, "提交事务失败")
		return
	}

	// 记录导入操作
	slog.Info("配置导入成功")

	common.Success(c, gin.H{"message": "配置导入成功"})
}

// ExportDatabase 导出完整数据库文件
func ExportDatabase(c *gin.Context) {
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

	// 检查文件大小（可选警告）
	if fileInfo.Size() > 100*1024*1024 { // 100MB
		slog.Warn("导出的数据库文件较大", "size", fileInfo.Size(), "path", dbPath)
	}

	// 生成带时间戳的文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("llmio_backup_%s.db", timestamp)

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// 记录导出操作
	slog.Info("数据库导出", "filename", filename, "size", fileInfo.Size())

	// 直接发送文件
	c.File(dbPath)
}
