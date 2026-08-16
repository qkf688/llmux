package logs

import (
	"time"

	"github.com/qkf688/llmux/models"
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
