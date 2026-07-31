package providerapi

import (
	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/providers"
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
