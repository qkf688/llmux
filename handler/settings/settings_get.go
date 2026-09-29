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
		httpresp.InternalErrorMessage(c, "Failed to get settings", err)
		return
	}

	response, err := buildSettingsResponse(settingsList)
	if err != nil {
		httpresp.InternalErrorMessage(c, "Failed to build settings response", err)
		return
	}

	httpresp.Success(c, response)
}

// buildSettingsResponse 根据 schema 默认值与数据库值构建 SettingsResponse。
//
// 对「schema 有、通用响应结构里没有位置」的键**静默跳过**（Stage 36）：健康检测系设置
// 有自己的独立域（handler/settings/health.go），在通用设置页没有位置是已知归属边界，
// 不是组装失败——逐条 WARN 会让每次打开设置页刷 7 条噪声、淹掉真警告，也把人往
// 「组装出错」的方向带。静默跳过的风险（真漏字段被吞）由字段契约门兜住：
// TestGenericSettingsResponseFieldContract 要求每个非独立域的设置项都在响应里有位置。
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
		if !models.SettingFieldPresent(&response, schema) {
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
