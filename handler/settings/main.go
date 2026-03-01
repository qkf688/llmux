package settings

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"

	"github.com/atopos31/llmio/common"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
