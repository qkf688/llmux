package chat

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/credwrite"
)

// discardResponseProcesser 是「只把响应体读完、不解析 usage / 计时」的 processer 占位：
// 本文件两个用例只验证 raw 字段的清理策略，不关心解析结果，用它把 RecordLog 的入参填齐。
// 单一定义：两处原先各写一份逐字相同的闭包，被重复率门判为新增重复块。
var discardResponseProcesser Processer = func(_ context.Context, pr io.Reader, _ bool, _ time.Time) (*models.ChatLog, *models.OutputUnion, error) {
	_, _ = io.ReadAll(pr)
	return &models.ChatLog{}, &models.OutputUnion{OfString: `{"ok":true}`}, nil
}

// runRawClearRecordLog 用占位 processer 对给定日志行跑一次后处理：本文件两个用例的
// 入参逐字相同，收成一个入口——重复率门会把两处逐字块判为新增重复。
func runRawClearRecordLog(t *testing.T, logID uint) {
	t.Helper()
	RecordLog(context.Background(), RecordLogInput{
		ReqStart:     time.Now(),
		Reader:       io.NopCloser(strings.NewReader("x")),
		Processer:    discardResponseProcesser,
		LogID:        logID,
		Before:       Before{Stream: false, raw: []byte(`{}`)},
		ProviderName: "p",
	})
}

func initChatRecordTestDB(t *testing.T) {
	t.Helper()
	credwrite.EnableTestSyncMode(true)
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmux-test.db"))
	// 与 models.DB 同步默认 Repositories，避免 Default 缓存上一个用例的连接
	repository.SetDefault(repository.New(models.DB))
	t.Cleanup(func() {
		// 测试基线保持 sync=true，避免后续用例误入异步队列写已关库。
		credwrite.EnableTestSyncMode(true)
		credwrite.ResetDroppedForTest()
		repository.SetDefault(nil)
		if models.DB != nil {
			sqlDB, err := models.DB.DB()
			if err == nil {
				_ = sqlDB.Close()
			}
		}
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

	log := testutil.NewRow(t, models.DB, models.ChatLog{
		Name:            "m",
		ProviderModel:   "pm",
		ProviderName:    "p",
		Status:          "success",
		RequestHeaders:  `{"x":"y"}`,
		RequestBody:     `{"in":"1"}`,
		ResponseHeaders: `{"Content-Type":["application/json"]}`,
		ResponseBody:    `{"will":"be cleared"}`,
		RawResponseBody: `{"raw":"be cleared"}`,
	})

	runRawClearRecordLog(t, log.ID)

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

	log := testutil.NewRow(t, models.DB, models.ChatLog{
		Name:            "m",
		ProviderModel:   "pm",
		ProviderName:    "p",
		Status:          "error",
		RequestHeaders:  `{"x":"y"}`,
		RequestBody:     `{"in":"1"}`,
		ResponseHeaders: `{"Content-Type":["application/json"]}`,
		ResponseBody:    `{"keep":"me"}`,
		RawResponseBody: `{"raw":"keep"}`,
	})

	runRawClearRecordLog(t, log.ID)

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
