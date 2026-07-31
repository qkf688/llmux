package autoassoc

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/service"
)

// PreviewAutoAssociate 预览将要添加的关联。
func PreviewAutoAssociate(c *gin.Context) {
	previews, err := service.GetAutoAssocService().PreviewAssociate(c.Request.Context())
	if err != nil {
		slog.Error("auto-associate preview failed", "error", err)
		httpresp.InternalServerError(c, "failed to preview auto associations")
		return
	}
	httpresp.Success(c, previews)
}

// AutoAssociateModels 手动批量关联（绕过 Model.AutoAssociate 门控）。
// 部分失败仍返回 200 + 计数，前端可展示部分成功结果。
func AutoAssociateModels(c *gin.Context) {
	result, err := service.GetAutoAssocService().AssociateAll(c.Request.Context())
	if err != nil {
		slog.Error("auto-associate failed", "error", err)
		httpresp.InternalServerError(c, "failed to auto associate models")
		return
	}
	httpresp.Success(c, map[string]interface{}{
		"added":  result.Success,
		"failed": result.Failed,
	})
}

// TriggerAutoAssociate 触发自动关联（全局开关由 service 检查）。
func TriggerAutoAssociate(ctx context.Context) {
	service.GetAutoAssocService().TriggerAssociateIfEnabled(ctx)
}
