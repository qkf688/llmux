package chat

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
)

// TestRecordLog_Success_BackfillsProxyTime 回归保护：ProxyTime 只在建行瞬间赋过一次值
// （该时刻上游请求尚未发出，见 executeSingleProviderAttempt），此后成功路径从不回填，
// 所有成功日志的「代理耗时」恒为近零快照。要求 RecordLog 成功分支把端到端耗时写回。
func TestRecordLog_Success_BackfillsProxyTime(t *testing.T) {
	initChatRecordTestDB(t)

	log := models.ChatLog{
		Name:          "m1",
		ProviderModel: "pm1",
		ProviderName:  "p1",
		Status:        "success",
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	// 回拨 ReqStart：真实请求中它是 handler 的 startReq，processer 读到 EOF 返回时
	// 必然已过毫秒级时间；回拨让断言有可测阈值，不必在测试里真实 sleep。
	reqStart := time.Now().Add(-300 * time.Millisecond)

	proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		_, _ = io.ReadAll(pr)
		return &models.ChatLog{Usage: models.Usage{TotalTokens: 1}}, &models.OutputUnion{}, nil
	}

	RecordLog(context.Background(), RecordLogInput{
		ReqStart:     reqStart,
		Reader:       io.NopCloser(strings.NewReader("x")),
		Processer:    proc,
		LogID:        log.ID,
		Before:       Before{Stream: false, raw: []byte(`{}`)},
		ProviderName: "p1",
	})

	var got models.ChatLog
	if err := models.DB.First(&got, log.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "success" {
		t.Fatalf("Status = %q, want success", got.Status)
	}
	if got.ProxyTime < 250*time.Millisecond {
		t.Fatalf("ProxyTime = %v, want ≈ 300ms（端到端耗时回填），当前仍是建行近零快照", got.ProxyTime)
	}
}

// TestRecordLog_ProcesserError_BackfillsProxyTime 回归保护（212 现场路径）：客户端
// 断开后 processer 读到流中断返回错误（如 context canceled），RecordLog 错误分支
// 只更新 Status/Error，ProxyTime 从不回填 → 错误日志「代理耗时」同样是近零快照。
// 要求错误分支的 UpdateByID 同时写回端到端耗时。
func TestRecordLog_ProcesserError_BackfillsProxyTime(t *testing.T) {
	initChatRecordTestDB(t)

	log := models.ChatLog{
		Name:          "m1",
		ProviderModel: "pm1",
		ProviderName:  "p1",
		Status:        "success",
	}
	if err := models.DB.Create(&log).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	reqStart := time.Now().Add(-300 * time.Millisecond)

	proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		_, _ = io.ReadAll(pr)
		// 212 场景：客户端断开，流读取在超时后被取消。
		return nil, nil, context.Canceled
	}

	RecordLog(context.Background(), RecordLogInput{
		ReqStart:     reqStart,
		Reader:       io.NopCloser(strings.NewReader("x")),
		Processer:    proc,
		LogID:        log.ID,
		Before:       Before{Stream: false, raw: []byte(`{}`)},
		ProviderName: "p1",
	})

	var got models.ChatLog
	if err := models.DB.First(&got, log.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "error" {
		t.Fatalf("Status = %q, want error", got.Status)
	}
	if got.ProxyTime < 250*time.Millisecond {
		t.Fatalf("ProxyTime = %v, want ≈ 300ms（错误路径端到端耗时回填），当前仍是建行近零快照", got.ProxyTime)
	}
}
