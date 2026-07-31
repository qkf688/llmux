package virtualmodels

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service"
)

// GetVirtualModelStats 获取虚拟模型统计信息。
func GetVirtualModelStats(c *gin.Context) {
	id, ok := httpx.ParseUintParamAllowZero(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if _, err := repos().VirtualModel.Get(ctx, id); err != nil {
		httpresp.NotFound(c, "Virtual model not found")
		return
	}

	// 统计逻辑委托 service/virtualmodel
	virtualModelService := service.NewVirtualModelService(models.DB)
	stats, err := virtualModelService.GetVirtualModelStats(ctx, id)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to get stats: "+err.Error())
		return
	}

	httpresp.Success(c, stats)
}
