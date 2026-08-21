package logs

import (
	"errors"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/transform"
)

// chatLogResponse 是 /api/logs 列表与 /api/logs/:id 详情的响应契约。
//
// 独立于 models.ChatLog 定义，原因有二：
//   - 持久化模型的字段改名不应波及对外 API（此前 ChatIO 直接裸序列化，
//     改一个 Go 字段名就破坏了前端契约）；
//   - 对外字段统一 snake_case（AGENTS.md 3.1），模型层保持 Go 命名。
//
// 用结构体而非 map[string]any：编译器能保证字段不漏。历史上
// completion_tokens_details 就是在 map 版本里漏写的，落库有值但 API 不返回。
type chatLogResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`

	Name          string `json:"name"`
	ProviderModel string `json:"provider_model"`
	ProviderName  string `json:"provider_name"`
	Status        string `json:"status"`
	Style         string `json:"style"`
	UserAgent     string `json:"user_agent"`
	RemoteIP      string `json:"remote_ip"`
	ChatIO        bool   `json:"chat_io"`

	Error          string        `json:"error"`
	Retry          int           `json:"retry"`
	ProxyTime      time.Duration `json:"proxy_time"`
	FirstChunkTime time.Duration `json:"first_chunk_time"`
	ChunkTime      time.Duration `json:"chunk_time"`
	Tps            float64       `json:"tps"`

	// details 两个子结构直接复用 models 侧的值对象：它们已是 snake_case
	// 且无持久化语义，再复制一份定义只会产生两处需要同步的真相。
	PromptTokens            int64                          `json:"prompt_tokens"`
	CompletionTokens        int64                          `json:"completion_tokens"`
	TotalTokens             int64                          `json:"total_tokens"`
	PromptTokensDetails     models.PromptTokensDetails     `json:"prompt_tokens_details"`
	CompletionTokensDetails models.CompletionTokensDetails `json:"completion_tokens_details"`
	// UsageSource 暴露给管理端用于排查「token 全为 0」是上游没给还是归集链路丢了。
	UsageSource string `json:"usage_source"`

	IsVirtualModel      bool `json:"is_virtual_model"`
	HasFormatConversion bool `json:"has_format_conversion"`
	// 仅在 HasFormatConversion 为真时有值，此时两者必非空（见 enrichChatLogs 的判定）。
	SourceFormat string `json:"source_format,omitempty"`
	TargetFormat string `json:"target_format,omitempty"`

	// raw 组：列表默认不返回，include_raw=true 时整组出现。
	// 用指针而非 string+omitempty——空字符串是合法值，必须与「本次不返回该字段」区分。
	RequestHeaders  *string `json:"request_headers,omitempty"`
	RequestBody     *string `json:"request_body,omitempty"`
	RawRequestBody  *string `json:"raw_request_body,omitempty"`
	ResponseHeaders *string `json:"response_headers,omitempty"`
	ResponseBody    *string `json:"response_body,omitempty"`
	RawResponseBody *string `json:"raw_response_body,omitempty"`

	// UnclaimedRequestFields 是读时计算的诊断字段（不落库），指出客户端原始请求体里
	// 有哪些顶层键本网关根本没解析——即转换后会静默消失的字段。
	//
	// 与 raw 组同一个门控：它的输入就是 RawRequestBody，include_raw=false 时那个字段
	// 压根没从库里读出来，此时算出来的只会是假的「未记录」。
	UnclaimedRequestFields *requestFieldDiagnostics `json:"unclaimed_request_fields,omitempty"`

	// MismatchedRequestFields 是另一类静默：键**被认领了**，但值的类型与入站 DTO 不符，
	// 被宽容容器当成「没传」丢掉（`{"temperature":"0.5"}` 会 200 通过但参数不生效）。
	//
	// 与 UnclaimedRequestFields 分成两个字段而非合并成一个列表：两者的成因与修法不同
	// （前者是网关没实现该字段，后者是客户端传错类型），混在一起用户无法判断该改谁。
	// 且两项检测的支持面不重合——openai-res 支持前者、不支持后者。
	MismatchedRequestFields *requestFieldDiagnostics `json:"mismatched_request_fields,omitempty"`
}

// requestFieldDiagnostics 用状态枚举而非「nil / 空数组」表达检测结果。
//
// 因为「查不出来」有三种彼此需要区分的原因（原始 body 没记、该协议不支持检测、
// body 解析失败），挤进一个可空数组的话前端只能靠猜，最坏是把「查不了」显示成
// 「已检查、无问题」——假阴性比没有这个功能更糟。
//
// 未认领与类型不匹配两项检测共用本类型：四态语义逐条相同，各写一份必然漂移。
type requestFieldDiagnostics struct {
	Status string `json:"status"`
	// Fields 仅 Status 为 ok 时有意义；无命中键时是空数组而非 null。
	Fields []string `json:"fields"`
	// Detail 只在 parse_error 时给出，便于定位是哪种畸形 body。
	Detail string `json:"detail,omitempty"`
}

const (
	// unclaimedStatusOK：已完成检测，Fields 即结论（可能为空）。
	unclaimedStatusOK = "ok"
	// unclaimedStatusRawNotRecorded：原始请求体没落库，无从检测。
	// 常见原因是 log_raw_request_response 开关默认全关，或 errors_only 在成功时清空了它。
	unclaimedStatusRawNotRecorded = "raw_not_recorded"
	// unclaimedStatusStyleUnsupported：该入站协议没有注册对应的检测实现。
	// 未认领检测：三个生产 style（openai / openai-res / anthropic）现已全部注册，故该项
	// 在生产**暂无可达路径**；保留是因为它仍是活契约——新 style 落地时必然先经过未注册
	// 阶段，那期间只有本状态能把「查不了」与「已检查、无命中」区分开。
	// 类型不匹配检测：**openai-res 当前就走这一态**（它的入站 DTO 用裸类型/指针字段，
	// 类型不对时整条请求解析失败，不存在静默丢弃可报）。
	unclaimedStatusStyleUnsupported = "style_unsupported"
	// unclaimedStatusParseError：原始 body 不是 JSON 对象，键的概念不成立。
	unclaimedStatusParseError = "parse_error"
)

// computeUnclaimedRequestFields 读时计算，纯函数：只读传入的日志行，不查库不写库。
func computeUnclaimedRequestFields(log models.ChatLog) *requestFieldDiagnostics {
	// 必须传**入站** style（ChatLog.Style 存的就是客户端进来时的 style）：传出站协议
	// 会把转换时改名的字段全部误报成未知。
	return computeRequestFieldDiagnostics(log, transform.UnknownRequestKeys, transform.ErrClaimedKeysUnsupported)
}

// computeMismatchedRequestFields 读时计算「被认领但类型不匹配而丢弃」的顶层键。
func computeMismatchedRequestFields(log models.ChatLog) *requestFieldDiagnostics {
	return computeRequestFieldDiagnostics(log, transform.MismatchedRequestKeys, transform.ErrMismatchedKeysUnsupported)
}

// computeRequestFieldDiagnostics 是两项检测共用的四态编排。
//
// 抽出来是因为两者只差「调哪个检测函数、认哪个 unsupported 哨兵」，其余（raw 未记录
// 优先、unsupported 与 parse_error 分流、Fields 必为非 nil 切片）必须逐条一致——
// 复制一份必然在某次改动里只改了一侧，让同一个弹窗里两块状态语义不同。
func computeRequestFieldDiagnostics(
	log models.ChatLog,
	detect func(style consts.Style, rawBody []byte) ([]string, error),
	errUnsupported error,
) *requestFieldDiagnostics {
	if log.RawRequestBody == "" {
		return &requestFieldDiagnostics{Status: unclaimedStatusRawNotRecorded, Fields: []string{}}
	}

	fields, err := detect(consts.Style(log.Style), []byte(log.RawRequestBody))
	switch {
	case errors.Is(err, errUnsupported):
		return &requestFieldDiagnostics{Status: unclaimedStatusStyleUnsupported, Fields: []string{}}
	case err != nil:
		return &requestFieldDiagnostics{
			Status: unclaimedStatusParseError,
			Fields: []string{},
			Detail: err.Error(),
		}
	default:
		return &requestFieldDiagnostics{Status: unclaimedStatusOK, Fields: fields}
	}
}

// chatIOResponse 是 /api/logs/:id/chat-io 的响应契约。
//
// 不含 gorm.Model 的 ID / CreatedAt / UpdatedAt / DeletedAt：这条记录只为
// 展示某条日志的输入输出，主键与软删时间对调用方无意义。
type chatIOResponse struct {
	LogID         uint     `json:"log_id"`
	Input         string   `json:"input"`
	OfString      string   `json:"of_string"`
	OfStringArray []string `json:"of_string_array"`
}

func buildChatLogResponse(log models.ChatLog, enrich chatLogEnrichResult, includeRaw bool) chatLogResponse {
	resp := chatLogResponse{
		ID:        log.ID,
		CreatedAt: log.CreatedAt,

		Name:          log.Name,
		ProviderModel: log.ProviderModel,
		ProviderName:  log.ProviderName,
		Status:        log.Status,
		Style:         log.Style,
		UserAgent:     log.UserAgent,
		RemoteIP:      log.RemoteIP,
		ChatIO:        log.ChatIO,

		Error:          log.Error,
		Retry:          log.Retry,
		ProxyTime:      log.ProxyTime,
		FirstChunkTime: log.FirstChunkTime,
		ChunkTime:      log.ChunkTime,
		Tps:            log.Tps,

		PromptTokens:            log.PromptTokens,
		CompletionTokens:        log.CompletionTokens,
		TotalTokens:             log.TotalTokens,
		PromptTokensDetails:     log.PromptTokensDetails,
		CompletionTokensDetails: log.CompletionTokensDetails,
		UsageSource:             log.UsageSource,

		IsVirtualModel:      enrich.isVirtualModel,
		HasFormatConversion: enrich.hasFormatConversion,
	}

	if enrich.hasFormatConversion {
		resp.SourceFormat = enrich.sourceFormat
		resp.TargetFormat = enrich.targetFormat
	}

	if includeRaw {
		resp.RequestHeaders = &log.RequestHeaders
		resp.RequestBody = &log.RequestBody
		resp.RawRequestBody = &log.RawRequestBody
		resp.ResponseHeaders = &log.ResponseHeaders
		resp.ResponseBody = &log.ResponseBody
		resp.RawResponseBody = &log.RawResponseBody
		resp.UnclaimedRequestFields = computeUnclaimedRequestFields(log)
		resp.MismatchedRequestFields = computeMismatchedRequestFields(log)
	}

	return resp
}

func buildChatIOResponse(io models.ChatIO) chatIOResponse {
	return chatIOResponse{
		LogID:         io.LogID,
		Input:         io.Input,
		OfString:      io.OfString,
		OfStringArray: io.OfStringArray,
	}
}
