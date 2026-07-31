package importexport

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
)

// ExportConfig 导出系统配置为 JSON。
func ExportConfig(c *gin.Context) {
	var providers []models.Provider
	var modelsList []models.Model
	var modelProviders []models.ModelWithProvider
	var settings []models.Setting

	if err := models.DB.Find(&providers).Error; err != nil {
		slog.Error("读取提供商数据失败", "error", err)
		httpresp.InternalServerError(c, "读取提供商数据失败")
		return
	}

	if err := models.DB.Find(&modelsList).Error; err != nil {
		slog.Error("读取模型数据失败", "error", err)
		httpresp.InternalServerError(c, "读取模型数据失败")
		return
	}

	if err := models.DB.Find(&modelProviders).Error; err != nil {
		slog.Error("读取模型-提供商关联数据失败", "error", err)
		httpresp.InternalServerError(c, "读取模型-提供商关联数据失败")
		return
	}

	if err := models.DB.Find(&settings).Error; err != nil {
		slog.Error("读取系统设置失败", "error", err)
		httpresp.InternalServerError(c, "读取系统设置失败")
		return
	}

	config := map[string]interface{}{
		"providers":       providers,
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
