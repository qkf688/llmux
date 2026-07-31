package settings

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
	"gorm.io/gorm"
)

// settingStore 返回默认 settings.Store。
// 必须函数式懒取：包级变量初始化早于 main 里的 models.Init，
// 提前捕获 models.DB 会永久持有 nil。
func settingStore() *settings.Store {
	return settings.Default()
}

// SchemaForSettingKey 是 models.SettingSchemaForKey 的本地别名，便于测试替换。
var SchemaForSettingKey = models.SettingSchemaForKey

// GetSettings 获取所有设置
func GetSettings(c *gin.Context) {
	ctx := c.Request.Context()
	settingsList, err := gorm.G[models.Setting](models.DB).Find(ctx)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to get settings: "+err.Error())
		return
	}

	response, err := buildSettingsResponse(settingsList)
	if err != nil {
		httpresp.InternalServerError(c, "Failed to build settings response: "+err.Error())
		return
	}

	httpresp.Success(c, response)
}

// buildSettingsResponse 根据 schema 默认值与数据库值构建 SettingsResponse。
func buildSettingsResponse(settingsList []models.Setting) (SettingsResponse, error) {
	var response SettingsResponse
	if err := models.ApplySettingDefaults(&response); err != nil {
		return response, err
	}

	for _, setting := range settingsList {
		schema, ok := SchemaForSettingKey(setting.Key)
		if !ok {
			continue
		}
		parsed, err := settings.FromString(setting.Value, schema.Type)
		if err != nil {
			slog.Warn("failed to parse setting value", "key", setting.Key, "error", err)
			continue
		}
		if err := models.SetSettingField(&response, schema, parsed); err != nil {
			slog.Warn("failed to set setting field", "key", setting.Key, "error", err)
			continue
		}
	}

	return response, nil
}
