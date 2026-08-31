package models

import (
	"context"
	"fmt"
	"reflect"
)

// SettingType 表示设置项的值类型。
type SettingType string

const (
	SettingTypeBool        SettingType = "bool"
	SettingTypeInt         SettingType = "int"
	SettingTypeString      SettingType = "string"
	SettingTypeStringSlice SettingType = "[]string"
	SettingTypeJSON        SettingType = "json"
)

// SettingSchema 描述一个设置项的元数据。
type SettingSchema struct {
	Key      string      // 数据库中的 key（snake_case）
	Field    string      // DTO 中的字段名（Go 导出字段名）
	Type     SettingType // 值类型
	Default  any         // 默认值
	Category string      // 用于 UI 分组/批量更新
	Min      *int        // int 最小值校验
	Max      *int        // int 最大值校验
	Enum     []string    // 枚举校验
	Validate func(value any) error
	OnChange func(ctx context.Context, oldValue, newValue any) error
}

// SettingSchemas 返回所有已注册的 schema。
func SettingSchemas() map[string]SettingSchema {
	return settingSchemas
}

// SettingSchemaForKey 根据数据库 key 查找 schema。
func SettingSchemaForKey(key string) (SettingSchema, bool) {
	s, ok := settingSchemas[key]
	return s, ok
}

// SettingSchemaForField 根据 DTO 字段名查找 schema。
func SettingSchemaForField(field string) (SettingSchema, bool) {
	for _, s := range settingSchemas {
		if s.Field == field {
			return s, true
		}
	}
	return SettingSchema{}, false
}

// DefaultValueForKey 返回指定 key 的默认值；若 key 不存在返回 nil。
func DefaultValueForKey(key string) any {
	if s, ok := settingSchemas[key]; ok {
		return s.Default
	}
	return nil
}

var settingSchemas map[string]SettingSchema

func init() {
	settingSchemas = buildSettingSchemas()
}

func intPtr(v int) *int { return &v }

func buildSettingSchemas() map[string]SettingSchema {
	list := []SettingSchema{
		// 能力匹配
		{Key: SettingKeyStrictCapabilityMatch, Field: "StrictCapabilityMatch", Type: SettingTypeBool, Default: true, Category: "capability"},

		// 权重调整
		{Key: SettingKeyAutoWeightDecay, Field: "AutoWeightDecay", Type: SettingTypeBool, Default: false, Category: "weight"},
		{Key: SettingKeyAutoWeightDecayDefault, Field: "AutoWeightDecayDefault", Type: SettingTypeInt, Default: 100, Category: "weight", Min: intPtr(1)},
		{Key: SettingKeyAutoWeightDecayStep, Field: "AutoWeightDecayStep", Type: SettingTypeInt, Default: 1, Category: "weight", Min: intPtr(1)},
		{Key: SettingKeyAutoSuccessIncrease, Field: "AutoSuccessIncrease", Type: SettingTypeBool, Default: true, Category: "weight"},
		{Key: SettingKeyAutoWeightIncreaseStep, Field: "AutoWeightIncreaseStep", Type: SettingTypeInt, Default: 1, Category: "weight", Min: intPtr(1)},
		{Key: SettingKeyAutoWeightIncreaseMax, Field: "AutoWeightIncreaseMax", Type: SettingTypeInt, Default: 100, Category: "weight", Min: intPtr(1)},

		// 优先级调整
		{Key: SettingKeyAutoPriorityDecay, Field: "AutoPriorityDecay", Type: SettingTypeBool, Default: false, Category: "priority"},
		{Key: SettingKeyAutoPriorityDecayDefault, Field: "AutoPriorityDecayDefault", Type: SettingTypeInt, Default: 100, Category: "priority", Min: intPtr(1)},
		{Key: SettingKeyAutoPriorityDecayStep, Field: "AutoPriorityDecayStep", Type: SettingTypeInt, Default: 1, Category: "priority", Min: intPtr(1)},
		{Key: SettingKeyAutoPriorityDecayThreshold, Field: "AutoPriorityDecayThreshold", Type: SettingTypeInt, Default: 90, Category: "priority", Min: intPtr(1)},
		{Key: SettingKeyAutoPriorityDecayDisableEnabled, Field: "AutoPriorityDecayDisableEnabled", Type: SettingTypeBool, Default: true, Category: "priority"},
		{Key: SettingKeyAutoPriorityIncreaseStep, Field: "AutoPriorityIncreaseStep", Type: SettingTypeInt, Default: 1, Category: "priority", Min: intPtr(1)},
		{Key: SettingKeyAutoPriorityIncreaseMax, Field: "AutoPriorityIncreaseMax", Type: SettingTypeInt, Default: 100, Category: "priority", Min: intPtr(0)},

		// 失败处理
		{Key: SettingKeyConsecutiveFailureThreshold, Field: "ConsecutiveFailureThreshold", Type: SettingTypeInt, Default: 3, Category: "failure", Min: intPtr(1)},
		{Key: SettingKeyConsecutiveFailureDisableEnabled, Field: "ConsecutiveFailureDisableEnabled", Type: SettingTypeBool, Default: true, Category: "failure"},

		// 日志
		{Key: SettingKeyLogRetentionCount, Field: "LogRetentionCount", Type: SettingTypeInt, Default: 100, Category: "log", Min: intPtr(0)},
		{Key: SettingKeyLogRawRequestResponse, Field: "LogRawRequestResponse", Type: SettingTypeJSON, Default: RawLogOptions{}, Category: "log"},
		{Key: SettingKeyLogRawRequestResponseErrorsOnly, Field: "LogRawRequestResponseErrorsOnly", Type: SettingTypeBool, Default: false, Category: "log"},
		{Key: SettingKeyDisableAllLogs, Field: "DisableAllLogs", Type: SettingTypeBool, Default: false, Category: "log"},

		// 健康检查
		// 注意：Field 用 HealthCheck 前缀，避免与通用设置 DTO 的同名字段（如 LogRetentionCount）
		// 在 SettingSchemaForField 的全局按名查找中互相覆盖。
		// 这些键由 handler/settings/health.go 单独更新，不参与 UpdateSettingsRequest 的反射遍历，
		// 但必须在此声明——seed 只遍历 SettingSchemas，缺席就不会建行，
		// 而健康检测的更新是 UPDATE-only（行不存在时静默 no-op）。
		{Key: SettingKeyHealthCheckEnabled, Field: "HealthCheckEnabled", Type: SettingTypeBool, Default: false, Category: "health"},
		{Key: SettingKeyHealthCheckInterval, Field: "HealthCheckInterval", Type: SettingTypeInt, Default: 60, Category: "health", Min: intPtr(1)},
		{Key: SettingKeyHealthCheckFailureThreshold, Field: "HealthCheckFailureThreshold", Type: SettingTypeInt, Default: 3, Category: "health", Min: intPtr(1)},
		{Key: SettingKeyHealthCheckFailureDisableEnabled, Field: "HealthCheckFailureDisableEnabled", Type: SettingTypeBool, Default: true, Category: "health"},
		{Key: SettingKeyHealthCheckAutoEnable, Field: "HealthCheckAutoEnable", Type: SettingTypeBool, Default: false, Category: "health"},
		{Key: SettingKeyHealthCheckLogRetentionCount, Field: "HealthCheckLogRetentionCount", Type: SettingTypeInt, Default: 100, Category: "health", Min: intPtr(0)},
		{Key: SettingKeyHealthCheckCheckDisabledOnly, Field: "HealthCheckCheckDisabledOnly", Type: SettingTypeBool, Default: false, Category: "health"},
		{Key: SettingKeyHealthCheckCountAsSuccess, Field: "CountHealthCheckAsSuccess", Type: SettingTypeBool, Default: true, Category: "health"},
		{Key: SettingKeyHealthCheckCountAsFailure, Field: "CountHealthCheckAsFailure", Type: SettingTypeBool, Default: false, Category: "health"},

		// 性能优化
		{Key: SettingKeyDisablePerformanceTracking, Field: "DisablePerformanceTracking", Type: SettingTypeBool, Default: false, Category: "performance"},
		{Key: SettingKeyDisableTokenCounting, Field: "DisableTokenCounting", Type: SettingTypeBool, Default: false, Category: "performance"},
		{Key: SettingKeyEnableRequestTrace, Field: "EnableRequestTrace", Type: SettingTypeBool, Default: true, Category: "performance"},
		{Key: SettingKeyStripResponseHeaders, Field: "StripResponseHeaders", Type: SettingTypeBool, Default: false, Category: "performance"},
		{Key: SettingKeyEnableFormatConversion, Field: "EnableFormatConversion", Type: SettingTypeBool, Default: true, Category: "performance"},

		// 模型关联
		{Key: SettingKeyAutoAssociateOnAdd, Field: "AutoAssociateOnAdd", Type: SettingTypeBool, Default: false, Category: "association"},
		{Key: SettingKeyAutoCleanOnDelete, Field: "AutoCleanOnDelete", Type: SettingTypeBool, Default: false, Category: "association"},
		{Key: SettingKeyAutoSaveTemplateOnAssociate, Field: "AutoSaveTemplateOnAssociate", Type: SettingTypeBool, Default: false, Category: "association"},

		// 模型同步
		{Key: SettingKeyModelSyncEnabled, Field: "ModelSyncEnabled", Type: SettingTypeBool, Default: false, Category: "model_sync"},
		{Key: SettingKeyModelSyncInterval, Field: "ModelSyncInterval", Type: SettingTypeInt, Default: 12, Category: "model_sync", Min: intPtr(1)},
		{Key: SettingKeyModelSyncLogRetentionCount, Field: "ModelSyncLogRetentionCount", Type: SettingTypeInt, Default: 100, Category: "model_sync", Min: intPtr(0)},
		{Key: SettingKeyModelSyncLogRetentionDays, Field: "ModelSyncLogRetentionDays", Type: SettingTypeInt, Default: 7, Category: "model_sync", Min: intPtr(0)},
		{Key: SettingKeyModelSyncFilterRules, Field: "ModelSyncFilterRules", Type: SettingTypeStringSlice, Default: []string{}, Category: "model_sync"},

		// 模板模糊匹配
		{Key: SettingKeyTemplateFuzzyMatchEnabled, Field: "TemplateFuzzyMatchEnabled", Type: SettingTypeBool, Default: false, Category: "template"},
		{Key: SettingKeyTemplateFuzzyMatchSeparators, Field: "TemplateFuzzyMatchSeparators", Type: SettingTypeStringSlice, Default: []string{":", "-"}, Category: "template"},
		{Key: SettingKeyTemplateFuzzyMatchSuffixes, Field: "TemplateFuzzyMatchSuffixes", Type: SettingTypeStringSlice, Default: []string{"free"}, Category: "template"},

		// reasoning_effort 映射
		{Key: SettingKeyReasoningEffortMappingEnabled, Field: "ReasoningEffortMappingEnabled", Type: SettingTypeBool, Default: true, Category: "reasoning"},
		{Key: SettingKeyReasoningEffortDefaultValue, Field: "ReasoningEffortDefaultValue", Type: SettingTypeString, Default: "low", Category: "reasoning", Enum: []string{"minimal", "low", "medium", "high", "xhigh", "max"}},
		{Key: SettingKeyReasoningEffortUnknownStrategy, Field: "ReasoningEffortUnknownStrategy", Type: SettingTypeString, Default: "clamp_to_default", Category: "reasoning", Enum: []string{"clamp_to_default", "passthrough"}},

		// 请求参数（全局超时/重试，替代原 per-model time_out/max_retry）
		// 总预算默认 90 = 响应头超时 30 × 最大重试 3：最坏"每次尝试都等头超时"时，
		// 后续候选仍各有完整等头窗口（等头超时不再一击致命，见 chat_balance 解耦）。
		{Key: SettingKeyRequestHeaderTimeout, Field: "RequestHeaderTimeout", Type: SettingTypeInt, Default: 30, Category: "request", Min: intPtr(1)},
		{Key: SettingKeyRequestTotalTimeout, Field: "RequestTotalTimeout", Type: SettingTypeInt, Default: 90, Category: "request", Min: intPtr(1)},
		{Key: SettingKeyStreamFirstByteTimeout, Field: "StreamFirstByteTimeout", Type: SettingTypeInt, Default: 15, Category: "request", Min: intPtr(1)},
		{Key: SettingKeyRequestMaxRetry, Field: "RequestMaxRetry", Type: SettingTypeInt, Default: 3, Category: "request", Min: intPtr(1)},

		// 凭据健康（S4 状态机 #6-1）：429 与过载合并一个窗口字段，5xx/超时/网络共用一个
		// 窗口（设计定案第 5 节）；鉴权失败(401/403)在 #6-2 判停落地前暂归 server 窗口。
		{Key: SettingKeyCredHealthCooldown429Sec, Field: "CredHealthCooldown429Sec", Type: SettingTypeInt, Default: 60, Category: "cred_health", Min: intPtr(1)},
		{Key: SettingKeyCredHealthCooldownServerSec, Field: "CredHealthCooldownServerSec", Type: SettingTypeInt, Default: 60, Category: "cred_health", Min: intPtr(1)},
	}

	result := make(map[string]SettingSchema, len(list))
	for _, s := range list {
		result[s.Key] = s
	}
	return result
}

// ApplySettingDefaults 用 schema 默认值初始化一个 DTO 结构体指针。
func ApplySettingDefaults(dst any) error {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("dst must be a non-nil pointer")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("dst must point to a struct")
	}

	for _, s := range settingSchemas {
		field := v.FieldByName(s.Field)
		if !field.IsValid() || !field.CanSet() {
			continue
		}
		val := reflect.ValueOf(s.Default)
		if val.IsValid() && field.Type() == val.Type() {
			field.Set(val)
		}
	}
	return nil
}

// SetSettingField 根据 schema 将解析后的值设置到 DTO 结构体指针的对应字段。
func SetSettingField(dst any, s SettingSchema, parsed any) error {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("dst must be a non-nil pointer")
	}
	v = v.Elem()

	field := v.FieldByName(s.Field)
	if !field.IsValid() || !field.CanSet() {
		return fmt.Errorf("field %s not found or not settable", s.Field)
	}

	val := reflect.ValueOf(parsed)
	if !val.IsValid() {
		return nil
	}
	if field.Type() != val.Type() {
		return fmt.Errorf("type mismatch for field %s: expected %s, got %s", s.Field, field.Type(), val.Type())
	}
	field.Set(val)
	return nil
}

// GetSettingField 从 DTO 结构体中获取指定 schema 字段的值。
func GetSettingField(src any, s SettingSchema) (any, bool) {
	v := reflect.ValueOf(src)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, false
	}
	field := v.FieldByName(s.Field)
	if !field.IsValid() || !field.CanInterface() {
		return nil, false
	}
	return field.Interface(), true
}
