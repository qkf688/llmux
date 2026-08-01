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
		Name:            "m1",
		ProviderName:    "p1",
		ProviderModel:   "pm1",
		Status:          "success",
		Style:           "anthropic",
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
	if _, ok := item["RequestHeaders"]; ok {
		t.Fatalf("expected RequestHeaders omitted in list response")
	}
	if _, ok := item["RequestBody"]; ok {
		t.Fatalf("expected RequestBody omitted in list response")
	}
	if _, ok := item["RawRequestBody"]; ok {
		t.Fatalf("expected RawRequestBody omitted in list response")
	}
	if _, ok := item["ResponseHeaders"]; ok {
		t.Fatalf("expected ResponseHeaders omitted in list response")
	}
	if _, ok := item["ResponseBody"]; ok {
		t.Fatalf("expected ResponseBody omitted in list response")
	}
	if _, ok := item["RawResponseBody"]; ok {
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
	if item["RequestHeaders"] != log.RequestHeaders {
		t.Fatalf("RequestHeaders = %v, want %v", item["RequestHeaders"], log.RequestHeaders)
	}
	if item["RequestBody"] != log.RequestBody {
		t.Fatalf("RequestBody = %v, want %v", item["RequestBody"], log.RequestBody)
	}
	if item["RawRequestBody"] != log.RawRequestBody {
		t.Fatalf("RawRequestBody = %v, want %v", item["RawRequestBody"], log.RawRequestBody)
	}
	if item["ResponseHeaders"] != log.ResponseHeaders {
		t.Fatalf("ResponseHeaders = %v, want %v", item["ResponseHeaders"], log.ResponseHeaders)
	}
	if item["ResponseBody"] != log.ResponseBody {
		t.Fatalf("ResponseBody = %v, want %v", item["ResponseBody"], log.ResponseBody)
	}
	if item["RawResponseBody"] != log.RawResponseBody {
		t.Fatalf("RawResponseBody = %v, want %v", item["RawResponseBody"], log.RawResponseBody)
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

	if payload.Data["RequestHeaders"] != log.RequestHeaders {
		t.Fatalf("RequestHeaders = %v, want %v", payload.Data["RequestHeaders"], log.RequestHeaders)
	}
	if payload.Data["RequestBody"] != log.RequestBody {
		t.Fatalf("RequestBody = %v, want %v", payload.Data["RequestBody"], log.RequestBody)
	}
	if payload.Data["RawRequestBody"] != log.RawRequestBody {
		t.Fatalf("RawRequestBody = %v, want %v", payload.Data["RawRequestBody"], log.RawRequestBody)
	}
	if payload.Data["ResponseHeaders"] != log.ResponseHeaders {
		t.Fatalf("ResponseHeaders = %v, want %v", payload.Data["ResponseHeaders"], log.ResponseHeaders)
	}
	if payload.Data["ResponseBody"] != log.ResponseBody {
		t.Fatalf("ResponseBody = %v, want %v", payload.Data["ResponseBody"], log.ResponseBody)
	}
	if payload.Data["RawResponseBody"] != log.RawResponseBody {
		t.Fatalf("RawResponseBody = %v, want %v", payload.Data["RawResponseBody"], log.RawResponseBody)
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
