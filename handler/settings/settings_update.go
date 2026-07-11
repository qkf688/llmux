package settings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/atopos31/llmio/httpresp"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

type directClientError struct {
	message string
}

func (e directClientError) Error() string {
	return e.message
}

func newDirectClientError(message string) error {
	return directClientError{message: message}
}

// UpdateSettings 更新设置
func UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	if err := updateSettingsFromRequest(ctx, &req); err != nil {
		handleUpdateSettingsError(c, err)
		return
	}

	// 如果设置了保留条数限制，立即执行清理
	if req.LogRetentionCount > 0 {
		go cleanupExcessLogs(req.LogRetentionCount)
	}

	// 返回更新后的设置
	GetSettings(c)
}

func handleUpdateSettingsError(c *gin.Context, err error) {
	var clientErr directClientError
	if errors.As(err, &clientErr) {
		httpresp.InternalServerError(c, clientErr.Error())
		return
	}

	httpresp.InternalServerError(c, "Failed to update settings: "+err.Error())
}

func triggerBatchImportForAutoSave() {
	go batchImportExistingAssociations(context.Background())
}

// updateSettingsFromRequest 根据 SettingSchema 反射遍历 UpdateSettingsRequest 字段并写入数据库。
// 新增设置项只需在 models.SettingSchemas 中声明，无需再修改此处或 section updater。
func updateSettingsFromRequest(ctx context.Context, req *UpdateSettingsRequest) error {
	normalizeUpdateSettingsRequest(req)

	v := reflect.ValueOf(req).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		schema, ok := models.SettingSchemaForField(field.Name)
		if !ok {
			continue
		}
		value := v.Field(i).Interface()
		if err := validateSettingValue(schema, value); err != nil {
			return newDirectClientError(err.Error())
		}
		if err := settingStore.SetTyped(ctx, schema.Key, value); err != nil {
			return err
		}
	}

	// 副作用：开启 auto_save_template_on_associate 时触发批量导入
	oldAutoSave := settingStore.GetBool(ctx, models.SettingKeyAutoSaveTemplateOnAssociate)
	if !oldAutoSave && req.AutoSaveTemplateOnAssociate {
		triggerBatchImportForAutoSave()
		slog.Info("triggered batch import of existing associations")
	}

	return nil
}

// normalizeUpdateSettingsRequest 保留原有 section updater 中的交叉字段归一化逻辑。
func normalizeUpdateSettingsRequest(req *UpdateSettingsRequest) {
	if req.AutoSuccessIncrease {
		if req.AutoWeightIncreaseStep < 1 {
			req.AutoWeightIncreaseStep = 1
		}
		if req.AutoWeightIncreaseMax < 1 {
			req.AutoWeightIncreaseMax = 100
		}
	}

	if req.AutoPriorityIncreaseStep < 1 {
		req.AutoPriorityIncreaseStep = 1
	}
	if req.AutoPriorityIncreaseMax < 0 {
		req.AutoPriorityIncreaseMax = 100
	}

	if req.ConsecutiveFailureThreshold < 1 {
		req.ConsecutiveFailureThreshold = 3
	}

	if req.ModelSyncInterval < 1 {
		req.ModelSyncInterval = 12
	}
	if req.ModelSyncLogRetentionCount < 0 {
		req.ModelSyncLogRetentionCount = 100
	}
	if req.ModelSyncLogRetentionDays < 0 {
		req.ModelSyncLogRetentionDays = 7
	}

	if req.ReasoningEffortDefaultValue != "low" &&
		req.ReasoningEffortDefaultValue != "medium" &&
		req.ReasoningEffortDefaultValue != "high" {
		req.ReasoningEffortDefaultValue = "low"
	}
}

// validateSettingValue 根据 schema 的 Min/Max/Enum 校验值。
func validateSettingValue(schema models.SettingSchema, value any) error {
	switch schema.Type {
	case models.SettingTypeInt:
		v, ok := value.(int)
		if !ok {
			return fmt.Errorf("field %s must be int", schema.Field)
		}
		if schema.Min != nil && v < *schema.Min {
			return fmt.Errorf("field %s must be >= %d", schema.Field, *schema.Min)
		}
		if schema.Max != nil && v > *schema.Max {
			return fmt.Errorf("field %s must be <= %d", schema.Field, *schema.Max)
		}
	case models.SettingTypeString:
		v, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %s must be string", schema.Field)
		}
		if len(schema.Enum) > 0 {
			found := false
			for _, e := range schema.Enum {
				if v == e {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("field %s must be one of %v", schema.Field, schema.Enum)
			}
		}
	}
	return nil
}
