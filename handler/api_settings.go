package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SystemConfigRequest represents the request body for updating system configuration
type SystemConfigRequest struct {
	EnableSmartRouting  bool    `json:"enable_smart_routing"`
	SuccessRateWeight   float64 `json:"success_rate_weight"`
	ResponseTimeWeight  float64 `json:"response_time_weight"`
	DecayThresholdHours int     `json:"decay_threshold_hours"`
	MinWeight           int     `json:"min_weight"`
}

// SettingsResponse 设置响应结构
type SettingsResponse struct {
	StrictCapabilityMatch            bool                 `json:"strict_capability_match"`
	AutoWeightDecay                  bool                 `json:"auto_weight_decay"`
	AutoWeightDecayDefault           int                  `json:"auto_weight_decay_default"`
	AutoWeightDecayStep              int                  `json:"auto_weight_decay_step"`
	AutoSuccessIncrease              bool                 `json:"auto_success_increase"`
	AutoWeightIncreaseStep           int                  `json:"auto_weight_increase_step"`
	AutoWeightIncreaseMax            int                  `json:"auto_weight_increase_max"`
	AutoPriorityDecay                bool                 `json:"auto_priority_decay"`
	AutoPriorityDecayDefault         int                  `json:"auto_priority_decay_default"`
	AutoPriorityDecayStep            int                  `json:"auto_priority_decay_step"`
	AutoPriorityDecayThreshold       int                  `json:"auto_priority_decay_threshold"`
	AutoPriorityDecayDisableEnabled  bool                 `json:"auto_priority_decay_disable_enabled"`
	AutoPriorityIncreaseStep         int                  `json:"auto_priority_increase_step"`
	AutoPriorityIncreaseMax          int                  `json:"auto_priority_increase_max"`
	ConsecutiveFailureThreshold      int                  `json:"consecutive_failure_threshold"`
	ConsecutiveFailureDisableEnabled bool                 `json:"consecutive_failure_disable_enabled"`
	LogRetentionCount                int                  `json:"log_retention_count"`
	LogRawRequestResponse            models.RawLogOptions `json:"log_raw_request_response"`
	DisableAllLogs                   bool                 `json:"disable_all_logs"`
	CountHealthCheckAsSuccess        bool                 `json:"count_health_check_as_success"`
	CountHealthCheckAsFailure        bool                 `json:"count_health_check_as_failure"`
	// 性能优化相关设置
	DisablePerformanceTracking bool `json:"disable_performance_tracking"`
	DisableTokenCounting       bool `json:"disable_token_counting"`
	EnableRequestTrace         bool `json:"enable_request_trace"`
	StripResponseHeaders       bool `json:"strip_response_headers"`
	EnableFormatConversion     bool `json:"enable_format_conversion"`
	// 模型同步相关设置
	ModelSyncEnabled           bool     `json:"model_sync_enabled"`
	ModelSyncInterval          int      `json:"model_sync_interval"`
	ModelSyncLogRetentionCount int      `json:"model_sync_log_retention_count"`
	ModelSyncLogRetentionDays  int      `json:"model_sync_log_retention_days"`
	ModelSyncFilterRules       []string `json:"model_sync_filter_rules"`
	// 模板模糊匹配相关设置
	TemplateFuzzyMatchEnabled    bool     `json:"template_fuzzy_match_enabled"`
	TemplateFuzzyMatchSeparators []string `json:"template_fuzzy_match_separators"`
	TemplateFuzzyMatchSuffixes   []string `json:"template_fuzzy_match_suffixes"`
	// 模型关联相关设置
	AutoAssociateOnAdd          bool `json:"auto_associate_on_add"`
	AutoCleanOnDelete           bool `json:"auto_clean_on_delete"`
	AutoSaveTemplateOnAssociate bool `json:"auto_save_template_on_associate"`
	// reasoning_effort 映射相关设置
	ReasoningEffortMappingEnabled bool   `json:"reasoning_effort_mapping_enabled"`
	ReasoningEffortDefaultValue   string `json:"reasoning_effort_default_value"` // low/medium/high
}

// UpdateSettingsRequest 更新设置请求结构
type UpdateSettingsRequest struct {
	StrictCapabilityMatch            bool                 `json:"strict_capability_match"`
	AutoWeightDecay                  bool                 `json:"auto_weight_decay"`
	AutoWeightDecayDefault           int                  `json:"auto_weight_decay_default"`
	AutoWeightDecayStep              int                  `json:"auto_weight_decay_step"`
	AutoSuccessIncrease              bool                 `json:"auto_success_increase"`
	AutoWeightIncreaseStep           int                  `json:"auto_weight_increase_step"`
	AutoWeightIncreaseMax            int                  `json:"auto_weight_increase_max"`
	AutoPriorityDecay                bool                 `json:"auto_priority_decay"`
	AutoPriorityDecayDefault         int                  `json:"auto_priority_decay_default"`
	AutoPriorityDecayStep            int                  `json:"auto_priority_decay_step"`
	AutoPriorityDecayThreshold       int                  `json:"auto_priority_decay_threshold"`
	AutoPriorityDecayDisableEnabled  bool                 `json:"auto_priority_decay_disable_enabled"`
	AutoPriorityIncreaseStep         int                  `json:"auto_priority_increase_step"`
	AutoPriorityIncreaseMax          int                  `json:"auto_priority_increase_max"`
	ConsecutiveFailureThreshold      int                  `json:"consecutive_failure_threshold"`
	ConsecutiveFailureDisableEnabled bool                 `json:"consecutive_failure_disable_enabled"`
	LogRetentionCount                int                  `json:"log_retention_count"`
	LogRawRequestResponse            models.RawLogOptions `json:"log_raw_request_response"`
	DisableAllLogs                   bool                 `json:"disable_all_logs"`
	CountHealthCheckAsSuccess        bool                 `json:"count_health_check_as_success"`
	CountHealthCheckAsFailure        bool                 `json:"count_health_check_as_failure"`
	// 性能优化相关设置
	DisablePerformanceTracking bool `json:"disable_performance_tracking"`
	DisableTokenCounting       bool `json:"disable_token_counting"`
	EnableRequestTrace         bool `json:"enable_request_trace"`
	StripResponseHeaders       bool `json:"strip_response_headers"`
	EnableFormatConversion     bool `json:"enable_format_conversion"`
	// 模型同步相关设置
	ModelSyncEnabled           bool     `json:"model_sync_enabled"`
	ModelSyncInterval          int      `json:"model_sync_interval"`
	ModelSyncLogRetentionCount int      `json:"model_sync_log_retention_count"`
	ModelSyncLogRetentionDays  int      `json:"model_sync_log_retention_days"`
	ModelSyncFilterRules       []string `json:"model_sync_filter_rules"`
	// 模板模糊匹配相关设置
	TemplateFuzzyMatchEnabled    bool     `json:"template_fuzzy_match_enabled"`
	TemplateFuzzyMatchSeparators []string `json:"template_fuzzy_match_separators"`
	TemplateFuzzyMatchSuffixes   []string `json:"template_fuzzy_match_suffixes"`
	// 模型关联相关设置
	AutoAssociateOnAdd          bool `json:"auto_associate_on_add"`
	AutoCleanOnDelete           bool `json:"auto_clean_on_delete"`
	AutoSaveTemplateOnAssociate bool `json:"auto_save_template_on_associate"`
	// reasoning_effort 映射相关设置
	ReasoningEffortMappingEnabled bool   `json:"reasoning_effort_mapping_enabled"`
	ReasoningEffortDefaultValue   string `json:"reasoning_effort_default_value"`
}

// HealthCheckSettingsResponse 健康检测设置响应结构
type HealthCheckSettingsResponse struct {
	Enabled                 bool `json:"enabled"`
	Interval                int  `json:"interval"`
	FailureThreshold        int  `json:"failure_threshold"`
	FailureDisableEnabled   bool `json:"failure_disable_enabled"`
	AutoEnable              bool `json:"auto_enable"`
	LogRetentionCount       int  `json:"log_retention_count"`
	CountHealthCheckSuccess bool `json:"count_health_check_as_success"`
	CountHealthCheckFailure bool `json:"count_health_check_as_failure"`
	CheckDisabledOnly       bool `json:"check_disabled_only"`
}

// UpdateHealthCheckSettingsRequest 更新健康检测设置请求结构
type UpdateHealthCheckSettingsRequest struct {
	Enabled                 bool `json:"enabled"`
	Interval                int  `json:"interval"`
	FailureThreshold        int  `json:"failure_threshold"`
	FailureDisableEnabled   bool `json:"failure_disable_enabled"`
	AutoEnable              bool `json:"auto_enable"`
	LogRetentionCount       int  `json:"log_retention_count"`
	CountHealthCheckSuccess bool `json:"count_health_check_as_success"`
	CountHealthCheckFailure bool `json:"count_health_check_as_failure"`
	CheckDisabledOnly       bool `json:"check_disabled_only"`
}

// GetSystemConfig 获取系统配置
func GetSystemConfig(c *gin.Context) {
	config := map[string]interface{}{
		"enable_smart_routing":  true,
		"success_rate_weight":   0.7,
		"response_time_weight":  0.3,
		"decay_threshold_hours": 24,
		"min_weight":            1,
	}

	common.Success(c, config)
}

// UpdateSystemConfig 更新系统配置
func UpdateSystemConfig(c *gin.Context) {
	var req SystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	config := map[string]interface{}{
		"enable_smart_routing":  req.EnableSmartRouting,
		"success_rate_weight":   req.SuccessRateWeight,
		"response_time_weight":  req.ResponseTimeWeight,
		"decay_threshold_hours": req.DecayThresholdHours,
		"min_weight":            req.MinWeight,
	}

	common.Success(c, config)
}

// GetSettings 获取所有设置
func GetSettings(c *gin.Context) {
	settings, err := gorm.G[models.Setting](models.DB).Find(c.Request.Context())
	if err != nil {
		common.InternalServerError(c, "Failed to get settings: "+err.Error())
		return
	}

	// 构建响应
	response := SettingsResponse{
		StrictCapabilityMatch:            true, // 默认值
		AutoWeightDecay:                  false,
		AutoWeightDecayDefault:           100,
		AutoWeightDecayStep:              1,
		AutoSuccessIncrease:              true,
		AutoWeightIncreaseStep:           1,
		AutoWeightIncreaseMax:            100,
		AutoPriorityDecay:                false,
		AutoPriorityDecayDefault:         100,
		AutoPriorityDecayStep:            1,
		AutoPriorityDecayThreshold:       90,
		AutoPriorityDecayDisableEnabled:  true, // 默认启用自动禁用功能
		LogRetentionCount:                100,  // 默认保留100条
		AutoPriorityIncreaseStep:         1,
		AutoPriorityIncreaseMax:          100,
		ConsecutiveFailureThreshold:      3,
		ConsecutiveFailureDisableEnabled: true,
		CountHealthCheckAsSuccess:        true,
		CountHealthCheckAsFailure:        false,
		// 性能优化相关默认值
		DisablePerformanceTracking: false,
		DisableTokenCounting:       false,
		EnableRequestTrace:         true, // 默认启用
		StripResponseHeaders:       false,
		EnableFormatConversion:     true,
		// 模型同步相关默认值
		ModelSyncEnabled:             false,
		ModelSyncInterval:            12,
		ModelSyncLogRetentionCount:   100,
		ModelSyncLogRetentionDays:    7,
		ModelSyncFilterRules:         []string{},
		// 模板模糊匹配相关默认值
		TemplateFuzzyMatchEnabled:    false,
		TemplateFuzzyMatchSeparators: []string{":", "-"},
		TemplateFuzzyMatchSuffixes:   []string{"free"},
		// reasoning_effort 映射相关默认值
		ReasoningEffortMappingEnabled: true,  // 默认启用
		ReasoningEffortDefaultValue:   "low", // 默认值为 low
	}

	for _, setting := range settings {
		switch setting.Key {
		case models.SettingKeyStrictCapabilityMatch:
			response.StrictCapabilityMatch = setting.Value == "true"
		case models.SettingKeyAutoWeightDecay:
			response.AutoWeightDecay = setting.Value == "true"
		case models.SettingKeyAutoWeightDecayDefault:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoWeightDecayDefault = val
			}
		case models.SettingKeyAutoWeightDecayStep:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoWeightDecayStep = val
			}
		case models.SettingKeyAutoSuccessIncrease:
			response.AutoSuccessIncrease = setting.Value == "true"
		case models.SettingKeyAutoWeightIncreaseStep:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoWeightIncreaseStep = val
			}
		case models.SettingKeyAutoWeightIncreaseMax:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoWeightIncreaseMax = val
			}
		case models.SettingKeyAutoPriorityDecay:
			response.AutoPriorityDecay = setting.Value == "true"
		case models.SettingKeyAutoPriorityDecayDefault:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoPriorityDecayDefault = val
			}
		case models.SettingKeyAutoPriorityDecayStep:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoPriorityDecayStep = val
			}
		case models.SettingKeyAutoPriorityDecayThreshold:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoPriorityDecayThreshold = val
			}
		case models.SettingKeyAutoPriorityDecayDisableEnabled:
			response.AutoPriorityDecayDisableEnabled = setting.Value == "true"
		case models.SettingKeyAutoPriorityIncreaseStep:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoPriorityIncreaseStep = val
			}
		case models.SettingKeyAutoPriorityIncreaseMax:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.AutoPriorityIncreaseMax = val
			}
		case models.SettingKeyConsecutiveFailureThreshold:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.ConsecutiveFailureThreshold = val
			}
		case models.SettingKeyConsecutiveFailureDisableEnabled:
			response.ConsecutiveFailureDisableEnabled = setting.Value == "true"
		case models.SettingKeyLogRetentionCount:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.LogRetentionCount = val
			}
		case models.SettingKeyLogRawRequestResponse:
			var options models.RawLogOptions
			if err := json.Unmarshal([]byte(setting.Value), &options); err == nil {
				response.LogRawRequestResponse = options
			}
		case models.SettingKeyDisableAllLogs:
			response.DisableAllLogs = setting.Value == "true"
		case models.SettingKeyHealthCheckCountAsSuccess:
			response.CountHealthCheckAsSuccess = setting.Value == "true"
		case models.SettingKeyHealthCheckCountAsFailure:
			response.CountHealthCheckAsFailure = setting.Value == "true"
		case models.SettingKeyDisablePerformanceTracking:
			response.DisablePerformanceTracking = setting.Value == "true"
		case models.SettingKeyDisableTokenCounting:
			response.DisableTokenCounting = setting.Value == "true"
		case models.SettingKeyEnableRequestTrace:
			response.EnableRequestTrace = setting.Value == "true"
		case models.SettingKeyStripResponseHeaders:
			response.StripResponseHeaders = setting.Value == "true"
		case models.SettingKeyEnableFormatConversion:
			response.EnableFormatConversion = setting.Value == "true"
		case models.SettingKeyAutoAssociateOnAdd:
			response.AutoAssociateOnAdd = setting.Value == "true"
		case models.SettingKeyAutoCleanOnDelete:
			response.AutoCleanOnDelete = setting.Value == "true"
		case models.SettingKeyAutoSaveTemplateOnAssociate:
			response.AutoSaveTemplateOnAssociate = setting.Value == "true"
		case models.SettingKeyModelSyncEnabled:
			response.ModelSyncEnabled = setting.Value == "true"
		case models.SettingKeyModelSyncInterval:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.ModelSyncInterval = val
			}
		case models.SettingKeyModelSyncLogRetentionCount:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.ModelSyncLogRetentionCount = val
			}
		case models.SettingKeyModelSyncLogRetentionDays:
			if val, err := strconv.Atoi(setting.Value); err == nil {
				response.ModelSyncLogRetentionDays = val
			}
		case models.SettingKeyModelSyncFilterRules:
			var rules []string
			if err := json.Unmarshal([]byte(setting.Value), &rules); err == nil {
				response.ModelSyncFilterRules = rules
			}
		case models.SettingKeyTemplateFuzzyMatchEnabled:
			response.TemplateFuzzyMatchEnabled = setting.Value == "true"
		case models.SettingKeyTemplateFuzzyMatchSeparators:
			var seps []string
			if err := json.Unmarshal([]byte(setting.Value), &seps); err == nil {
				response.TemplateFuzzyMatchSeparators = seps
			}
		case models.SettingKeyTemplateFuzzyMatchSuffixes:
			var suffs []string
			if err := json.Unmarshal([]byte(setting.Value), &suffs); err == nil {
				response.TemplateFuzzyMatchSuffixes = suffs
			}
		case models.SettingKeyReasoningEffortMappingEnabled:
			response.ReasoningEffortMappingEnabled = setting.Value == "true"
		case models.SettingKeyReasoningEffortDefaultValue:
			response.ReasoningEffortDefaultValue = setting.Value
		}
	}

	common.Success(c, response)
}

// UpdateSettings 更新设置
func UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// 更新严格能力匹配设置
	strictValue := "false"
	if req.StrictCapabilityMatch {
		strictValue = "true"
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyStrictCapabilityMatch).
		Update(ctx, "value", strictValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动权重衰减开关
	autoWeightDecayValue := "false"
	if req.AutoWeightDecay {
		autoWeightDecayValue = "true"
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecay).
		Update(ctx, "value", autoWeightDecayValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动权重衰减默认值
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecayDefault).
		Update(ctx, "value", strconv.Itoa(req.AutoWeightDecayDefault)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动权重衰减步长
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightDecayStep).
		Update(ctx, "value", strconv.Itoa(req.AutoWeightDecayStep)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if req.AutoSuccessIncrease {
		if req.AutoWeightIncreaseStep < 1 {
			req.AutoWeightIncreaseStep = 1
		}
		if req.AutoWeightIncreaseMax < 1 {
			req.AutoWeightIncreaseMax = 100
		}
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoSuccessIncrease).
		Update(ctx, "value", strconv.FormatBool(req.AutoSuccessIncrease)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightIncreaseStep).
		Update(ctx, "value", strconv.Itoa(req.AutoWeightIncreaseStep)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoWeightIncreaseMax).
		Update(ctx, "value", strconv.Itoa(req.AutoWeightIncreaseMax)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动优先级衰减开关
	autoPriorityDecayValue := "false"
	if req.AutoPriorityDecay {
		autoPriorityDecayValue = "true"
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecay).
		Update(ctx, "value", autoPriorityDecayValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动优先级衰减默认值
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayDefault).
		Update(ctx, "value", strconv.Itoa(req.AutoPriorityDecayDefault)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动优先级衰减步长
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayStep).
		Update(ctx, "value", strconv.Itoa(req.AutoPriorityDecayStep)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动优先级衰减阈值
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayThreshold).
		Update(ctx, "value", strconv.Itoa(req.AutoPriorityDecayThreshold)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动优先级衰减禁用开关
	autoPriorityDecayDisableEnabledValue := "false"
	if req.AutoPriorityDecayDisableEnabled {
		autoPriorityDecayDisableEnabledValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayDisableEnabled).
		Update(ctx, "value", autoPriorityDecayDisableEnabledValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if req.AutoPriorityIncreaseStep < 1 {
		req.AutoPriorityIncreaseStep = 1
	}
	if req.AutoPriorityIncreaseMax < 0 {
		req.AutoPriorityIncreaseMax = 100
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityIncreaseStep).
		Update(ctx, "value", strconv.Itoa(req.AutoPriorityIncreaseStep)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityIncreaseMax).
		Update(ctx, "value", strconv.Itoa(req.AutoPriorityIncreaseMax)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	if req.ConsecutiveFailureThreshold < 1 {
		req.ConsecutiveFailureThreshold = 3
	}

	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyConsecutiveFailureThreshold).
		Update(ctx, "value", strconv.Itoa(req.ConsecutiveFailureThreshold)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	consecutiveFailureDisableValue := "false"
	if req.ConsecutiveFailureDisableEnabled {
		consecutiveFailureDisableValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyConsecutiveFailureDisableEnabled).
		Update(ctx, "value", consecutiveFailureDisableValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	countHealthCheckValue := "false"
	if req.CountHealthCheckAsSuccess {
		countHealthCheckValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsSuccess).
		Update(ctx, "value", countHealthCheckValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	countHealthCheckFailureValue := "false"
	if req.CountHealthCheckAsFailure {
		countHealthCheckFailureValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsFailure).
		Update(ctx, "value", countHealthCheckFailureValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新日志保留条数设置
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyLogRetentionCount).
		Update(ctx, "value", strconv.Itoa(req.LogRetentionCount)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新原始请求响应记录选项
	logRawOptionsJSON, err := json.Marshal(req.LogRawRequestResponse)
	if err != nil {
		common.InternalServerError(c, "Failed to marshal log raw options: "+err.Error())
		return
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyLogRawRequestResponse).
		Update(ctx, "value", string(logRawOptionsJSON)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新完全关闭日志记录开关
	disableAllLogsValue := "false"
	if req.DisableAllLogs {
		disableAllLogsValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyDisableAllLogs).
		Update(ctx, "value", disableAllLogsValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新性能优化相关设置
	disablePerformanceTrackingValue := "false"
	if req.DisablePerformanceTracking {
		disablePerformanceTrackingValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyDisablePerformanceTracking).
		Update(ctx, "value", disablePerformanceTrackingValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	disableTokenCountingValue := "false"
	if req.DisableTokenCounting {
		disableTokenCountingValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyDisableTokenCounting).
		Update(ctx, "value", disableTokenCountingValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	enableRequestTraceValue := "false"
	if req.EnableRequestTrace {
		enableRequestTraceValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyEnableRequestTrace).
		Update(ctx, "value", enableRequestTraceValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新移除响应头设置
	stripResponseHeadersValue := "false"
	if req.StripResponseHeaders {
		stripResponseHeadersValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyStripResponseHeaders).
		Update(ctx, "value", stripResponseHeadersValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新格式转换设置
	enableFormatConversionValue := "false"
	if req.EnableFormatConversion {
		enableFormatConversionValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyEnableFormatConversion).
		Update(ctx, "value", enableFormatConversionValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新模型关联相关设置
	autoAssociateValue := "false"
	if req.AutoAssociateOnAdd {
		autoAssociateValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoAssociateOnAdd).
		Update(ctx, "value", autoAssociateValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	autoCleanValue := "false"
	if req.AutoCleanOnDelete {
		autoCleanValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoCleanOnDelete).
		Update(ctx, "value", autoCleanValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新 auto_save_template_on_associate 设置
	// 检查是否首次开启，如果是则触发批量导入
	oldAutoSaveValue := GetSettingBool(ctx, models.SettingKeyAutoSaveTemplateOnAssociate)
	newAutoSaveValue := req.AutoSaveTemplateOnAssociate

	autoSaveTemplateValue := "false"
	if newAutoSaveValue {
		autoSaveTemplateValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoSaveTemplateOnAssociate).
		Update(ctx, "value", autoSaveTemplateValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 如果是首次开启，触发批量导入
	if !oldAutoSaveValue && newAutoSaveValue {
		go batchImportExistingAssociations(context.Background())
		slog.Info("triggered batch import of existing associations")
	}

	// 更新模型同步开关
	modelSyncEnabledValue := "false"
	if req.ModelSyncEnabled {
		modelSyncEnabledValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyModelSyncEnabled).
		Update(ctx, "value", modelSyncEnabledValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新模型同步间隔
	if req.ModelSyncInterval < 1 {
		req.ModelSyncInterval = 12
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyModelSyncInterval).
		Update(ctx, "value", strconv.Itoa(req.ModelSyncInterval)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新模型同步日志保留条数
	if req.ModelSyncLogRetentionCount < 0 {
		req.ModelSyncLogRetentionCount = 100
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyModelSyncLogRetentionCount).
		Update(ctx, "value", strconv.Itoa(req.ModelSyncLogRetentionCount)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新模型同步日志保留天数
	if req.ModelSyncLogRetentionDays < 0 {
		req.ModelSyncLogRetentionDays = 7
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyModelSyncLogRetentionDays).
		Update(ctx, "value", strconv.Itoa(req.ModelSyncLogRetentionDays)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新模型同步过滤规则
	filterRulesJSON, err := json.Marshal(req.ModelSyncFilterRules)
	if err != nil {
		common.InternalServerError(c, "Failed to marshal filter rules: "+err.Error())
		return
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyModelSyncFilterRules).
		Update(ctx, "value", string(filterRulesJSON)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新模板模糊匹配开关
	templateFuzzyMatchValue := "false"
	if req.TemplateFuzzyMatchEnabled {
		templateFuzzyMatchValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyTemplateFuzzyMatchEnabled).
		Update(ctx, "value", templateFuzzyMatchValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新模板模糊匹配分隔符
	separatorsJSON, err := json.Marshal(req.TemplateFuzzyMatchSeparators)
	if err != nil {
		common.InternalServerError(c, "Failed to marshal separators: "+err.Error())
		return
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyTemplateFuzzyMatchSeparators).
		Update(ctx, "value", string(separatorsJSON)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新模板模糊匹配后缀
	suffixesJSON, err := json.Marshal(req.TemplateFuzzyMatchSuffixes)
	if err != nil {
		common.InternalServerError(c, "Failed to marshal suffixes: "+err.Error())
		return
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyTemplateFuzzyMatchSuffixes).
		Update(ctx, "value", string(suffixesJSON)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新 reasoning_effort 映射开关
	reasoningEffortMappingEnabledValue := "false"
	if req.ReasoningEffortMappingEnabled {
		reasoningEffortMappingEnabledValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyReasoningEffortMappingEnabled).
		Update(ctx, "value", reasoningEffortMappingEnabledValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新 reasoning_effort 默认值（验证合法性）
	if req.ReasoningEffortDefaultValue != "low" &&
		req.ReasoningEffortDefaultValue != "medium" &&
		req.ReasoningEffortDefaultValue != "high" {
		req.ReasoningEffortDefaultValue = "low"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyReasoningEffortDefaultValue).
		Update(ctx, "value", req.ReasoningEffortDefaultValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 如果设置了保留条数限制，立即执行清理
	if req.LogRetentionCount > 0 {
		go cleanupExcessLogs(req.LogRetentionCount)
	}

	// 返回更新后的设置
	GetSettings(c)
}

// cleanupExcessLogs 清理超出保留条数的日志
func cleanupExcessLogs(retentionCount int) {
	// 获取总日志数
	var total int64
	if err := models.DB.Model(&models.ChatLog{}).Count(&total).Error; err != nil {
		slog.Error("failed to count logs for cleanup", "error", err)
		return
	}

	// 如果日志数超过保留条数，删除多余的
	if int(total) > retentionCount {
		deleteCount := int(total) - retentionCount

		// 获取需要删除的日志ID（最旧的）
		var logsToDelete []models.ChatLog
		if err := models.DB.Model(&models.ChatLog{}).
			Order("id ASC").
			Limit(deleteCount).
			Find(&logsToDelete).Error; err != nil {
			slog.Error("failed to find logs to delete", "error", err)
			return
		}

		// 提取ID列表
		ids := make([]uint, len(logsToDelete))
		for i, log := range logsToDelete {
			ids[i] = log.ID
		}

		// 删除对应的ChatIO记录
		if err := models.DB.Unscoped().
			Where("log_id IN ?", ids).
			Delete(&models.ChatIO{}).Error; err != nil {
			slog.Error("failed to delete chat io records", "error", err)
		}

		// 删除日志记录
		if err := models.DB.Unscoped().
			Where("id IN ?", ids).
			Delete(&models.ChatLog{}).Error; err != nil {
			slog.Error("failed to delete logs hard", "error", err)
			return
		}

		slog.Info("cleaned up excess logs", "deleted", deleteCount, "retention", retentionCount)
	}
}

// GetHealthCheckSettings 获取健康检测设置
func GetHealthCheckSettings(c *gin.Context) {
	ctx := c.Request.Context()
	enabled, interval, failureThreshold, failureDisableEnabled, autoEnable, logRetentionCount, countAsSuccess, countAsFailure, checkDisabledOnly := service.GetHealthCheckSettings(ctx)

	response := HealthCheckSettingsResponse{
		Enabled:                 enabled,
		Interval:                interval,
		FailureThreshold:        failureThreshold,
		FailureDisableEnabled:   failureDisableEnabled,
		AutoEnable:              autoEnable,
		LogRetentionCount:       logRetentionCount,
		CountHealthCheckSuccess: countAsSuccess,
		CountHealthCheckFailure: countAsFailure,
		CheckDisabledOnly:       checkDisabledOnly,
	}

	common.Success(c, response)
}

// UpdateHealthCheckSettings 更新健康检测设置
func UpdateHealthCheckSettings(c *gin.Context) {
	var req UpdateHealthCheckSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	// 更新启用状态
	enabledValue := "false"
	if req.Enabled {
		enabledValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckEnabled).
		Update(ctx, "value", enabledValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新检测间隔
	if req.Interval < 1 {
		req.Interval = 60
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckInterval).
		Update(ctx, "value", strconv.Itoa(req.Interval)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新失败次数阈值
	if req.FailureThreshold < 1 {
		req.FailureThreshold = 3
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckFailureThreshold).
		Update(ctx, "value", strconv.Itoa(req.FailureThreshold)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新失败自动禁用开关
	failureDisableEnabledValue := "false"
	if req.FailureDisableEnabled {
		failureDisableEnabledValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckFailureDisableEnabled).
		Update(ctx, "value", failureDisableEnabledValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新自动启用
	autoEnableValue := "false"
	if req.AutoEnable {
		autoEnableValue = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckAutoEnable).
		Update(ctx, "value", autoEnableValue); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新健康检测日志保留条数
	if req.LogRetentionCount < 0 {
		req.LogRetentionCount = 0
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckLogRetentionCount).
		Update(ctx, "value", strconv.Itoa(req.LogRetentionCount)); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	countHealthCheckSuccess := "false"
	if req.CountHealthCheckSuccess {
		countHealthCheckSuccess = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsSuccess).
		Update(ctx, "value", countHealthCheckSuccess); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	countHealthCheckFailure := "false"
	if req.CountHealthCheckFailure {
		countHealthCheckFailure = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCountAsFailure).
		Update(ctx, "value", countHealthCheckFailure); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 更新只检测停用的模型设置
	checkDisabledOnly := "false"
	if req.CheckDisabledOnly {
		checkDisabledOnly = "true"
	}
	if _, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyHealthCheckCheckDisabledOnly).
		Update(ctx, "value", checkDisabledOnly); err != nil {
		common.InternalServerError(c, "Failed to update settings: "+err.Error())
		return
	}

	// 重启健康检测服务
	go service.GetHealthChecker().Restart(context.Background())

	// 执行日志清理以满足新的保留策略
	go service.EnforceHealthCheckLogRetention(context.Background())

	// 返回更新后的设置
	GetHealthCheckSettings(c)
}

// GetStrictCapabilityMatch 获取严格能力匹配设置
func GetStrictCapabilityMatch(ctx context.Context) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyStrictCapabilityMatch).
		First(ctx)
	if err != nil {
		return true // 默认开启
	}
	return setting.Value == "true"
}

// GetAutoPriorityDecayDefault 获取自动优先级衰减默认值
func GetAutoPriorityDecayDefault(ctx context.Context) int {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", models.SettingKeyAutoPriorityDecayDefault).
		First(ctx)
	if err != nil {
		return 100 // 默认优先级100
	}
	val, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 100
	}
	return val
}

// GetSettingBool 获取设置的布尔值
func GetSettingBool(ctx context.Context, key string) bool {
	setting, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", key).
		First(ctx)
	if err != nil {
		return false
	}
	return setting.Value == "true"
}

// ResetModelWeights 重置所有模型权重
func ResetModelWeights(c *gin.Context) {
	ctx := c.Request.Context()
	
	// 重置所有模型权重为默认值
	if err := models.DB.WithContext(ctx).Model(&models.ModelWithProvider{}).Update("weight", 5).Error; err != nil {
		slog.Error("重置模型权重失败", "error", err)
		common.InternalServerError(c, "重置模型权重失败: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"message": "模型权重已重置为默认值",
		"timestamp": time.Now(),
	})
}

// ResetModelPriorities 重置所有模型优先级
func ResetModelPriorities(c *gin.Context) {
	ctx := c.Request.Context()
	
	// 获取默认优先级值
	defaultPriority := GetAutoPriorityDecayDefault(ctx)
	
	// 重置所有模型优先级为默认值
	if err := models.DB.WithContext(ctx).Model(&models.ModelWithProvider{}).Update("priority", defaultPriority).Error; err != nil {
		slog.Error("重置模型优先级失败", "error", err)
		common.InternalServerError(c, "重置模型优先级失败: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"message": "模型优先级已重置为默认值",
		"timestamp": time.Now(),
	})
}

// EnableAllAssociations 启用所有模型关联
func EnableAllAssociations(c *gin.Context) {
	ctx := c.Request.Context()
	
	// 启用所有模型关联
	trueVal := true
	if err := models.DB.WithContext(ctx).Model(&models.ModelWithProvider{}).Update("status", &trueVal).Error; err != nil {
		slog.Error("启用所有模型关联失败", "error", err)
		common.InternalServerError(c, "启用所有模型关联失败: "+err.Error())
		return
	}

	common.Success(c, map[string]interface{}{
		"message": "所有模型关联已启用",
		"timestamp": time.Now(),
	})
}
