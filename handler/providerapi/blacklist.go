package providerapi

import (
	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/repository"
	"github.com/gin-gonic/gin"
)

// GetProviderBlacklist 获取拉黑的供应商ID列表。
func GetProviderBlacklist(c *gin.Context) {
	ctx := c.Request.Context()
	trueVal := true
	list, err := repos().Provider.List(ctx, repository.ProviderFilter{Blacklisted: &trueVal})
	if err != nil {
		common.InternalServerError(c, "Failed to get blacklist: "+err.Error())
		return
	}
	ids := make([]uint, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.ID)
	}
	common.Success(c, map[string]interface{}{"blacklisted_ids": ids})
}

// UpdateProviderBlacklist 更新供应商黑名单（整体替换）。
func UpdateProviderBlacklist(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		ProviderIDs []uint `json:"provider_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	rowsAffected, err := repos().Provider.UpdateBlacklist(ctx, req.ProviderIDs)
	if err != nil {
		common.InternalServerError(c, "Failed to update blacklist: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{"updated": rowsAffected})
}