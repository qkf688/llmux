package providerapi

import (
	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/providers"
	"github.com/gin-gonic/gin"
)

// GetProviderTemplates 获取提供商模板（来自 providers 元数据注册表）。
func GetProviderTemplates(c *gin.Context) {
	all := providers.AllMetadata()
	templates := make([]ProviderTemplate, 0, len(all))
	for _, m := range all {
		templates = append(templates, ProviderTemplate{
			Type:     m.Type,
			Template: m.ConfigTemplate,
		})
	}
	httpresp.Success(c, templates)
}