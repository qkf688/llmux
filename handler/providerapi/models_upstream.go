package providerapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/providers"
)

// GetProviderModels 拉取供应商上游模型列表。
func GetProviderModels(c *gin.Context) {
	id := c.Param("id")
	source := c.Query("source")

	providerID, err := parseUintParam(id)
	if err != nil {
		httpresp.BadRequest(c, "Invalid ID format")
		return
	}

	provider, err := repos().Provider.Get(c.Request.Context(), providerID)
	if err != nil {
		httpresp.InternalServerError(c, err.Error())
		return
	}

	config := provider.Config
	if source == "upstream" {
		if cleanedConfig, err := dropCustomModels(config); err == nil {
			config = cleanedConfig
		} else {
			slog.Warn("failed to strip custom models for upstream refresh", "provider_id", id, "error", err)
		}
	}

	chatModel, err := providers.New(provider.Type, config, provider.Proxy)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to get models: "+err.Error())
		return
	}
	modelList, err := chatModel.Models(c.Request.Context())
	if err != nil {
		httpresp.NotFound(c, "Failed to get models: "+err.Error())
		return
	}
	// 确保返回的是数组而不是 nil，避免前端白屏
	if modelList == nil {
		modelList = []providers.Model{}
	}
	httpresp.Success(c, modelList)
}
