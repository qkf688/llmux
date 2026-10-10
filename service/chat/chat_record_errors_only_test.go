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
// 写成函数声明而非包级变量：变量形态会让签名看不出「它依赖什么」，同包测试还能就地改写它。
func discardResponseProcesser(_ context.Context, pr io.Reader, _ bool, _ time.Time) (*models.ChatLog, *models.OutputUnion, error) {
	_, _ = io.ReadAll(pr)
	return &models.ChatLog{}, &models.OutputUnion{OfString: `{"ok":true}`}, nil
}

// runSuccessfulRecordLog 用占位 processer 对给定日志行跑一次 RecordLog 成功路径：
// 本文件 raw 清理用例与 chat_log_status_machine_test.go 的状态机用例入参逐字相同，
// 收成一个入口（重复率门会把两处逐字块判为新增重复）。
func runSuccessfulRecordLog(t *testing.T, logID uint) {
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
	// 本用例专属的后台任务 Manager：收尾必须能排空它（见 chat_testenv_test.go）。
	testEnvManager(t)
	credwrite.EnableTestSyncMode(true)
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmux-test.db"))
	// 与 models.DB 同步默认 Repositories，避免 Default 缓存上一个用例的连接
	repository.SetDefault(repository.New(models.DB))
	t.Cleanup(func() {
		// 顺序硬约束：先排空后台任务，再清默认 repository / 关库。
		// 后台任务经 repos() 读默认 repository、经 models.GetSettingIntE 读 models.DB，
		// 反序会让在飞任务撞上「库已关闭」，或读到下一个用例的库（-race 记为 data race）。
		drainBgTasks(t)
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

	runSuccessfulRecordLog(t, log.ID)

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

// TestMaybeClearRawOnSuccess_KeepsRawUnlessTerminalSuccess 锁判据方向（状态机）：
// 只有**终态已确认为 success** 的行才清 raw。建行值是 processing，若判据写成「非 error
// 即清」，仍在途/尚未收尾的行会被当成成功行清掉——而 raw 原文恰是排查这类行唯一的东西。
//
// 为什么直调本函数而不经 RecordLog：成功出口已显式写 `Status=success`，「行不是 success」
// 这组入参从 RecordLog 再也造不出来（改前靠「processer 返回空 Status 被零值跳过」碰巧造出）。
// 表驱动覆盖两个非 success 态：processing（未到终态）与 error（终态是失败）。
func TestMaybeClearRawOnSuccess_KeepsRawUnlessTerminalSuccess(t *testing.T) {
	initChatRecordTestDB(t)

	for _, status := range []string{"processing", "error"} {
		t.Run(status, func(t *testing.T) {
			log := testutil.NewRow(t, models.DB, models.ChatLog{
				Name:            "m-" + status,
				ProviderModel:   "pm",
				ProviderName:    "p",
				Status:          status,
				RequestHeaders:  `{"x":"y"}`,
				RequestBody:     `{"in":"1"}`,
				ResponseHeaders: `{"Content-Type":["application/json"]}`,
				ResponseBody:    `{"keep":"me"}`,
				RawResponseBody: `{"raw":"keep"}`,
			})

			maybeClearRawOnSuccess(context.Background(), log.ID, true, true)

			var got models.ChatLog
			if err := models.DB.First(&got, log.ID).Error; err != nil {
				t.Fatalf("reload log: %v", err)
			}
			if got.Status != status {
				t.Fatalf("status = %q, want %q（清理不得顺手改状态）", got.Status, status)
			}
			if got.RequestHeaders == "" || got.RequestBody == "" || got.ResponseHeaders == "" || got.ResponseBody == "" || got.RawResponseBody == "" {
				t.Fatalf("expected raw fields kept on %s status, got request_headers=%q request_body=%q response_headers=%q response_body=%q raw_response_body=%q",
					status, got.RequestHeaders, got.RequestBody, got.ResponseHeaders, got.ResponseBody, got.RawResponseBody)
			}
		})
	}
}
