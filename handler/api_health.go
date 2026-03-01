package handler

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RunHealthCheck 手动运行单个模型提供商的健康检测
func RunHealthCheck(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	ctx := c.Request.Context()

	log, err := service.GetHealthChecker().CheckSingle(ctx, uint(id))
	if err != nil {
		common.InternalServerError(c, "Failed to run health check: "+err.Error())
		return
	}

	common.Success(c, log)
}

// RunHealthCheckAll 手动运行所有模型提供商的健康检测
func RunHealthCheckAll(c *gin.Context) {
	batchID := uuid.New().String()

	go func() {
		checker := service.GetHealthChecker()
		ctx := context.Background()

		if err := checker.CheckAllWithBatch(ctx, batchID); err != nil {
			slog.Error("failed to run batch health check", "error", err, "batch_id", batchID)
		}
	}()

	common.Success(c, map[string]string{
		"batch_id": batchID,
		"message":  "Health check started for all model providers",
	})
}

// BatchHealthCheckStatus 批次健康检测状态响应
type BatchHealthCheckStatus struct {
	BatchID    string                  `json:"batch_id"`
	TotalCount int                     `json:"total_count"`
	Success    int                     `json:"success"`
	Failed     int                     `json:"failed"`
	Pending    int                     `json:"pending"`
	Completed  bool                    `json:"completed"`
	Logs       []models.HealthCheckLog `json:"logs"`
}

// GetBatchHealthCheckStatus 查询批次健康检测状态
func GetBatchHealthCheckStatus(c *gin.Context) {
	ctx := c.Request.Context()
	batchID := c.Param("batchId")

	if batchID == "" {
		common.BadRequest(c, "batch_id is required")
		return
	}

	// 获取所有模型提供商数量
	totalCount, err := gorm.G[models.ModelWithProvider](models.DB).Count(ctx, "id")
	if err != nil {
		common.InternalServerError(c, "Failed to count model providers: "+err.Error())
		return
	}

	// 查询该批次的所有日志
	logs, err := gorm.G[models.HealthCheckLog](models.DB).
		Where("batch_id = ?", batchID).
		Order("checked_at DESC").
		Find(ctx)
	if err != nil {
		common.InternalServerError(c, "Failed to fetch health check logs: "+err.Error())
		return
	}

	// 统计结果
	successCount := 0
	failedCount := 0
	for _, log := range logs {
		if log.Status == "success" {
			successCount++
		} else if log.Status == "error" {
			failedCount++
		}
	}

	completedCount := len(logs)
	pendingCount := int(totalCount) - completedCount
	completed := pendingCount == 0

	status := BatchHealthCheckStatus{
		BatchID:    batchID,
		TotalCount: int(totalCount),
		Success:    successCount,
		Failed:     failedCount,
		Pending:    pendingCount,
		Completed:  completed,
		Logs:       logs,
	}

	common.Success(c, status)
}
