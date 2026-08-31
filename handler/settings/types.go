package settings

import (
	"github.com/qkf688/llmux/models"
)

// SystemConfigRequest represents the request body for updating system configuration
type SystemConfigRequest struct {
	EnableSmartRouting  bool    `json:"enable_smart_routing"`
	SuccessRateWeight   float64 `json:"success_rate_weight"`
	ResponseTimeWeight  float64 `json:"response_time_weight"`
	DecayThresholdHours int     `json:"decay_threshold_hours"`
	MinWeight           int     `json:"min_weight"`
}

// Settings 系统设置的共享 DTO 结构体。
// SettingsResponse 与 UpdateSettingsRequest 字段完全一致，通过类型别名复用，
// 新增设置项时只需在此处添加一次字段。
type Settings struct {
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
	LogRawRequestResponseErrorsOnly  bool                 `json:"log_raw_request_response_errors_only"`
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
	ReasoningEffortMappingEnabled  bool   `json:"reasoning_effort_mapping_enabled"`
	ReasoningEffortDefaultValue    string `json:"reasoning_effort_default_value"`    // minimal/low/medium/high/xhigh/max
	ReasoningEffortUnknownStrategy string `json:"reasoning_effort_unknown_strategy"` // clamp_to_default / passthrough
	// 请求参数相关设置（全局超时/重试）
	RequestHeaderTimeout   int `json:"request_header_timeout"`    // 单次尝试等待响应头超时（秒）
	RequestTotalTimeout    int `json:"request_total_timeout"`     // 整个请求总预算超时（秒）
	StreamFirstByteTimeout int `json:"stream_first_byte_timeout"` // 流式响应头后首字节等待超时（秒）
	RequestMaxRetry        int `json:"request_max_retry"`         // 单候选池最大尝试次数
	// 凭据健康相关设置
	CredHealthCooldown429Sec    int `json:"cred_health_cooldown_429_sec"`    // 凭据 429 限流冷却窗口（秒）
	CredHealthCooldownServerSec int `json:"cred_health_cooldown_server_sec"` // 凭据服务端错误冷却窗口（秒，5xx/超时/网络）
	CredHealthAuthFailThreshold int `json:"cred_health_auth_fail_threshold"` // 凭据连续鉴权失败（401/403）判停阈值（次）
}

// SettingsResponse 设置响应结构（类型别名，字段定义统一在 Settings 中）
type SettingsResponse = Settings

// UpdateSettingsRequest 更新设置请求结构（类型别名，字段定义统一在 Settings 中）
type UpdateSettingsRequest = Settings

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
