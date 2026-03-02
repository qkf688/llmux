package settings

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type modelScopeRequest struct {
	ModelID *uint `json:"model_id"`
}

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

	var req modelScopeRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	const defaultWeight = 5
	db := models.DB.WithContext(ctx).Model(&models.ModelWithProvider{})
	if req.ModelID != nil {
		db = db.Where("model_id = ?", *req.ModelID)
	} else {
		db = db.Session(&gorm.Session{AllowGlobalUpdate: true})
	}

	// 重置所有模型权重为默认值
	// 仅更新非默认值，避免对已是默认值的记录重复更新导致“updated”计数误导
	result := db.Where("weight <> ?", defaultWeight).Update("weight", defaultWeight)
	if result.Error != nil {
		slog.Error("重置模型权重失败", "error", result.Error)
		common.InternalServerError(c, "重置模型权重失败: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"message":        "模型权重已重置为默认值",
		"timestamp":      time.Now(),
		"updated":        result.RowsAffected,
		"default_weight": defaultWeight,
	})
}

// ResetModelPriorities 重置所有模型优先级
func ResetModelPriorities(c *gin.Context) {
	ctx := c.Request.Context()

	var req modelScopeRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	// 获取默认优先级值
	defaultPriority := GetAutoPriorityDecayDefault(ctx)

	db := models.DB.WithContext(ctx).Model(&models.ModelWithProvider{})
	if req.ModelID != nil {
		db = db.Where("model_id = ?", *req.ModelID)
	} else {
		db = db.Session(&gorm.Session{AllowGlobalUpdate: true})
	}

	// 重置所有模型优先级为默认值
	// 仅更新非默认值，避免对已是默认值的记录重复更新导致“updated”计数误导
	result := db.Where("priority <> ?", defaultPriority).Update("priority", defaultPriority)
	if result.Error != nil {
		slog.Error("重置模型优先级失败", "error", result.Error)
		common.InternalServerError(c, "重置模型优先级失败: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"message":          "模型优先级已重置为默认值",
		"timestamp":        time.Now(),
		"updated":          result.RowsAffected,
		"default_priority": defaultPriority,
	})
}

// EnableAllAssociations 启用所有模型关联
func EnableAllAssociations(c *gin.Context) {
	ctx := c.Request.Context()

	var req modelScopeRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	db := models.DB.WithContext(ctx).Model(&models.ModelWithProvider{})
	if req.ModelID != nil {
		db = db.Where("model_id = ?", *req.ModelID)
	} else {
		db = db.Session(&gorm.Session{AllowGlobalUpdate: true})
	}

	// 启用所有模型关联
	// 仅对当前为停用（status=false）的关联生效，避免对已启用记录重复更新导致“updated”计数误导
	result := db.Where("status = ?", false).Update("status", true)
	if result.Error != nil {
		slog.Error("启用所有模型关联失败", "error", result.Error)
		common.InternalServerError(c, "启用所有模型关联失败: "+result.Error.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"message":   "所有模型关联已启用",
		"timestamp": time.Now(),
		"updated":   result.RowsAffected,
	})
}
