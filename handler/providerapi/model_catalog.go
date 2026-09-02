package providerapi

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/service/modelsync"
)

// GetProviderModelCatalog 返回全部供应商的组织级模型目录（分组白名单并集
// + custom_models，只读聚合，不打上游）。前端 models / model-providers /
// providers 页目录数据源，替代 Config.upstream_models 客户端解析（#8-4）。
func GetProviderModelCatalog(c *gin.Context) {
	items, err := modelsync.GetProviderModelCatalogs(c.Request.Context(), repos())
	if err != nil {
		httpresp.InternalServerError(c, "Failed to build model catalog: "+err.Error())
		return
	}
	httpresp.Success(c, items)
}
