package virtualmodels

import (
	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
)

// GetVirtualModelStats 获取虚拟模型统计信息。
func GetVirtualModelStats(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if _, err := repos().VirtualModel.Get(ctx, id); err != nil {
		common.NotFound(c, "Virtual model not found")
		return
	}

	// 统计逻辑委托 service/virtualmodel
	virtualModelService := service.NewVirtualModelService(models.DB)
	stats, err := virtualModelService.GetVirtualModelStats(ctx, id)
	if err != nil {
		common.InternalServerError(c, "Failed to get stats: "+err.Error())
		return
	}

	common.Success(c, stats)
}