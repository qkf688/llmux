package autoassoc

import (
	"context"
	"log/slog"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
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

// AutoAssociateModels 一键添加关联：将提供商模型自动关联到模板匹配的模型（并集规则）。
func AutoAssociateModels(c *gin.Context) {
	addedCount, err := service.GetAutoAssocService().Associate(c.Request.Context())
	if err != nil {
		slog.Error("auto-associate failed", "error", err, "added", addedCount)
		httpresp.InternalServerError(c, "failed to auto associate models")
		return
	}
	httpresp.Success(c, map[string]interface{}{
		"added": addedCount,
	})
}

// TriggerAutoAssociate 触发自动关联（全局开关由 service 检查）。
func TriggerAutoAssociate(ctx context.Context) {
	service.GetAutoAssocService().TriggerAssociateIfEnabled(ctx)
}
