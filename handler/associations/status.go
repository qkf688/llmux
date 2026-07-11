package associations

import (
	"errors"
	"strconv"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UpdateModelProviderStatus 切换模型提供商关联启用状态。
func UpdateModelProviderStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		common.BadRequest(c, "Invalid ID format")
		return
	}

	var req ModelProviderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()
	existing, err := repos().ModelWithProvider.Get(ctx, uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.NotFound(c, "Model-provider association not found")
			return
		}
		common.InternalServerError(c, "Failed to retrieve model-provider association: "+err.Error())
		return
	}

	status := req.Status
	updates := models.ModelWithProvider{
		Status: &status,
	}
	if status {
		updates.ConsecutiveFailures = 0
	}

	if err := repos().ModelWithProvider.Update(ctx, uint(id), updates); err != nil {
		common.InternalServerError(c, "Failed to update status: "+err.Error())
		return
	}

	existing.Status = &status
	common.Success(c, existing)
}

// BatchUpdateModelProvidersStatus 批量更新模型提供商关联状态。
func BatchUpdateModelProvidersStatus(c *gin.Context) {
	var req BatchUpdateModelProvidersStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	status := req.Status
	updates := models.ModelWithProvider{
		Status: &status,
	}
	if status {
		updates.ConsecutiveFailures = 0
	}

	result, err := repos().ModelWithProvider.UpdateByIDs(c.Request.Context(), req.IDs, updates)
	if err != nil {
		common.InternalServerError(c, "Failed to update status: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"updated": result,
	})
}