package associations

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/httpx"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
)

// BatchUpdateModelProvidersCapabilities 批量更新模型提供商关联能力字段。
func BatchUpdateModelProvidersCapabilities(c *gin.Context) {
	var req BatchUpdateModelProvidersCapabilitiesRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	if len(req.IDs) == 0 {
		httpresp.BadRequest(c, "No IDs provided")
		return
	}

	if req.ToolCall == nil && req.StructuredOutput == nil && req.Image == nil {
		httpresp.BadRequest(c, "No capability fields provided")
		return
	}

	updates := models.ModelWithProvider{
		ToolCall:         req.ToolCall,
		StructuredOutput: req.StructuredOutput,
		Image:            req.Image,
	}

	result, err := repos().ModelWithProvider.UpdateByIDs(c.Request.Context(), req.IDs, updates)
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to update capabilities", err)
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"updated": result,
	})
}
