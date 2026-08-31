package logs

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

type requestLogsResponse struct {
	Data     []map[string]any `json:"data"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Pages    int64            `json:"pages"`
}

func TestGetRequestLogs_DefaultOmitRawFields(t *testing.T) {
	testsupport.InitTestDB(t)

	enabled := true
	virtualModel := models.VirtualModel{Name: "m1", Enabled: &enabled}
	if err := models.DB.Create(&virtualModel).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}

	provider := models.Provider{Name: "p1", Type: "openai"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	log := models.ChatLog{
		Name:             "m1",
		ProviderName:     "p1",
		ProviderModel:    "pm1",
		Status:           "success",
		Style:            "anthropic",
		EndpointProtocol: "openai", // S3-3 起转换判定按端点协议：入站 anthropic × 出站 openai = 有转换
		RequestHeaders:   `{"x":"y"}`,
		RequestBody:      `{"hello":"world"}`,
		RawRequestBody:   `{"raw":"client"}`,
		ResponseHeaders:  `{"a":"b"}`,
		ResponseBody:     `{"ok":true}`,
		RawResponseBody:  `{"raw":true}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs?page=1&page_size=20")
	GetRequestLogs(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[requestLogsResponse]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if len(payload.Data.Data) != 1 {
		t.Fatalf("logs length = %d, want 1", len(payload.Data.Data))
	}

	item := payload.Data.Data[0]
	if _, ok := item["request_headers"]; ok {
		t.Fatalf("expected RequestHeaders omitted in list response")
	}
	if _, ok := item["request_body"]; ok {
		t.Fatalf("expected RequestBody omitted in list response")
	}
	if _, ok := item["raw_request_body"]; ok {
		t.Fatalf("expected RawRequestBody omitted in list response")
	}
	if _, ok := item["response_headers"]; ok {
		t.Fatalf("expected ResponseHeaders omitted in list response")
	}
	if _, ok := item["response_body"]; ok {
		t.Fatalf("expected ResponseBody omitted in list response")
	}
	if _, ok := item["raw_response_body"]; ok {
		t.Fatalf("expected RawResponseBody omitted in list response")
	}

	if val, ok := item["is_virtual_model"].(bool); !ok || !val {
		t.Fatalf("is_virtual_model = %v, want true", item["is_virtual_model"])
	}
	if val, ok := item["has_format_conversion"].(bool); !ok || !val {
		t.Fatalf("has_format_conversion = %v, want true", item["has_format_conversion"])
	}
	if _, ok := item["source_format"]; !ok {
		t.Fatalf("expected source_format present when has_format_conversion=true")
	}
	if _, ok := item["target_format"]; !ok {
		t.Fatalf("expected target_format present when has_format_conversion=true")
	}
}

func TestGetRequestLogs_IncludeRawReturnsRawFields(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:            "m1",
		ProviderName:    "p1",
		ProviderModel:   "pm1",
		Status:          "success",
		Style:           "openai",
		RequestHeaders:  `{"x":"y"}`,
		RequestBody:     `{"hello":"world"}`,
		RawRequestBody:  `{"raw":"client"}`,
		ResponseHeaders: `{"a":"b"}`,
		ResponseBody:    `{"ok":true}`,
		RawResponseBody: `{"raw":true}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs?page=1&page_size=20&include_raw=true")
	GetRequestLogs(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[requestLogsResponse]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if len(payload.Data.Data) != 1 {
		t.Fatalf("logs length = %d, want 1", len(payload.Data.Data))
	}

	item := payload.Data.Data[0]
	if item["request_headers"] != log.RequestHeaders {
		t.Fatalf("RequestHeaders = %v, want %v", item["request_headers"], log.RequestHeaders)
	}
	if item["request_body"] != log.RequestBody {
		t.Fatalf("RequestBody = %v, want %v", item["request_body"], log.RequestBody)
	}
	if item["raw_request_body"] != log.RawRequestBody {
		t.Fatalf("RawRequestBody = %v, want %v", item["raw_request_body"], log.RawRequestBody)
	}
	if item["response_headers"] != log.ResponseHeaders {
		t.Fatalf("ResponseHeaders = %v, want %v", item["response_headers"], log.ResponseHeaders)
	}
	if item["response_body"] != log.ResponseBody {
		t.Fatalf("ResponseBody = %v, want %v", item["response_body"], log.ResponseBody)
	}
	if item["raw_response_body"] != log.RawResponseBody {
		t.Fatalf("RawResponseBody = %v, want %v", item["raw_response_body"], log.RawResponseBody)
	}
}

func TestGetRequestLogDetail_ReturnsRawFields(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:            "m1",
		ProviderName:    "p1",
		ProviderModel:   "pm1",
		Status:          "success",
		Style:           "openai",
		RequestHeaders:  `{"x":"y"}`,
		RequestBody:     `{"hello":"world"}`,
		RawRequestBody:  `{"raw":"client"}`,
		ResponseHeaders: `{"a":"b"}`,
		ResponseBody:    `{"ok":true}`,
		RawResponseBody: `{"raw":true}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs/"+strconv.FormatUint(uint64(log.ID), 10))
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(log.ID), 10)}}
	GetRequestLogDetail(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[map[string]any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}

	if payload.Data["request_headers"] != log.RequestHeaders {
		t.Fatalf("RequestHeaders = %v, want %v", payload.Data["request_headers"], log.RequestHeaders)
	}
	if payload.Data["request_body"] != log.RequestBody {
		t.Fatalf("RequestBody = %v, want %v", payload.Data["request_body"], log.RequestBody)
	}
	if payload.Data["raw_request_body"] != log.RawRequestBody {
		t.Fatalf("RawRequestBody = %v, want %v", payload.Data["raw_request_body"], log.RawRequestBody)
	}
	if payload.Data["response_headers"] != log.ResponseHeaders {
		t.Fatalf("ResponseHeaders = %v, want %v", payload.Data["response_headers"], log.ResponseHeaders)
	}
	if payload.Data["response_body"] != log.ResponseBody {
		t.Fatalf("ResponseBody = %v, want %v", payload.Data["response_body"], log.ResponseBody)
	}
	if payload.Data["raw_response_body"] != log.RawResponseBody {
		t.Fatalf("RawResponseBody = %v, want %v", payload.Data["raw_response_body"], log.RawResponseBody)
	}
}

func TestGetRequestLogDetail_NotFound(t *testing.T) {
	testsupport.InitTestDB(t)

	c, w := testsupport.NewTestContext("GET", "/logs/999")
	c.Params = gin.Params{{Key: "id", Value: "999"}}
	GetRequestLogDetail(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 404 {
		t.Fatalf("payload code = %d, want 404, body=%s", payload.Code, w.Body.String())
	}
}

// TestGetRequestLogs_ReturnsUsageDetailsAndSource 回归保护：早期用 map[string]any
// 手拼响应时漏写了 completion_tokens_details——reasoning_tokens 已落库但 API 永不返回，
// 前端类型声明成了谎言；usage_source 同样从未暴露，管理端无法区分「上游没给 token」
// 与「归集链路丢了」。改用结构体 DTO 后编译器保证字段齐全，此测试锁住这两个字段。
func TestGetRequestLogs_ReturnsUsageDetailsAndSource(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:         "m1",
		ProviderName: "p1",
		Status:       "success",
		Style:        "openai",
		Usage: models.Usage{
			PromptTokens:        10,
			CompletionTokens:    20,
			TotalTokens:         30,
			PromptTokensDetails: models.PromptTokensDetails{CachedTokens: 3},
			CompletionTokensDetails: models.CompletionTokensDetails{
				ReasoningTokens:      7,
				ReasoningTokensKnown: true,
			},
		},
		UsageSource: models.UsageSourceUpstream,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs?page=1&page_size=20")
	GetRequestLogs(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[requestLogsResponse]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if len(payload.Data.Data) != 1 {
		t.Fatalf("logs length = %d, want 1", len(payload.Data.Data))
	}

	item := payload.Data.Data[0]

	completionDetails, ok := item["completion_tokens_details"].(map[string]any)
	if !ok {
		t.Fatalf("completion_tokens_details missing or not an object: %v", item["completion_tokens_details"])
	}
	if completionDetails["reasoning_tokens"] != float64(7) {
		t.Fatalf("reasoning_tokens = %v, want 7", completionDetails["reasoning_tokens"])
	}
	if completionDetails["reasoning_tokens_known"] != true {
		t.Fatalf("reasoning_tokens_known = %v, want true（known 必须透传到响应，前端据此区分「未知」与「真 0」）", completionDetails["reasoning_tokens_known"])
	}

	promptDetails, ok := item["prompt_tokens_details"].(map[string]any)
	if !ok {
		t.Fatalf("prompt_tokens_details missing or not an object: %v", item["prompt_tokens_details"])
	}
	if promptDetails["cached_tokens"] != float64(3) {
		t.Fatalf("cached_tokens = %v, want 3", promptDetails["cached_tokens"])
	}

	if item["usage_source"] != models.UsageSourceUpstream {
		t.Fatalf("usage_source = %v, want %v", item["usage_source"], models.UsageSourceUpstream)
	}
}
