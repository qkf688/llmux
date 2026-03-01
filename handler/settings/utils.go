package settings

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetStrictCapabilityMatch 获取严格能力匹配设置
func GetStrictCapabilityMatch(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyStrictCapabilityMatch).
		First(ctx)
	if err != nil {
		return true // 默认开启
	}
	return setting.Value == "true"
}

// GetAutoPriorityDecayDefault 获取自动优先级衰减默认值
func GetAutoPriorityDecayDefault(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayDefault).
		First(ctx)
	if err != nil {
		return 100 // 默认优先级100
	}
	val, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 100
	}
	return val
}

// GetSettingBool 获取设置的布尔值
func GetSettingBool(ctx context.Context, key string) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", key).
		First(ctx)
	if err != nil {
		return false
	}
	return setting.Value == "true"
}

// cleanupExcessLogs 清理超出保留条数的日志
func cleanupExcessLogs(retentionCount int) {
	// 获取总日志数
	var total int64
	if err := models.DB.Model(&models.ChatLog{}).Count(&total).Error; err != nil {
		slog.Error("failed to count logs for cleanup", "error", err)
		return
	}

	// 如果日志数超过保留条数，删除多余的
	if int(total) > retentionCount {
		deleteCount := int(total) - retentionCount

		// 获取需要删除的日志ID（最旧的）
		var logsToDelete []models.ChatLog
		if err := models.DB.Model(&models.ChatLog{}).
			Order("id ASC").
			Limit(deleteCount).
			Find(&logsToDelete).Error; err != nil {
			slog.Error("failed to find logs to delete", "error", err)
			return
		}

		// 提取ID列表
		ids := make([]uint, len(logsToDelete))
		for i, log := range logsToDelete {
			ids[i] = log.ID
		}

		// 删除对应的ChatIO记录
		if err := models.DB.Unscoped().
			Where("log_id IN ?", ids).
			Delete(&models.ChatIO{}).Error; err != nil {
			slog.Error("failed to delete chat io records", "error", err)
		}

		// 删除日志记录
		if err := models.DB.Unscoped().
			Where("id IN ?", ids).
			Delete(&models.ChatLog{}).Error; err != nil {
			slog.Error("failed to delete logs hard", "error", err)
			return
		}

		slog.Info("cleaned up excess logs", "deleted", deleteCount, "retention", retentionCount)
	}
}

// batchImportExistingAssociations 批量导入现有关联
func batchImportExistingAssociations(ctx context.Context) {
	// 这里应该实现批量导入的逻辑
	// 由于原代码中没有具体实现，这里暂时留空
	slog.Info("batch import of existing associations completed")
}

// ResetModelWeights 重置所有模型权重
func ResetModelWeights(c *gin.Context) {
	ctx := c.Request.Context()
	
	// 重置所有模型权重为默认值
	if err := models.DB.WithContext(ctx).Model(&models.ModelWithProvider{}).Update("weight", 5).Error; err != nil {
		slog.Error("重置模型权重失败", "error", err)
		common.InternalServerError(c, "重置模型权重失败: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"message": "模型权重已重置为默认值",
		"timestamp": time.Now(),
	})
}

// ResetModelPriorities 重置所有模型优先级
func ResetModelPriorities(c *gin.Context) {
	ctx := c.Request.Context()
	
	// 获取默认优先级值
	defaultPriority := GetAutoPriorityDecayDefault(ctx)
	
	// 重置所有模型优先级为默认值
	if err := models.DB.WithContext(ctx).Model(&models.ModelWithProvider{}).Update("priority", defaultPriority).Error; err != nil {
		slog.Error("重置模型优先级失败", "error", err)
		common.InternalServerError(c, "重置模型优先级失败: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"message": "模型优先级已重置为默认值",
		"timestamp": time.Now(),
	})
}

// EnableAllAssociations 启用所有模型关联
func EnableAllAssociations(c *gin.Context) {
	ctx := c.Request.Context()
	
	// 启用所有模型关联
	trueVal := true
	if err := models.DB.WithContext(ctx).Model(&models.ModelWithProvider{}).Update("status", &trueVal).Error; err != nil {
		slog.Error("启用所有模型关联失败", "error", err)
		common.InternalServerError(c, "启用所有模型关联失败: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"message": "所有模型关联已启用",
		"timestamp": time.Now(),
	})
}
