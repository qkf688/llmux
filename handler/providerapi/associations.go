package providerapi

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"gorm.io/gorm"
)

// ClearProviderAssociations 清除指定提供商的所有模型关联。
func ClearProviderAssociations(c *gin.Context) {
	// 错误文案与现网一致：Invalid provider ID format（非通用 Invalid ID format）
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		httpresp.BadRequest(c, "Invalid provider ID format")
		return
	}

	ctx := c.Request.Context()

	provider, err := repos().Provider.Get(ctx, uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpresp.NotFound(c, "Provider not found")
		} else {
			httpresp.InternalServerError(c, "Failed to get provider: "+err.Error())
		}
		return
	}

	deleted, err := repos().ModelWithProvider.DeleteByProviderID(ctx, uint(id))
	if err != nil {
		httpresp.InternalServerError(c, "Failed to clear associations: "+err.Error())
		return
	}

	httpresp.Success(c, map[string]interface{}{
		"provider_id":   id,
		"provider_name": provider.Name,
		"deleted":       deleted,
	})
}
