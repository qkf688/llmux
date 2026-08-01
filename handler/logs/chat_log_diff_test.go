package logs

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

func TestGetRequestLogDiff_DifferentBodies(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:           "m1",
		ProviderName:   "p1",
		ProviderModel:  "pm1",
		Status:         "success",
		Style:          "anthropic",
		RawRequestBody: `{"model":"claude-3","max_tokens":1048576,"messages":[{"role":"user","content":"hi"}],"system":"you are helpful"}`,
		RequestBody:    `{"model":"claude-3","max_tokens":8192,"messages":[{"role":"user","content":"hi"}]}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs/"+strconv.FormatUint(uint64(log.ID), 10)+"/diff")
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(log.ID), 10)}}
	GetRequestLogDiff(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[common.DiffResult]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// max_tokens 值从 1048576 变为 8192
	foundMaxTokensChange := false
	for _, e := range payload.Data.ChangedValues {
		if e.Path == "max_tokens" {
			foundMaxTokensChange = true
			if e.Raw != float64(1048576) || e.After != float64(8192) {
				t.Fatalf("max_tokens: raw=%v after=%v, want 1048576→8192", e.Raw, e.After)
			}
		}
	}
	if !foundMaxTokensChange {
		t.Fatalf("expected max_tokens in changed_values, got %+v", payload.Data.ChangedValues)
	}

	// system 字段在 raw 有、转换后没有
	foundSystemLost := false
	for _, e := range payload.Data.LostFields {
		if e.Path == "system" {
			foundSystemLost = true
		}
	}
	if !foundSystemLost {
		t.Fatalf("expected system in lost_fields, got %+v", payload.Data.LostFields)
	}
}

func TestGetRequestLogDiff_IdenticalBodies(t *testing.T) {
	testsupport.InitTestDB(t)

	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`
	log := models.ChatLog{
		Name:           "m1",
		ProviderName:   "p1",
		ProviderModel:  "pm1",
		Status:         "success",
		Style:          "openai",
		RawRequestBody: body,
		RequestBody:    body,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs/"+strconv.FormatUint(uint64(log.ID), 10)+"/diff")
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(log.ID), 10)}}
	GetRequestLogDiff(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[common.DiffResult]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Data.LostFields) != 0 || len(payload.Data.AddedFields) != 0 || len(payload.Data.ChangedValues) != 0 {
		t.Fatalf("expected empty diff, got lost=%d added=%d changed=%d",
			len(payload.Data.LostFields), len(payload.Data.AddedFields), len(payload.Data.ChangedValues))
	}
}

func TestGetRequestLogDiff_RawEmptyAllAdded(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:           "m1",
		ProviderName:   "p1",
		ProviderModel:  "pm1",
		Status:         "success",
		Style:          "openai",
		RawRequestBody: "",
		RequestBody:    `{"model":"gpt-4","max_tokens":8192}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs/"+strconv.FormatUint(uint64(log.ID), 10)+"/diff")
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(log.ID), 10)}}
	GetRequestLogDiff(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[common.DiffResult]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Data.AddedFields) != 2 {
		t.Fatalf("expected 2 added fields, got %d: %+v", len(payload.Data.AddedFields), payload.Data.AddedFields)
	}
}

func TestGetRequestLogDiff_InvalidJSON(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:           "m1",
		ProviderName:   "p1",
		ProviderModel:  "pm1",
		Status:         "success",
		Style:          "openai",
		RawRequestBody: `{invalid`,
		RequestBody:    `{"model":"gpt-4"}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs/"+strconv.FormatUint(uint64(log.ID), 10)+"/diff")
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(log.ID), 10)}}
	GetRequestLogDiff(c)
	var payload testsupport.APIEnvelope[common.DiffResult]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Code != 400 {
		t.Fatalf("code = %d, want 400, body=%s", payload.Code, w.Body.String())
	}
}

func TestGetRequestLogDiff_LogNotFound(t *testing.T) {
	testsupport.InitTestDB(t)

	c, w := testsupport.NewTestContext("GET", "/logs/999999/diff")
	c.Params = gin.Params{{Key: "id", Value: "999999"}}
	GetRequestLogDiff(c)

	var payload testsupport.APIEnvelope[common.DiffResult]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Code != 404 {
		t.Fatalf("code = %d, want 404 (not found), body=%s", payload.Code, w.Body.String())
	}
}

func TestGetRequestLogDiff_TransformedEmptyAllLost(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:           "m1",
		ProviderName:   "p1",
		ProviderModel:  "pm1",
		Status:         "success",
		Style:          "openai",
		RawRequestBody: `{"model":"gpt-4","max_tokens":8192}`,
		RequestBody:    "",
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs/"+strconv.FormatUint(uint64(log.ID), 10)+"/diff")
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(log.ID), 10)}}
	GetRequestLogDiff(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[common.DiffResult]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Data.LostFields) != 2 {
		t.Fatalf("expected 2 lost fields, got %d: %+v", len(payload.Data.LostFields), payload.Data.LostFields)
	}
}

func TestGetRequestLogDiff_BothEmpty(t *testing.T) {
	testsupport.InitTestDB(t)

	log := models.ChatLog{
		Name:          "m1",
		ProviderName:  "p1",
		ProviderModel: "pm1",
		Status:        "success",
		Style:         "openai",
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/logs/"+strconv.FormatUint(uint64(log.ID), 10)+"/diff")
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(log.ID), 10)}}
	GetRequestLogDiff(c)
	var payload testsupport.APIEnvelope[common.DiffResult]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.Code != 400 {
		t.Fatalf("code = %d, want 400, body=%s", payload.Code, w.Body.String())
	}
}
