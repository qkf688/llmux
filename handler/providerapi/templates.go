package providerapi

import (
	"github.com/atopos31/llmio/common"
	"github.com/gin-gonic/gin"
)

// GetProviderTemplates 获取提供商模板。
func GetProviderTemplates(c *gin.Context) {
	common.Success(c, template)
}