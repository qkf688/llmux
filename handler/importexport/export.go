package importexport

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// ExportConfig 导出系统配置为 JSON。
func ExportConfig(c *gin.Context) {
	var providerList []models.Provider
	var modelsList []models.Model
	var modelProviders []models.ModelWithProvider
	var settings []models.Setting

	if err := models.DB.Find(&providerList).Error; err != nil {
		httpresp.InternalErrorMessage(c, "读取提供商数据失败", err)
		return
	}

	// 顶层 AuthType 出库即派生（Stage 20）：导出物与详情/列表同口径，config 才是真源。
	for i := range providerList {
		providers.FillDerivedAuthType(&providerList[i])
	}

	if err := models.DB.Find(&modelsList).Error; err != nil {
		httpresp.InternalErrorMessage(c, "读取模型数据失败", err)
		return
	}

	if err := models.DB.Find(&modelProviders).Error; err != nil {
		httpresp.InternalErrorMessage(c, "读取模型-提供商关联数据失败", err)
		return
	}

	if err := models.DB.Find(&settings).Error; err != nil {
		httpresp.InternalErrorMessage(c, "读取系统设置失败", err)
		return
	}

	config := map[string]interface{}{
		"providers":       providerList,
		"models":          modelsList,
		"model_providers": modelProviders,
		"settings":        settings,
		"exported_at":     time.Now(),
		"version":         "1.0",
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("llmux_config_%s.json", timestamp)

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")

	slog.Info("配置导出", "filename", filename)
	c.JSON(200, config)
}
