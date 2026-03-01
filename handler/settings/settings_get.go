package settings

import (
	"encoding/json"
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

	response := defaultSettingsResponse()
	for _, setting := range settings {
		applySettingToResponse(&response, setting)
	}

	common.Success(c, response)
}

func defaultSettingsResponse() SettingsResponse {
	return SettingsResponse{
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
		ModelSyncEnabled:           false,
		ModelSyncInterval:          12,
		ModelSyncLogRetentionCount: 100,
		ModelSyncLogRetentionDays:  7,
		ModelSyncFilterRules:       []string{},
		// 模板模糊匹配相关默认值
		TemplateFuzzyMatchEnabled:    false,
		TemplateFuzzyMatchSeparators: []string{":", "-"},
		TemplateFuzzyMatchSuffixes:   []string{"free"},
		// reasoning_effort 映射相关默认值
		ReasoningEffortMappingEnabled: true,  // 默认启用
		ReasoningEffortDefaultValue:   "low", // 默认值为 low
	}
}

func applySettingToResponse(response *SettingsResponse, setting models.Setting) {
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
