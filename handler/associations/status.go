package associations

import (
	"errors"
	"strconv"

	"github.com/atopos31/llmio/httpresp"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// statusUpdateFields 构造状态切换要写入的列。
// 启用时一并把连续失败计数清零，否则关联恢复后再失败一次就会立刻达阈值被重新禁用。
// 必须用列名 map：结构体 Updates 会把 consecutive_failures 的 0 当零值跳过。
func statusUpdateFields(enabled bool) map[string]any {
	fields := map[string]any{"status": enabled}
	if enabled {
		fields["consecutive_failures"] = 0
	}
	return fields
}

// UpdateModelProviderStatus 切换模型提供商关联启用状态。
func UpdateModelProviderStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}

	var req ModelProviderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()
	existing, err := repos().ModelWithProvider.Get(ctx, uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Model-provider association not found")
			return
		}
		httpresp.InternalServerError(c, "Failed to retrieve model-provider association: "+err.Error())
		return
	}

	status := req.Status
	if _, err := repos().ModelWithProvider.UpdateFields(ctx, uint(id), statusUpdateFields(status)); err != nil {
		httpresp.InternalServerError(c, "Failed to update status: "+err.Error())
		return
	}

	existing.Status = &status
	httpresp.Success(c, existing)
}

// BatchUpdateModelProvidersStatus 批量更新模型提供商关联状态。
func BatchUpdateModelProvidersStatus(c *gin.Context) {
	var req BatchUpdateModelProvidersStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}

	result, err := repos().ModelWithProvider.UpdateFieldsByIDs(c.Request.Context(), req.IDs, statusUpdateFields(req.Status))
	if err != nil {
		httpresp.InternalServerError(c, "Failed to update status: "+err.Error())
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"updated": result,
	})
}
