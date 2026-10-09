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
	ModelFilterEnabled *bool // 是否启用模型过滤（按规则过滤上游模型）
	// AuthType 认证方式展示值：**由 Config 派生、不落库**（gorm:"-"，Stage 20 消灭双写）。
	// 运行时与展示的真值同源——config 的 auth_type（anthropic）/ openai_auth_type（openai 系），
	// 派生规则单源 providers.DeriveConfigAuthType，由读侧（详情/列表/导出）出库后填充。
	// 此前是 anthropic 专属列副本且与 config 双写无一致性约束，PUT 顶层 auth_type 可造出
	// 「列显 bearer、实际发 x-api-key」的漂移；现写侧入口已删，读侧恒为 config 真值。
	AuthType    *string `gorm:"-"`
	Blacklisted *bool   `gorm:"default:false" json:"blacklisted"` // 是否拉黑（拉黑后不参与自动关联）
	// Protocols 勾选的出站协议（设计定案第 2 节），取值见 consts.Protocol*。
	// 存量 Provider 由启动迁移按 type 派生填充（见 migrate_channels.go），
	// 新建 Provider 由前端勾选写入（S6）；nil = 未设置。
	Protocols []string `gorm:"serializer:json"`
}

// Model 模型目录主表：真实模型行（IsUpstream）与自定义模型行共用。
type Model struct {
	ID         uint `gorm:"primarykey"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Name       string
	Remark     string
	IOLog      *bool // 是否记录IO
	IsUpstream *bool // 是否是上游模型（true=上游，false=自定义）
	// AutoAssociate 二态开关（是否允许自动关联触发），与 SupportsThinking 同范式：
	// bool 而非 *bool——UI 与批量操作都只有勾选/不勾两态，「未设置」从来不是可表达的
	// 状态，且 nil 与 true 在 allowsAutoAssociate 里行为一致（都=允许），三态是假抽象。
	// Create 缺省口径由 handler 兜底为 true（与前端表单默认值一致），不靠 gorm default
	// tag——default tag 会把显式 false 也当零值吞掉，与 SupportsThinking 同理故不用。
	AutoAssociate bool `json:"auto_associate"`
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

	// 以下三项是模型固有能力真值（ToolCall 工具调用 / StructuredOutput 结构化输出 /
	// Image 视觉），与 SupportsThinking 同为 Model 层 single source of truth：
	// 二态 bool，无「未设置」——Model 层不存在继承对象，nil 与 false 业务等价；
	// 「继承」语义只存在于 ModelWithProvider 的对应 override 字段（nil=继承本字段）。
	//
	// 为什么能力真值必须同时留在关联层：能力在物理上是**链路**属性而非模型属性——
	// 同一模型经不同第三方中转时，视觉/工具支持度可能不一致（中转站可能直接丢图或报错）。
	// Model 层给真值、关联层按需 override，缺任一层都会丢表达能力：
	// 只留 Model 层 → 无法表达「某家中转不支持视觉」；
	// 只留关联层 → 每加一个供应商都要人肉重填三格，且无统一真值可继承。
	ToolCall         bool `json:"tool_call"`         // 是否支持工具调用
	StructuredOutput bool `json:"structured_output"` // 是否支持结构化输出
	Image            bool `json:"image"`             // 是否支持视觉（图片输入）
}

type ModelWithProvider struct {
	gorm.Model
	ModelID       uint
	ProviderModel string
	ProviderID    uint
	// ToolCall / StructuredOutput / Image 三项能力 override，三态语义（与下方
	// SupportsThinking 同范式）：
	//   nil = 继承 Model 的同名能力真值（绝大多数第三方与官方一致，新建关联即此态）；
	//   true / false = override（仅用于「该链路比模型弱」或「比模型强」的少数情形）。
	// 生效值由 repository 的 COALESCE(mwp.<col>, model.<col>) 在 SQL 层解析，
	// 供 service/chat 的能力筛选（见 queryEnabledModelProviders）使用。
	ToolCall            *bool             // 能否接受带有工具调用的请求；nil=继承 Model.ToolCall
	StructuredOutput    *bool             // 能否接受带有结构化输出的请求；nil=继承 Model.StructuredOutput
	Image               *bool             // 能否接受带有图片的请求(视觉)；nil=继承 Model.Image
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
	Status        string `gorm:"index"` // 状态机取值见 ChatLogStatus* 常量（processing 建行 / success 成功出口显式写 / error 与 canceled 失败出口，写法见 service/chat 的 failureLogStatus）
	// Stream 入站 stream 标记快照：true = 流式 / false = 非流式。
	// 建行时由 service/chat/chat_attempt 从 Before.Stream 拷贝（落点同 ApiKeyID 等快照字段）。
	// 用 bool 而非 *bool：不引入「未设置」三态语义。
	// **默认是流式**——落到两层：
	//   1. 存量行：上线前 chat_logs.stream IS NULL 行一次 UPDATE 已落 1（备份在 db/llmux.db.pre-stream-default-backfill.bak）。
	//   2. 新建行：service/chat/chat_attempt.go 始终显式从 Before.Stream 拷贝，DB 列无需 DEFAULT 约束。
	// 不加 gorm:"default:true"——GORM 的 default 标签在 bool 字段零值时覆盖显式 false（Go footgun），
	// 与「false 是合法值」语义冲突；现状靠代码侧显式赋值兜底即可。
	Stream    bool
	Style     string // 类型
	UserAgent string `gorm:"index"` // 用户代理
	RemoteIP  string // 访问ip
	ChatIO    bool   // 是否开启IO记录

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

	// 调度明细（S3-3 起写入）：一次选路的三层命中（service/channel Selection）。
	// 建行时填充（executeSingleProviderAttempt 的 logEntry）；失败更新走 struct
	// Updates 零值跳过语义，建行值自动贯穿全部失败出口。handler/logs 的 DTO 以
	// snake_case 键暴露，前端展示层（S0 原型）已按同名键消费。
	EndpointProtocol string // 命中的协议端点协议（openai/anthropic/responses）
	EndpointURL      string // 实际出站 URL（继承链解析后：端点 URL 或 Provider base_url）
	KeyGroupName     string // 命中的凭据分组名
	CredentialNote   string // 命中的凭据标识（Note 非空用 Note，否则 KeyHash 前 8 位）

	// 调用方密钥归因快照（多 api-key 方案 §4）：建行时从 ReqMeta 定格，
	// key 之后改名/删除不影响历史行——故无外键、查询不 JOIN api_keys，
	// ApiKeyName 即「当时名」。*uint 的 nil 表达「无归因」（内部路径/存量行），
	// 与 id=0 区分（主键恒 ≥1）。
	ApiKeyID   *uint
	ApiKeyName string

	// 请求编号快照（Stage 17）：建行时从 ReqMeta 定格带来源前缀的 rid
	// （自生成 gw: / 沿用 in:，见 middleware.RequestID）；失败出口是 Update 非
	// 建行，struct Updates 零值跳过保证快照不被空串覆盖。存量行为建行前旧口径，
	// 空串 = 无编号，DTO 键恒在（同 ApiKeyID 的「键在值空」语义）。
	RequestID string

	// 原始请求和响应内容
	RequestHeaders  string // 请求头JSON字符串
	RequestBody     string // 请求体（转换后，发给上游的）
	RawRequestBody  string // 原始请求体（转换前，客户端发来的）
	ResponseHeaders string // 响应头JSON字符串
	ResponseBody    string // 响应体（转换后）
	RawResponseBody string // 原始响应体（转换前）
}

// ChatLog.Status 的取值（单源：建行值、成功/失败出口、raw 清理判据、关联面板的
// 请求成败判定共用一个口径，禁散落字面量）。状态数已从两态扩到多态，各处按字面量
// 各自对齐必然漏改——`maybeClearRawOnSuccess` 与「最近 10 次请求成败」两处读点就
// 曾用过方向相反的判据。
const (
	// ChatLogStatusProcessing：处理中——行已建、上游请求已发出、尚未收尾。
	// 建行值；终态由成功出口（RecordLog 显式写 ChatLogStatusSuccess）与各失败出口落库
	// （error 与 canceled 的分工见 ChatLogStatusCanceled）。
	ChatLogStatusProcessing = "processing"
	// ChatLogStatusSuccess：已完成。
	ChatLogStatusSuccess = "success"
	// ChatLogStatusError：失败（上游故障 / 协议错误 / 体积闸 / 读体错误等）。
	ChatLogStatusError = "error"
	// ChatLogStatusCanceled：已取消——调用方主动撤走（客户端断开 / 点停止）。
	// 判据唯一：失败原因链上带 context.Canceled，由 service/chat 的 failureLogStatus
	// 单点判定（attempt 层的 buildAttemptFailureUpdate 与 RecordLog 的 processer 错误
	// 分支共用）。刻意不扩到 clientSideFailure 全族：写回被写上限掐断（Stage 39）与
	// 坏管道是「客户端不收结果」，供应商已把响应完整产出，不算「主动取消」。
	ChatLogStatusCanceled = "canceled"
)

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
	l.Status = ChatLogStatusError
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

// StatsRealModelTotal 真实命中模型调用统计（全量累计）
// 口径：按 ChatLog.RealModelName（为空时由写入侧回退到 ChatLog.Name）累计。
//
// Calls 与 token 三列的写入点不同，这是 usage 的时序决定的，不是随意分工：
// Calls 在建行时刻（SaveChatLog）就确定，那一刻只有模型名；token 要等上游响应
// 结束、转换层归集出 usage 才拿得到，只能在结算点（chat.RecordLog）累加。
// 两条路径用同一个模型名口径（RealModelName 空则回落 Name），故两套数字可对账。
type StatsRealModelTotal struct {
	Name  string `gorm:"primaryKey;type:varchar(255)" json:"name"`
	Calls int64  `json:"calls"`
	// Token 三列与 Calls 的历史起点不同：2026-09-09 之前 chat_logs 已被保留策略
	// 清空，那些调用的 token 无法回填，故早于该日期的模型此三列恒为 0。
	// 读侧（仪表盘）须把 0 显示为「未知」而非「零消耗」。
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalTokens      int64     `json:"total_tokens"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
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
	// ReasoningTokensKnown 标记上游是否明确报告了 reasoning 拆分。
	//
	// true  = 候选路径键存在（含明确报告 0——非推理模型报 0 与「没报」语义不同）；
	// false = 上游没报拆分，「推理为 0」应展示为「未知」而不是 0。
	//
	// 链路：归一入口 models.UsageFromMap 与三处手工作法点
	// （service/transform/openai、service/responses、service/transform/streaming
	// usageFromResponses）必须同步维护，新增一处解析即需在此登记。
	ReasoningTokensKnown bool  `json:"reasoning_tokens_known"`
	AudioTokens          int64 `json:"audio_tokens"`
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
	// 调用方密钥归因快照（ChatLog.ApiKeyID/ApiKeyName 的传值来源）：
	// handler/v1 从 middleware.CallerKey 翻译至此——models 不 import middleware，
	// 传递链只经本结构（auth→gin context→ReqMeta→建行点）。
	// #6（key 级出站头）落地时在此加 Headers 字段，读方不动（OCP）。
	ApiKeyID   *uint
	ApiKeyName string
	// RequestID 是带来源前缀的请求编号快照（Stage 17，ChatLog.RequestID 的传值
	// 来源）：handler/v1 从 gin context（consts.ContextKeyRequestID）翻译至此，
	// 值形如 gw:xxxx（网关自生成）/ in:xxxx（沿用调用方），见 middleware.RequestID。
	RequestID string
	// Headers 是 key 级自定义出站头快照（#6，方案 §2 切片 B）：整包透传至
	// chat_attempt 的唯一装配点 chatcore.BuildHeaders，链上零读方改动。
	Headers map[string]string
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
	// SettingKeyRawLogFieldMaxKiB 原始采集文本字段的单字段上限（KiB，Stage 2-D）：
	// 一个设置项统一盖 RequestBody/RawRequestBody/RawResponseBody/Error 四字段，
	// 挡原始日志把 SQLite 撑大（体积闸管内存瞬时峰值，本上限管库，抓手不同）。
	SettingKeyRawLogFieldMaxKiB = "raw_log_field_max_kib" // 原始采集字段单字段文本上限（KiB）
	// SettingKeyResponseLogMaxKiB 回复类日志的单条上限（KiB，Stage 23）：盖非流式
	// ResponseBody 与流式 ChatIO 累积两路（此前游离于 raw_log_field_max_kib 之外，
	// 单条可达 MiB×并发流数）。这是「看全文」口径——默认 2MB 基本等于完整原文，
	// 超限存头尾两段；与「采样排障」的四字段采集上限是两个抓手，故独立设键。
	SettingKeyResponseLogMaxKiB = "response_log_max_kib"
	SettingKeyDisableAllLogs    = "disable_all_logs" // 是否完全关闭所有日志记录

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

	// 格式转换是路由能力开关，不是性能取舍（关闭则只能直连）
	SettingKeyEnableFormatConversion = "enable_format_conversion"

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

	// 请求参数相关设置（全局超时/重试，替代原 per-model time_out/max_retry）
	// 两个「等待上游」的窗口语义不同源（Stage 38）：流式请求等的是上游响应头（真实等头，
	// 通常 1~2s 就到）；非流式请求上游要生成完整回答才发响应头，同一个窗口等价于
	// 「整段回答必须在 N 秒内写完」，故拆成两个键——同一字段一刀切属口径混用。
	SettingKeyRequestHeaderTimeout     = "request_header_timeout"      // 流式请求单次尝试等待响应头的超时（秒）
	SettingKeyNonStreamResponseTimeout = "non_stream_response_timeout" // 非流式请求单次尝试等待完整回答的窗口（秒，等价整段生成预算）
	SettingKeyRequestTotalTimeout      = "request_total_timeout"       // 整个请求的总时长上限（秒，硬上限：到点含流式输出一刀切）
	SettingKeyStreamFirstByteTimeout   = "stream_first_byte_timeout"   // 流式响应头到达后等待首个数据字节的超时（秒）
	SettingKeyRequestMaxRetry          = "request_max_retry"           // 单候选池最大尝试次数
	// SettingKeyUpstreamMaxResponseMiB 上游单条**非流式**响应体的体积上限（MiB，Stage 2 资源防御）。
	// 合法大响应主场景是文生图 base64（单张高清图编码后 4~8MB），默认 32 = 最大合法场景 ×4 余量；
	// 病态/被劫持上游的超大 body 在响应头提交给客户端之前即被拦截并故障转移。下限 1 MiB（不可关闭）。
	SettingKeyUpstreamMaxResponseMiB = "upstream_max_response_mib"

	// SettingKeyCacheAffinityEnabled 缓存亲和（粘性选路）开关：同一对话身份钉住同一
	// 真实模型×关联×分组×凭据，提高供应商提示词缓存命中、降低输入成本。钉定失效无害
	// 回退现状选择（编排见 service/chat/chat_affinity.go），故默认开。
	SettingKeyCacheAffinityEnabled = "cache_affinity_enabled"

	// 凭据健康相关设置
	SettingKeyCredHealthCooldown429Sec    = "cred_health_cooldown_429_sec"    // 凭据 429 限流冷却窗口（秒）
	SettingKeyCredHealthCooldownServerSec = "cred_health_cooldown_server_sec" // 凭据服务端错误冷却窗口（秒，5xx/超时/网络）
	SettingKeyCredHealthAuthFailThreshold = "cred_health_auth_fail_threshold" // 凭据连续鉴权失败（401/403）判停阈值（次）
	SettingKeyCredHealthProbeIntervalSec  = "cred_health_probe_interval_sec"  // 凭据惰性探活最小间隔（秒）
)

// HealthCheckLog 状态取值（单源：后端统计、判停计数、前端展示共用一个口径，禁散落字面量）。
const (
	// HealthCheckStatusSuccess：请求发出去且上游给了通过结论。
	HealthCheckStatusSuccess = "success"
	// HealthCheckStatusError：请求发出去且上游给了失败结论（超时 / 非通过状态码 / 内容形状非法）。
	HealthCheckStatusError = "error"
	// HealthCheckStatusNotMeasured：本机原因没测成（读不到库、没有启用的端点、没有可用凭据），
	// 不是上游的结论——不计入判停、不触发权重/优先级衰减、也不打断已有连续失败（Stage 36）。
	HealthCheckStatusNotMeasured = "not_measured"
)

// HealthCheckLog 模型健康检测日志
type HealthCheckLog struct {
	gorm.Model
	BatchID         string    `gorm:"index" json:"batch_id,omitempty"` // 批次ID，用于批量检测追踪
	ModelProviderID uint      `gorm:"index" json:"model_provider_id"`  // 关联的 ModelWithProvider ID
	ModelName       string    `gorm:"index" json:"model_name"`         // 模型名称
	ProviderName    string    `gorm:"index" json:"provider_name"`      // 提供商名称
	ProviderModel   string    `json:"provider_model"`                  // 提供商模型名称
	Status          string    `gorm:"index" json:"status"`             // 检测状态: success, error, not_measured（见上方常量）
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
