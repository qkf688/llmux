package models

import (
	"encoding/json"
	"net/http"
	"time"

	"gorm.io/gorm"
)

type Provider struct {
	gorm.Model
	Name          string
	Type          string
	Config        string
	Console       string // 控制台地址
	Proxy         string // 代理地址
	ModelEndpoint *bool  // 是否启用获取模型列表能力（用于对外“模型端点”与自动同步筛选），默认true
	// SQL NULL 契约：该字段无 gorm default tag，历史行落库为 NULL。
	// 语义上 NULL 视为 true（启用）。全仓所有查询必须经 repository.WhereModelEndpointEnabled /
	// WhereModelEndpointMatches，写法 `model_endpoint IS NULL OR model_endpoint = ?`，
	// 禁止直接 `model_endpoint = ?`（会漏掉 NULL 行）。新增依赖此字段的查询请勿另起 inline 写法。
	ModelFilterEnabled *bool   // 是否启用模型过滤（按规则过滤上游模型）
	AuthType           *string // 认证方式：x-api-key（默认）或 bearer，仅用于 Anthropic 类型
	Blacklisted        *bool   `gorm:"default:false" json:"blacklisted"` // 是否拉黑（拉黑后不参与自动关联）
}

type AnthropicConfig struct {
	BaseUrl string `json:"base_url"`
	ApiKey  string `json:"api_key"`
	Version string `json:"version"`
}

type Model struct {
	ID            uint `gorm:"primarykey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Name          string
	Remark        string
	MaxRetry      int   // 重试次数限制
	TimeOut       int   // 超时时间 单位秒
	IOLog         *bool // 是否记录IO
	IsUpstream    *bool // 是否是上游模型（true=上游，false=自定义）
	AutoAssociate *bool `gorm:"default:true" json:"auto_associate"` // 是否允许自动关联触发
	// SupportsThinking 是否支持 thinking（推理）能力。
	// 用 bool 而非 *bool：Model 层是能力真实值的 single source of truth，无继承对象，
	// nil 与 false 业务上等价；三态语义（nil=继承）只属于 ModelWithProvider 的 override 字段。
	// 与 IOLog/IsUpstream 的 *bool 风格不一致，是有意为之：Model 层不存在"未设置"状态。
	SupportsThinking bool `json:"supports_thinking"` // 是否支持 thinking（默认 false，存量/同步模型需手动开启）
	// ThinkingLevels 思考档位白名单（允许的 reasoning_effort 取值）。
	// 空切片=显式不约束（任意档位透传）；非空=只允许白名单内档位，超出按就近钳制规则收敛。
	// SupportsThinking=false 时此字段被忽略（thinking 整体剥离）。
	// 6 档有序 [minimal, low, medium, high, xhigh, max] + 2 特殊 [none, auto]；none/auto 不参与就近钳制。
	ThinkingLevels []string `json:"thinking_levels,omitempty" gorm:"serializer:json"`
}

type ModelWithProvider struct {
	gorm.Model
	ModelID             uint
	ProviderModel       string
	ProviderID          uint
	ToolCall            *bool             // 能否接受带有工具调用的请求
	StructuredOutput    *bool             // 能否接受带有结构化输出的请求
	Image               *bool             // 能否接受带有图片的请求(视觉)
	WithHeader          *bool             // 是否透传header
	Status              *bool             // 是否启用
	CustomerHeaders     map[string]string `gorm:"serializer:json"` // 自定义headers
	Weight              int
	Priority            int  // 优先级，值越高越优先选择
	MaxTokens           *int // max_tokens 上限，nil=不限；超过则裁剪到此值，避免客户端发超大值触发上游 400
	ConsecutiveFailures int  // 连续调用失败次数
	// SupportsThinking 是否支持 thinking（推理）能力，三态：nil=继承 Model.SupportsThinking，
	// true/false=override。与 Model.SupportsThinking 的 bool（二态）不同：关联层存在"继承"这一
	// 有效状态，因此必须用 *bool。解析语义见 SupportsThinkingResolved。
	SupportsThinking *bool
	// ThinkingLevels 思考档位 override，三态：
	// nil=继承 Model.ThinkingLevels；空切片=显式不约束（任意档位透传）；非空=override 白名单。
	// 用 *[]string 而非 []string 以区分"继承"与"显式不约束"。解析语义见 ThinkingLevelsResolved。
	// json tag 显式声明为 PascalCase 且**禁止 omitempty**：范式与上方三态兄弟字段
	// SupportsThinking 对齐——后者无 tag，序列化为 PascalCase 键 + null 表继承，前端一直正常消费。
	// 本字段曾被单独加 `thinking_levels,omitempty` 成为异类，导致继承态整键消失，
	// 前端读不到键便把"继承"误判为 override，并在保存时回写成"显式不约束"，静默破坏钳制规则。
	ThinkingLevels *[]string `json:"ThinkingLevels" gorm:"serializer:json"`
}

// SupportsThinkingResolved 解析该关联最终是否支持 thinking：
// override 非 nil 时以 override 为准；否则继承 model 的 SupportsThinking；
// model 为 nil 或 model.SupportsThinking 为 false 时返回 false。
// 纯函数（只读字段，无 IO），调用侧允许传 nil model，方法内部兜底。
func (m *ModelWithProvider) SupportsThinkingResolved(model *Model) bool {
	if m != nil && m.SupportsThinking != nil {
		return *m.SupportsThinking
	}
	if model != nil {
		return model.SupportsThinking
	}
	return false
}

// ThinkingLevelsResolved 解析该关联最终生效的思考档位白名单：
// override（ThinkingLevels）非 nil 时以 override 为准（含空切片=显式不约束）；
// 否则继承 model 的 ThinkingLevels（可能为空切片=不约束）。
// 纯函数（只读字段，无 IO），调用侧允许传 nil mwp / nil model，方法内部兜底返回 nil。
// 返回 nil 表示"不约束"（白名单空），非 nil 表示有效白名单。
// 注意：返回值 nil 与空切片语义不同——nil=继承且 model 也无白名单（按 unknown_strategy 处理），
// 空切片=显式不约束（任意档位透传，不进 ClampReasoningEffort 的 levels 分支）。
// 但 ClampReasoningEffort 的 levels 参数约定：nil 与空切片都视为"白名单空"，统一走 unknown_strategy。
// 因此本方法返回 nil 或空切片对 ClampReasoningEffort 等价，调用方无需区分。
func (m *ModelWithProvider) ThinkingLevelsResolved(model *Model) []string {
	if m != nil && m.ThinkingLevels != nil {
		return *m.ThinkingLevels
	}
	if model != nil {
		return model.ThinkingLevels
	}
	return nil
}

// SerializeThinkingLevelsForUpdate 将 []string 序列化为 GORM map-based UpdateFields 可接受的值。
// GORM serializer:json 只对 struct Updates 生效，map-based UpdateFields 不走 serializer，
// 需手动转 JSON 字符串。用于 Model.ThinkingLevels（[]string，非三态）。
// nil → JSON "null" 写入（GORM 会存 NULL）；非 nil → JSON 数组字符串。
func SerializeThinkingLevelsForUpdate(levels []string) (any, error) {
	data, err := json.Marshal(levels)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// SerializeThinkingLevelsPtrForUpdate 将 *[]string 序列化为 GORM map-based UpdateFields 可接受的值。
// 用于 ModelWithProvider.ThinkingLevels（*[]string，三态）。
// nil → nil（写 NULL=继承）；非 nil → JSON 数组字符串（空切片="[]"=显式不约束）。
func SerializeThinkingLevelsPtrForUpdate(levels *[]string) (any, error) {
	if levels == nil {
		return nil, nil
	}
	data, err := json.Marshal(*levels)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// thinkingEffortOrder 6 档思考档位的有序索引（从低到高）。
// none/auto 是特殊档位，不参与就近钳制，单独处理。
var thinkingEffortOrder = map[string]int{
	"minimal": 0,
	"low":     1,
	"medium":  2,
	"high":    3,
	"xhigh":   4,
	"max":     5,
}

// thinkingEffortLevels 6 档有序切片（从低到高），用于就近钳制时取最低档/遍历。
var thinkingEffortLevels = []string{"minimal", "low", "medium", "high", "xhigh", "max"}

// IsSixLevelEffort 判断 effort 是否为 6 档有序档位之一（不含 none/auto）。
// 用于校验 autoFallback 等设置值是否合法，防止数据库脏值传入 ClampReasoningEffort。
func IsSixLevelEffort(effort string) bool {
	_, ok := thinkingEffortOrder[effort]
	return ok
}

// ClampReasoningEffort 将 reasoning effort 按模型白名单就近钳制（平手偏低）。
// 纯函数：不读设置、不记日志，所有外部输入经参数传入，便于单测。
//
// 参数：
//   - effort: 客户端原始档位（已归一化小写，含 none/auto/未知值）
//   - levels: 模型白名单（ThinkingLevelsResolved 结果）。nil 或空切片=白名单空
//   - autoFallback: auto 不支持且白名单空时的兜底档位（调用方从 ctx 读 SettingKeyReasoningEffortDefaultValue 传入）
//   - unknownStrategy: 白名单空时未知档位的处理策略（"clamp_to_default" 或 "passthrough"，调用方从 ctx 读 SettingKeyReasoningEffortUnknownStrategy 传入）
//
// 返回钳制后的档位；返回空串表示"剥离 thinking"（调用方删 effort 字段 / 设 unified.ReasoningEffort=nil）。
//
// 规则：
//   - effort 为空串：原样返回空串（调用方本就不该 emit）
//   - levels 空（nil/空切片）：
//   - unknownStrategy=clamp_to_default → 未知档位兜底到 autoFallback；已知 6 档（minimal/low/medium/high/xhigh/max）始终透传
//   - unknownStrategy=passthrough → 原样透传
//   - levels 非空：
//   - effort 在白名单内 → 透传
//   - none：白名单含 none → 透传；不含 → 返回空串（剥离 thinking）
//   - auto：白名单含 auto → 透传；不含 → 取白名单中最低的 6 档档位（auto 语义=让模型自定，无 auto 能力时最低档最保守）
//   - 其余 6 档：就近钳制（平手偏低）——找白名单中 <= effort 档位的最高档；若无更低档则取白名单最低档
func ClampReasoningEffort(effort string, levels []string, autoFallback string, unknownStrategy string) string {
	if effort == "" {
		return ""
	}

	levelsEmpty := len(levels) == 0

	// 白名单空：走 unknown_strategy
	if levelsEmpty {
		if unknownStrategy == "passthrough" {
			return effort
		}
		// clamp_to_default：已知 6 档透传，未知档位兜底 autoFallback
		if _, isKnown := thinkingEffortOrder[effort]; isKnown {
			return effort
		}
		// none/auto 是特殊档位，clamp_to_default 下也兜底（none 不在 6 档，auto 不在 6 档）
		if autoFallback != "" {
			return autoFallback
		}
		return "low"
	}

	// 白名单非空：先查精确命中
	levelSet := make(map[string]bool, len(levels))
	for _, l := range levels {
		levelSet[l] = true
	}
	if levelSet[effort] {
		return effort
	}

	// none 特殊处理
	if effort == "none" {
		if levelSet["none"] {
			return "none"
		}
		return "" // 剥离 thinking
	}

	// auto 特殊处理：白名单不含 auto → 取白名单中最低的 6 档档位
	if effort == "auto" {
		if levelSet["auto"] {
			return "auto"
		}
		if lowest := lowestLevelInWhitelist(levelSet); lowest != "" {
			return lowest
		}
		// 白名单只含 none/auto 这类特殊档位，无 6 档 → 兜底 autoFallback
		if autoFallback != "" {
			return autoFallback
		}
		return "low"
	}

	// 6 档就近钳制（平手偏低）
	effortRank, isSixLevel := thinkingEffortOrder[effort]
	if !isSixLevel {
		// 未知档位 + 白名单非空：兜底到白名单最低档（保守）
		if lowest := lowestLevelInWhitelist(levelSet); lowest != "" {
			return lowest
		}
		if autoFallback != "" {
			return autoFallback
		}
		return "low"
	}

	// 找白名单中 <= effort 的最高档（平手偏低）
	best := ""
	bestRank := -1
	for _, l := range thinkingEffortLevels {
		if !levelSet[l] {
			continue
		}
		r := thinkingEffortOrder[l]
		if r <= effortRank && r > bestRank {
			best = l
			bestRank = r
		}
	}
	if best != "" {
		return best
	}
	// 无更低档：取白名单最低档
	if lowest := lowestLevelInWhitelist(levelSet); lowest != "" {
		return lowest
	}
	// 白名单只含 none/auto：兜底 autoFallback
	if autoFallback != "" {
		return autoFallback
	}
	return "low"
}

// lowestLevelInWhitelist 返回白名单中最低的 6 档档位；白名单无 6 档时返回空串。
func lowestLevelInWhitelist(levelSet map[string]bool) string {
	for _, l := range thinkingEffortLevels {
		if levelSet[l] {
			return l
		}
	}
	return ""
}

// HighestEffortInWhitelist 返回白名单中最高的 6 档档位；白名单为空或只含 none/auto 等非 6 档时返回空串。
// 纯函数，与 lowestLevelInWhitelist 对偶，但对外导出：供 budget-only 请求
// （客户端只给 reasoning budget、不给 effort）求 budget 上限——白名单最高档决定该模型允许的
// 最大思考预算；返回空串表示白名单不允许任何正向思考档位（调用方应剥离 thinking）。
//
// 为什么不复用 ClampReasoningEffort：那需要先把 budget 反推成 effort，而反推有损
// （513..19999 全塌到 low，正推只有 1000），会把白名单本已允许的中等 budget 过度降级。
// budget 数值映射不放在本包（models 禁止依赖 service），调用方拿档位后自行换算。
func HighestEffortInWhitelist(levels []string) string {
	levelSet := make(map[string]bool, len(levels))
	for _, l := range levels {
		levelSet[l] = true
	}
	for i := len(thinkingEffortLevels) - 1; i >= 0; i-- {
		if levelSet[thinkingEffortLevels[i]] {
			return thinkingEffortLevels[i]
		}
	}
	return ""
}

// ModelTemplateItem 模型模板条目：用于将 provider_model 映射到 ModelID（区分大小写、去重）
type ModelTemplateItem struct {
	gorm.Model
	ModelID uint   `gorm:"uniqueIndex:idx_model_template_item;not null"`
	Name    string `gorm:"uniqueIndex:idx_model_template_item;not null"`
}

// VirtualModel 虚拟模型：将多个真实模型组合成一个虚拟模型
type VirtualModel struct {
	gorm.Model
	Name        string `gorm:"type:varchar(255);uniqueIndex;not null"` // 虚拟模型名称
	Description string `gorm:"type:text"`                              // 描述
	Strategy    string `gorm:"type:varchar(50);default:'priority'"`    // 路由策略: priority, round_robin, random
	MaxRetry    int    `gorm:"default:10"`                             // 重试次数限制
	TimeOut     int    `gorm:"default:60"`                             // 超时时间 单位秒
	IOLog       *bool  `gorm:"default:false"`                          // 是否记录IO
	Enabled     *bool  `gorm:"default:true"`                           // 是否启用
}

// VirtualModelMapping 虚拟模型关联：虚拟模型与真实模型的映射关系
type VirtualModelMapping struct {
	gorm.Model
	VirtualModelID uint  `gorm:"uniqueIndex:idx_virtual_model_mapping;not null"` // 虚拟模型ID
	RealModelID    uint  `gorm:"uniqueIndex:idx_virtual_model_mapping;not null"` // 真实模型ID
	Priority       int   `gorm:"default:10"`                                     // 优先级，值越高越优先选择
	Weight         int   `gorm:"default:5"`                                      // 权重，用于同优先级的随机选择
	Enabled        *bool `gorm:"default:true"`                                   // 是否启用
}

type ChatLog struct {
	gorm.Model
	Name string `gorm:"index"`
	// RealModelName is the actual system model name that was hit.
	// For direct requests it equals Name; for virtual model routing it is the selected real model.
	RealModelName string `gorm:"index"`
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

	// UsageSource 标记本条 usage 的来源，用于排查「token 全为 0」是上游没给
	// 还是归集链路丢了。取值见 UsageSource* 常量。
	//
	// 放在 ChatLog 而不是内嵌的 Usage 上：Usage 会被序列化进对外响应，
	// 混入非 token 字段会污染协议形状。
	UsageSource string `gorm:"index"`

	// 原始请求和响应内容
	RequestHeaders  string // 请求头JSON字符串
	RequestBody     string // 请求体（转换后，发给上游的）
	RawRequestBody  string // 原始请求体（转换前，客户端发来的）
	ResponseHeaders string // 响应头JSON字符串
	ResponseBody    string // 响应体（转换后）
	RawResponseBody string // 原始响应体（转换前）
}

// ChatLog.UsageSource 的取值。
const (
	// UsageSourceUpstream：转换层旁路交出的上游原始 usage，最可信。
	UsageSourceUpstream = "upstream"
	// UsageSourcePassthrough：无协议转换（style == provider type），
	// usage 由 processer 直接解析上游响应得到，同样未经转换失真。
	UsageSourcePassthrough = "passthrough"
	// UsageSourceDownstream：走了协议转换但旁路没拿到 usage，退化为从
	// 转换后的下游流反解——可能因目标协议缺字段而失真，需要关注。
	UsageSourceDownstream = "downstream"
	// UsageSourceMissing：所有路径都没拿到有效 token 数。
	UsageSourceMissing = "missing"
)

func (l ChatLog) WithError(err error) ChatLog {
	l.Error = err.Error()
	l.Status = "error"
	return l
}

type Usage struct {
	PromptTokens            int64                   `json:"prompt_tokens"`
	CompletionTokens        int64                   `json:"completion_tokens"`
	TotalTokens             int64                   `json:"total_tokens"`
	PromptTokensDetails     PromptTokensDetails     `json:"prompt_tokens_details" gorm:"serializer:json"`
	CompletionTokensDetails CompletionTokensDetails `json:"completion_tokens_details" gorm:"serializer:json"`
}

// HasTokens 表示这份 usage 是否包含有效的 token 计数。
//
// 用来区分「上游明确报告了用量」与「拿到一个空壳 usage 对象」——各协议适配器
// 只要在回包里看到 usage 字段就会产出非 nil 的 Usage，字段可能全是 0，或键名
// 根本没被候选路径识别。侧信道的写入门禁与落库侧的 UsageSource 判定必须用
// 同一个谓词，否则会出现「侧信道丢弃了快照、落库侧却认为有值」的口径错配。
//
// 只看 Total / Prompt / Completion，不看 details：details 是明细而非计数总量，
// 且 total 的回退口径本身就不含缓存 token（见 ResolveTotalTokens）。若只有
// CachedTokens 非零而三个总量全为 0，即便保留该快照 TotalTokens 仍是 0，
// 计费拿不到任何东西，反而会被标成 upstream 而压掉 missing 告警。
func (u Usage) HasTokens() bool {
	return u.TotalTokens > 0 || u.PromptTokens > 0 || u.CompletionTokens > 0
}

// StatsTotal 系统统计（全量累计）
// 注意：该表用于仪表盘统计，不依赖 chat_logs，避免清空日志影响仪表盘。
type StatsTotal struct {
	ID        uint      `gorm:"primaryKey"`
	Reqs      int64     `json:"reqs"`
	Tokens    int64     `json:"tokens"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StatsDaily 系统统计（按天累计，日期格式：2006-01-02）
type StatsDaily struct {
	Date      string    `gorm:"primaryKey;type:varchar(10)" json:"date"`
	Reqs      int64     `json:"reqs"`
	Tokens    int64     `json:"tokens"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StatsHourly 系统统计（按小时累计，日期格式：2006-01-02，小时范围：0-23）
type StatsHourly struct {
	Date      string    `gorm:"primaryKey;type:varchar(10)" json:"date"`
	Hour      int       `gorm:"primaryKey" json:"hour"`
	Reqs      int64     `json:"reqs"`
	Tokens    int64     `json:"tokens"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StatsModelTotal 模型调用统计（全量累计）
type StatsModelTotal struct {
	Name      string    `gorm:"primaryKey;type:varchar(255)" json:"name"`
	Calls     int64     `json:"calls"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StatsRealModelTotal 真实命中模型调用统计（全量累计）
// 口径：按 ChatLog.RealModelName（为空时由写入侧回退到 ChatLog.Name）累计。
type StatsRealModelTotal struct {
	Name      string    `gorm:"primaryKey;type:varchar(255)" json:"name"`
	Calls     int64     `json:"calls"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StatsProviderTotal 供应商调用统计（全量累计）
type StatsProviderTotal struct {
	ProviderName        string    `gorm:"primaryKey;type:varchar(255)" json:"provider_name"`
	TotalRequests       int64     `json:"total_requests"`
	SuccessCount        int64     `json:"success_count"`
	FailureCount        int64     `json:"failure_count"`
	TotalTokens         int64     `json:"total_tokens"`
	AvgResponseTime     int64     `json:"avg_response_time"`     // 累计响应时间（用于计算平均值）
	ResponseTimeSamples int64     `json:"response_time_samples"` // 有响应时间的样本数（仅成功且有耗时时自增），读侧用它做分母
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type PromptTokensDetails struct {
	CachedTokens int64 `json:"cached_tokens"`
	AudioTokens  int64 `json:"audio_tokens"`
}

type CompletionTokensDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens"`
	AudioTokens     int64 `json:"audio_tokens"`
}

type ChatIO struct {
	gorm.Model
	LogID uint
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
	RequestBody     bool `json:"request_body"`      // 记录请求体（转换后）
	RawRequestBody  bool `json:"raw_request_body"`  // 记录原始请求体（转换前）
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

	SettingKeyAutoPriorityDecay                = "auto_priority_decay"                 // 自动优先级衰减开关
	SettingKeyAutoPriorityDecayDefault         = "auto_priority_decay_default"         // 自动优先级衰减默认优先级
	SettingKeyAutoPriorityDecayStep            = "auto_priority_decay_step"            // 自动优先级衰减步长（每次失败减少的优先级）
	SettingKeyAutoPriorityDecayThreshold       = "auto_priority_decay_threshold"       // 自动优先级衰减阈值（达到此值自动禁用）
	SettingKeyAutoPriorityDecayDisableEnabled  = "auto_priority_decay_disable_enabled" // 是否启用自动禁用功能（达到阈值时禁用）
	SettingKeyAutoPriorityIncreaseStep         = "auto_priority_increase_step"         // 自动优先级增加步长（每次成功增加的优先级）
	SettingKeyAutoPriorityIncreaseMax          = "auto_priority_increase_max"          // 自动优先级增加的上限
	SettingKeyAutoSuccessIncrease              = "auto_success_increase"               // 成功调用后是否执行自增
	SettingKeyConsecutiveFailureThreshold      = "consecutive_failure_threshold"       // 连续失败次数阈值（达到阈值自动禁用）
	SettingKeyConsecutiveFailureDisableEnabled = "consecutive_failure_disable_enabled" // 是否启用连续失败自动禁用

	SettingKeyLogRetentionCount               = "log_retention_count"                  // 日志保留条数，0表示不限制
	SettingKeyLogRawRequestResponse           = "log_raw_request_response"             // 原始请求响应记录选项（JSON格式的RawLogOptions）
	SettingKeyLogRawRequestResponseErrorsOnly = "log_raw_request_response_errors_only" // 是否仅保留错误日志的原始请求响应（成功调用会自动清空）
	SettingKeyDisableAllLogs                  = "disable_all_logs"                     // 是否完全关闭所有日志记录

	// 模型健康检测相关设置
	SettingKeyHealthCheckEnabled               = "health_check_enabled"                 // 健康检测总开关
	SettingKeyHealthCheckInterval              = "health_check_interval"                // 健康检测间隔（分钟）
	SettingKeyHealthCheckFailureThreshold      = "health_check_failure_threshold"       // 失败次数阈值（超过此值自动禁用）
	SettingKeyHealthCheckFailureDisableEnabled = "health_check_failure_disable_enabled" // 是否启用失败自动禁用功能
	SettingKeyHealthCheckAutoEnable            = "health_check_auto_enable"             // 检测成功后是否自动启用
	SettingKeyHealthCheckLogRetentionCount     = "health_check_log_retention_count"     // 健康检测日志保留条数，0表示不限制
	SettingKeyHealthCheckCountAsSuccess        = "health_check_count_as_success"        // 健康检测成功是否计入成功调用
	SettingKeyHealthCheckCountAsFailure        = "health_check_count_as_failure"        // 健康检测失败是否计入失败调用（触发衰减）
	SettingKeyHealthCheckCheckDisabledOnly     = "health_check_check_disabled_only"     // 是否只检测停用的模型

	// 性能优化相关设置
	SettingKeyDisablePerformanceTracking = "disable_performance_tracking" // 关闭性能追踪（首包时间、TPS）
	SettingKeyDisableTokenCounting       = "disable_token_counting"       // 关闭 token 统计
	SettingKeyEnableRequestTrace         = "enable_request_trace"         // 启用请求追踪（httptrace）
	SettingKeyStripResponseHeaders       = "strip_response_headers"       // 移除不必要的响应头
	SettingKeyEnableFormatConversion     = "enable_format_conversion"     // 允许格式转换（关闭则只能直连）

	// 模型同步相关设置
	SettingKeyModelSyncEnabled           = "model_sync_enabled"             // 自动同步模型开关
	SettingKeyModelSyncInterval          = "model_sync_interval"            // 同步间隔（小时）
	SettingKeyModelSyncLogRetentionCount = "model_sync_log_retention_count" // 同步日志保留条数
	SettingKeyModelSyncLogRetentionDays  = "model_sync_log_retention_days"  // 同步日志保留天数
	SettingKeyModelSyncFilterRules       = "model_sync_filter_rules"        // 模型同步过滤规则（JSON 数组）

	// 模板模糊匹配相关设置
	SettingKeyTemplateFuzzyMatchEnabled    = "template_fuzzy_match_enabled"    // 模板模糊匹配开关
	SettingKeyTemplateFuzzyMatchSeparators = "template_fuzzy_match_separators" // 模糊匹配分隔符（JSON 数组）
	SettingKeyTemplateFuzzyMatchSuffixes   = "template_fuzzy_match_suffixes"   // 模糊匹配后缀关键词（JSON 数组）

	// 模型关联相关设置
	SettingKeyAutoAssociateOnAdd          = "auto_associate_on_add"           // 添加模型时自动关联
	SettingKeyAutoCleanOnDelete           = "auto_clean_on_delete"            // 删除模型时自动清理关联
	SettingKeyAutoSaveTemplateOnAssociate = "auto_save_template_on_associate" // 关联模型时自动保存到模板

	// reasoning_effort 参数映射相关设置
	SettingKeyReasoningEffortMappingEnabled  = "reasoning_effort_mapping_enabled"  // 是否启用映射
	SettingKeyReasoningEffortDefaultValue    = "reasoning_effort_default_value"    // 默认值（minimal/low/medium/high/xhigh/max）
	SettingKeyReasoningEffortUnknownStrategy = "reasoning_effort_unknown_strategy" // 未知档位策略（clamp_to_default / passthrough）
)

// HealthCheckLog 模型健康检测日志
type HealthCheckLog struct {
	gorm.Model
	BatchID         string    `gorm:"index" json:"batch_id,omitempty"` // 批次ID，用于批量检测追踪
	ModelProviderID uint      `gorm:"index" json:"model_provider_id"`  // 关联的 ModelWithProvider ID
	ModelName       string    `gorm:"index" json:"model_name"`         // 模型名称
	ProviderName    string    `gorm:"index" json:"provider_name"`      // 提供商名称
	ProviderModel   string    `json:"provider_model"`                  // 提供商模型名称
	Status          string    `gorm:"index" json:"status"`             // 检测状态: success, error
	Error           string    `json:"error,omitempty"`                 // 错误信息
	ResponseTime    int64     `json:"response_time"`                   // 响应时间（毫秒）
	CheckedAt       time.Time `gorm:"index" json:"checked_at"`         // 检测时间
}

// ModelSyncLog 模型同步日志
type ModelSyncLog struct {
	gorm.Model
	BatchID       *string   `gorm:"index" json:"BatchID,omitempty"` // 批次ID
	ProviderID    uint      `gorm:"index" json:"ProviderID"`
	ProviderName  string    `gorm:"index" json:"ProviderName"`
	Status        string    `gorm:"index" json:"Status"` // 同步状态: success, error, unchanged
	Error         string    `json:"Error,omitempty"`     // 错误信息
	AddedCount    int       `json:"AddedCount"`
	RemovedCount  int       `json:"RemovedCount"`
	AddedModels   []string  `gorm:"serializer:json" json:"AddedModels"`
	RemovedModels []string  `gorm:"serializer:json" json:"RemovedModels"`
	SyncedAt      time.Time `gorm:"index" json:"SyncedAt"`
}
