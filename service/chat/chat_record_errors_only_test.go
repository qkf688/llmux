package chat

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
)

func initChatRecordTestDB(t *testing.T) {
	t.Helper()
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmio-test.db"))
	t.Cleanup(func() {
		sqlDB, err := models.DB.DB()
		if err != nil {
			return
		}
		_ = sqlDB.Close()
	})
}

func TestRecordLog_ErrorsOnly_ClearsRawFieldsOnSuccess(t *testing.T) {
	initChatRecordTestDB(t)

	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyLogRawRequestResponse).
		Update("value", `{"request_headers":true,"request_body":true,"response_headers":true,"response_body":true,"raw_response_body":true}`).Error; err != nil {
		t.Fatalf("set log raw options: %v", err)
	}
	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyLogRawRequestResponseErrorsOnly).
		Update("value", "true").Error; err != nil {
		t.Fatalf("enable errors-only: %v", err)
	}

	log := models.ChatLog{
		Name:            "m",
		ProviderModel:   "pm",
		ProviderName:    "p",
		Status:          "success",
		RequestHeaders:  `{"x":"y"}`,
		RequestBody:     `{"in":"1"}`,
		ResponseHeaders: `{"Content-Type":["application/json"]}`,
		ResponseBody:    `{"will":"be cleared"}`,
		RawResponseBody: `{"raw":"be cleared"}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		_, _ = io.ReadAll(pr)
		return &models.ChatLog{}, &models.OutputUnion{OfString: `{"ok":true}`}, nil
	}

	RecordLog(context.Background(), time.Now(), io.NopCloser(strings.NewReader("x")), proc, log.ID, Before{Stream: false, raw: []byte(`{}`)}, false, "p")

	var got models.ChatLog
	if err := models.DB.First(&got, log.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "success" {
		t.Fatalf("status = %q, want %q", got.Status, "success")
	}
	if got.RequestHeaders != "" || got.RequestBody != "" || got.ResponseHeaders != "" || got.ResponseBody != "" || got.RawResponseBody != "" {
		t.Fatalf("expected all raw fields cleared, got request_headers=%q request_body=%q response_headers=%q response_body=%q raw_response_body=%q",
			got.RequestHeaders, got.RequestBody, got.ResponseHeaders, got.ResponseBody, got.RawResponseBody)
	}
}

func TestRecordLog_ErrorsOnly_DoesNotClearRawFieldsOnErrorStatus(t *testing.T) {
	initChatRecordTestDB(t)

	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyLogRawRequestResponse).
		Update("value", `{"request_headers":true,"request_body":true,"response_headers":true,"response_body":true,"raw_response_body":true}`).Error; err != nil {
		t.Fatalf("set log raw options: %v", err)
	}
	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyLogRawRequestResponseErrorsOnly).
		Update("value", "true").Error; err != nil {
		t.Fatalf("enable errors-only: %v", err)
	}

	log := models.ChatLog{
		Name:            "m",
		ProviderModel:   "pm",
		ProviderName:    "p",
		Status:          "error",
		RequestHeaders:  `{"x":"y"}`,
		RequestBody:     `{"in":"1"}`,
		ResponseHeaders: `{"Content-Type":["application/json"]}`,
		ResponseBody:    `{"keep":"me"}`,
		RawResponseBody: `{"raw":"keep"}`,
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		_, _ = io.ReadAll(pr)
		return &models.ChatLog{}, &models.OutputUnion{OfString: `{"ok":true}`}, nil
	}

	RecordLog(context.Background(), time.Now(), io.NopCloser(strings.NewReader("x")), proc, log.ID, Before{Stream: false, raw: []byte(`{}`)}, false, "p")

	var got models.ChatLog
	if err := models.DB.First(&got, log.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "error" {
		t.Fatalf("status = %q, want %q", got.Status, "error")
	}
	if got.RequestHeaders == "" || got.RequestBody == "" || got.ResponseHeaders == "" || got.ResponseBody == "" || got.RawResponseBody == "" {
		t.Fatalf("expected raw fields kept on error status, got request_headers=%q request_body=%q response_headers=%q response_body=%q raw_response_body=%q",
			got.RequestHeaders, got.RequestBody, got.ResponseHeaders, got.ResponseBody, got.RawResponseBody)
	}
}
