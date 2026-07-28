package autoassoc

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
)

// PreviewCleanInvalid 预览将要删除的关联。
func PreviewCleanInvalid(c *gin.Context) {
	previews, err := service.GetAutoAssocService().PreviewClean(c.Request.Context())
	if err != nil {
		slog.Error("auto-clean preview failed", "error", err)
		httpresp.InternalServerError(c, "failed to preview invalid associations")
		return
	}
	httpresp.Success(c, previews)
}

// CleanInvalidAssociations 清除无效关联：删除提供商不存在或 provider_model 不在列表的关联。
// 部分失败仍返回 200 + 计数。
func CleanInvalidAssociations(c *gin.Context) {
	result, err := service.GetAutoAssocService().CleanInvalid(c.Request.Context())
	if err != nil {
		slog.Error("auto-clean failed", "error", err)
		httpresp.InternalServerError(c, "failed to clean invalid associations")
		return
	}
	httpresp.Success(c, map[string]interface{}{
		"removed": result.Success,
		"failed":  result.Failed,
	})
}

// TriggerAutoClean 触发自动清理（全局开关由 service 检查）。
func TriggerAutoClean(ctx context.Context) {
	service.GetAutoAssocService().TriggerCleanIfEnabled(ctx)
}
