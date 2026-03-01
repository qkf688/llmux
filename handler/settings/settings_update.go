package settings

import (
	"context"
	"errors"

	"github.com/atopos31/llmio/common"
	"github.com/gin-gonic/gin"
)

type directClientError struct {
	message string
}

func (e directClientError) Error() string {
	return e.message
}

func newDirectClientError(message string) error {
	return directClientError{message: message}
}

// UpdateSettings 更新设置
func UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	if err := updateStrictAndWeightSettings(ctx, &req); err != nil {
		handleUpdateSettingsError(c, err)
		return
	}
	if err := updatePriorityAndFailureSettings(ctx, &req); err != nil {
		handleUpdateSettingsError(c, err)
		return
	}
	if err := updateLogSettings(ctx, req); err != nil {
		handleUpdateSettingsError(c, err)
		return
	}
	if err := updatePerformanceSettings(ctx, req); err != nil {
		handleUpdateSettingsError(c, err)
		return
	}
	if err := updateAssociationSettings(ctx, req); err != nil {
		handleUpdateSettingsError(c, err)
		return
	}
	if err := updateModelSyncSettings(ctx, &req); err != nil {
		handleUpdateSettingsError(c, err)
		return
	}
	if err := updateTemplateFuzzyMatchSettings(ctx, req); err != nil {
		handleUpdateSettingsError(c, err)
		return
	}
	if err := updateReasoningEffortSettings(ctx, &req); err != nil {
		handleUpdateSettingsError(c, err)
		return
	}

	// 如果设置了保留条数限制，立即执行清理
	if req.LogRetentionCount > 0 {
		go cleanupExcessLogs(req.LogRetentionCount)
	}

	// 返回更新后的设置
	GetSettings(c)
}

func handleUpdateSettingsError(c *gin.Context, err error) {
	var clientErr directClientError
	if errors.As(err, &clientErr) {
		common.InternalServerError(c, clientErr.Error())
		return
	}

	common.InternalServerError(c, "Failed to update settings: "+err.Error())
}

func triggerBatchImportForAutoSave() {
	go batchImportExistingAssociations(context.Background())
}
