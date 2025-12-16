package models

import (
	"net/http"
	"time"

	"gorm.io/gorm"
)

type Provider struct {
	gorm.Model
	Name    string
	Type    string
	Config  string
	Console string // 控制台地址
	Proxy   string // 代理地址
}

type AnthropicConfig struct {
	BaseUrl string `json:"base_url"`
	ApiKey  string `json:"api_key"`
	Version string `json:"version"`
}

type Model struct {
	gorm.Model
	Name     string
	Remark   string
	MaxRetry int   // 重试次数限制
	TimeOut  int   // 超时时间 单位秒
	IOLog    *bool // 是否记录IO
}

type ModelWithProvider struct {
	gorm.Model
	ModelID          uint
	ProviderModel    string
	ProviderID       uint
	ToolCall         *bool             // 能否接受带有工具调用的请求
	StructuredOutput *bool             // 能否接受带有结构化输出的请求
	Image            *bool             // 能否接受带有图片的请求(视觉)
	WithHeader       *bool             // 是否透传header
	Status           *bool             // 是否启用
	CustomerHeaders  map[string]string `gorm:"serializer:json"` // 自定义headers
	Weight           int
	Priority         int // 优先级，值越高越优先选择
}

type ChatLog struct {
	gorm.Model
	Name          string `gorm:"index"`
	ProviderModel string `gorm:"index"`
	ProviderName  string `gorm:"index"`
	Status        string `gorm:"index"` // error or success
	Style         string // 类型
	UserAgent     string `gorm:"index"` // 用户代理
	RemoteIP      string // 访问ip
	ChatIO        bool   // 是否开启IO记录

	Error          string        // if status is error, this field will be set
	Retry          int           // 重试次数
	ProxyTime      time.Duration // 代理耗时
	FirstChunkTime time.Duration // 首个chunk耗时
	ChunkTime      time.Duration // chunk耗时
	Tps            float64
	Usage

	// 原始请求和响应内容
	RequestHeaders     string // 请求头JSON字符串
	RequestBody        string // 请求体
	ResponseHeaders    string // 响应头JSON字符串
	ResponseBody       string // 响应体（转换后）
	RawResponseBody    string // 原始响应体（转换前）
}

func (l ChatLog) WithError(err error) ChatLog {
	l.Error = err.Error()
	l.Status = "error"
	return l
}

type Usage struct {
	PromptTokens        int64               `json:"prompt_tokens"`
	CompletionTokens    int64               `json:"completion_tokens"`
	TotalTokens         int64               `json:"total_tokens"`
	PromptTokensDetails PromptTokensDetails `json:"prompt_tokens_details" gorm:"serializer:json"`
}

type PromptTokensDetails struct {
	CachedTokens int64 `json:"cached_tokens"`
	AudioTokens  int64 `json:"audio_tokens"`
}

type ChatIO struct {
	gorm.Model
	LogId uint
	Input string
	OutputUnion
}

type OutputUnion struct {
	OfString      string
	OfStringArray []string `gorm:"serializer:json"`
}

type ReqMeta struct {
	UserAgent string `gorm:"index"` // 用户代理
	RemoteIP  string // 访问ip
	Header    http.Header
}

// Setting 系统设置
type Setting struct {
	gorm.Model
	Key   string `gorm:"uniqueIndex"` // 设置键名
	Value string // 设置值
}

// RawLogOptions 原始日志记录选项
type RawLogOptions struct {
	RequestHeaders  bool `json:"request_headers"`   // 记录请求头
	RequestBody     bool `json:"request_body"`      // 记录请求体
	ResponseHeaders bool `json:"response_headers"`  // 记录响应头
	ResponseBody    bool `json:"response_body"`     // 记录响应体
	RawResponseBody bool `json:"raw_response_body"` // 记录原始响应体（转换前）
}

// 设置键常量
const (
	SettingKeyStrictCapabilityMatch  = "strict_capability_match"   // 严格能力匹配开关
	SettingKeyAutoWeightDecay        = "auto_weight_decay"         // 自动权重衰减开关
	SettingKeyAutoWeightDecayDefault = "auto_weight_decay_default" // 自动权重衰减默认权重
	SettingKeyAutoWeightDecayStep    = "auto_weight_decay_step"    // 自动权重衰减步长（每次失败减少的权重）
	SettingKeyAutoWeightIncreaseStep = "auto_weight_increase_step" // 自动权重增加步长（每次成功增加的权重）
	SettingKeyAutoWeightIncreaseMax  = "auto_weight_increase_max"  // 自动权重增加的上限

	SettingKeyAutoPriorityDecay               = "auto_priority_decay"                 // 自动优先级衰减开关
	SettingKeyAutoPriorityDecayDefault        = "auto_priority_decay_default"         // 自动优先级衰减默认优先级
	SettingKeyAutoPriorityDecayStep           = "auto_priority_decay_step"            // 自动优先级衰减步长（每次失败减少的优先级）
	SettingKeyAutoPriorityDecayThreshold      = "auto_priority_decay_threshold"       // 自动优先级衰减阈值（达到此值自动禁用）
	SettingKeyAutoPriorityDecayDisableEnabled = "auto_priority_decay_disable_enabled" // 是否启用自动禁用功能（达到阈值时禁用）
	SettingKeyAutoPriorityIncreaseStep        = "auto_priority_increase_step"         // 自动优先级增加步长（每次成功增加的优先级）
	SettingKeyAutoPriorityIncreaseMax         = "auto_priority_increase_max"          // 自动优先级增加的上限
	SettingKeyAutoSuccessIncrease             = "auto_success_increase"               // 成功调用后是否执行自增

	SettingKeyLogRetentionCount     = "log_retention_count"      // 日志保留条数，0表示不限制
	SettingKeyLogRawRequestResponse = "log_raw_request_response" // 原始请求响应记录选项（JSON格式的RawLogOptions）
	SettingKeyDisableAllLogs        = "disable_all_logs"         // 是否完全关闭所有日志记录

	// 模型健康检测相关设置
	SettingKeyHealthCheckEnabled               = "health_check_enabled"                 // 健康检测总开关
	SettingKeyHealthCheckInterval              = "health_check_interval"                // 健康检测间隔（分钟）
	SettingKeyHealthCheckFailureThreshold      = "health_check_failure_threshold"       // 失败次数阈值（超过此值自动禁用）
	SettingKeyHealthCheckFailureDisableEnabled = "health_check_failure_disable_enabled" // 是否启用失败自动禁用功能
	SettingKeyHealthCheckAutoEnable            = "health_check_auto_enable"             // 检测成功后是否自动启用
	SettingKeyHealthCheckLogRetentionCount     = "health_check_log_retention_count"     // 健康检测日志保留条数，0表示不限制
	SettingKeyHealthCheckCountAsSuccess        = "health_check_count_as_success"        // 健康检测成功是否计入成功调用
	SettingKeyHealthCheckCountAsFailure        = "health_check_count_as_failure"        // 健康检测失败是否计入失败调用（触发衰减）

	// 性能优化相关设置
	SettingKeyDisablePerformanceTracking = "disable_performance_tracking" // 关闭性能追踪（首包时间、TPS）
	SettingKeyDisableTokenCounting       = "disable_token_counting"       // 关闭 token 统计
	SettingKeyEnableRequestTrace         = "enable_request_trace"         // 启用请求追踪（httptrace）
	SettingKeyStripResponseHeaders       = "strip_response_headers"       // 移除不必要的响应头
	SettingKeyEnableFormatConversion     = "enable_format_conversion"     // 允许格式转换（关闭则只能直连）
)

// HealthCheckLog 模型健康检测日志
type HealthCheckLog struct {
	gorm.Model
	BatchID         string    `gorm:"index" json:"batch_id,omitempty"`    // 批次ID，用于批量检测追踪
	ModelProviderID uint      `gorm:"index" json:"model_provider_id"`     // 关联的 ModelWithProvider ID
	ModelName       string    `gorm:"index" json:"model_name"`            // 模型名称
	ProviderName    string    `gorm:"index" json:"provider_name"`         // 提供商名称
	ProviderModel   string    `json:"provider_model"`                     // 提供商模型名称
	Status          string    `gorm:"index" json:"status"`                // 检测状态: success, error
	Error           string    `json:"error,omitempty"`                    // 错误信息
	ResponseTime    int64     `json:"response_time"`                      // 响应时间（毫秒）
	CheckedAt       time.Time `gorm:"index" json:"checked_at"`            // 检测时间
}
