package settings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/httpresp"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
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
		retentionCount := req.LogRetentionCount
		bgtask.Go(func(context.Context) { cleanupExcessLogs(retentionCount) })
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

// triggerBatchImportForAutoSave 是「首次开启自动保存模板」的副作用出口。
// 声明为变量以便测试替换（异步 goroutine 无法直接断言）。
var triggerBatchImportForAutoSave = func() {
	bgtask.Go(batchImportExistingAssociations)
}

// updateSettingsFromRequest 根据 SettingSchema 反射遍历 UpdateSettingsRequest 字段并写入数据库。
// 新增设置项只需在 models.SettingSchemas 中声明，无需再修改此处或 section updater。
//
// 校验和写入分两遍执行：先全量校验所有字段，任一失败立即返回且不写库；
// 校验通过后在事务内统一写入，中途失败自动回滚，避免半更新状态。
func updateSettingsFromRequest(ctx context.Context, req *UpdateSettingsRequest) error {
	normalizeUpdateSettingsRequest(req)

	store := settingStore()
	// 必须在写循环之前取旧值：循环会把新值落库，之后再读只能读到新值，
	// 「首次开启」这个跳变条件就永远不成立了。
	oldAutoSave := store.GetBool(ctx, models.SettingKeyAutoSaveTemplateOnAssociate)

	v := reflect.ValueOf(req).Elem()
	t := v.Type()

	// 第一遍：全量校验，不写库。任一字段校验失败立即返回，避免部分字段已写入。
	type fieldUpdate struct {
		schema models.SettingSchema
		value  any
	}
	var updates []fieldUpdate
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
		updates = append(updates, fieldUpdate{schema: schema, value: value})
	}

	// 第二遍：在事务内统一写入。中途失败自动回滚，不会留下半更新状态。
	if err := store.RunInTx(ctx, func(txStore *settings.Store) error {
		for _, u := range updates {
			if err := txStore.SetTyped(ctx, u.schema.Key, u.value); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// 副作用：auto_save_template_on_associate 从关闭切到开启时触发批量导入
	// 在事务提交后触发，不在事务内执行异步操作。
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

	// reasoning_effort_default_value：硬编码 6 档校验（不走 schema enum，独立硬编码与现有风格一致）
	switch req.ReasoningEffortDefaultValue {
	case "minimal", "low", "medium", "high", "xhigh", "max":
		// 合法，保留
	default:
		req.ReasoningEffortDefaultValue = "low"
	}
	// reasoning_effort_unknown_strategy：硬编码枚举校验
	switch req.ReasoningEffortUnknownStrategy {
	case "clamp_to_default", "passthrough":
		// 合法，保留
	default:
		req.ReasoningEffortUnknownStrategy = "clamp_to_default"
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
