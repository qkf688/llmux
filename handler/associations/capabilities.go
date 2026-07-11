package associations

import (
	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

// BatchUpdateModelProvidersCapabilities 批量更新模型提供商关联能力字段。
func BatchUpdateModelProvidersCapabilities(c *gin.Context) {
	var req BatchUpdateModelProvidersCapabilitiesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	if len(req.IDs) == 0 {
		common.BadRequest(c, "No IDs provided")
		return
	}

	if req.ToolCall == nil && req.StructuredOutput == nil && req.Image == nil {
		common.BadRequest(c, "No capability fields provided")
		return
	}

	updates := models.ModelWithProvider{
		ToolCall:         req.ToolCall,
		StructuredOutput: req.StructuredOutput,
		Image:            req.Image,
	}

	result, err := repos().ModelWithProvider.UpdateByIDs(c.Request.Context(), req.IDs, updates)
	if err != nil {
		common.InternalServerError(c, "Failed to update capabilities: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"updated": result,
	})
}