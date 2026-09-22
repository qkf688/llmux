package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
)

// TestRecordLog_ProcesserFailure_LogsExactlyOnce 后处理失败的留痕计数契约（Stage 18 口径）：
// 一次失败恰一条留痕（RecordLog 收口），内层 processer 分支禁自记——内层+外层双份会把
// 一次故障算成两次（按 ERROR 行计数的告警翻倍）。2026-09-23 回批前实况即双份。
// 错误细节同时必须落 chat_logs（Status=error）：客户端零字节语义的落库侧。
func TestRecordLog_ProcesserFailure_LogsExactlyOnce(t *testing.T) {
	initChatRecordTestDB(t)

	logRow := models.ChatLog{Name: "m", ProviderModel: "pm", ProviderName: "p", Status: "success"}
	if err := models.DB.Create(&logRow).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		return nil, nil, errors.New("parse boom")
	}

	RecordLog(context.Background(), RecordLogInput{
		ReqStart:     time.Now(),
		Reader:       io.NopCloser(strings.NewReader("x")),
		Processer:    proc,
		LogID:        logRow.ID,
		Before:       Before{Stream: false},
		ProviderName: "p",
	})

	ls := strings.TrimSpace(logs.String())
	if n := strings.Count(ls, "\n"); n != 0 {
		t.Errorf("一次后处理失败写了 %d 条日志，want 1（双份留痕会让告警把一次故障算成两次）:\n%s", n+1, ls)
	}
	if !strings.Contains(ls, "record log error") {
		t.Errorf("缺收口留痕（唯一一条应出自 RecordLog 收口）:\n%s", ls)
	}
	if !strings.Contains(ls, "parse boom") {
		t.Errorf("错误细节未进留痕:\n%s", ls)
	}

	var got models.ChatLog
	if err := models.DB.First(&got, logRow.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "error" {
		t.Fatalf("chat_logs Status = %q, want %q（失败须落库）", got.Status, "error")
	}
}

// TestRecordLog_ClientDisconnect_LogsWarn 客户端断连类留痕降 Warn（framework-conventions §5
// 级别约定）：断连不是服务器故障，按 ERROR 计数的告警会被「建连即断」刷爆。
func TestRecordLog_ClientDisconnect_LogsWarn(t *testing.T) {
	initChatRecordTestDB(t)

	logRow := models.ChatLog{Name: "m", ProviderModel: "pm", ProviderName: "p", Status: "success"}
	if err := models.DB.Create(&logRow).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		// 模拟写客户端断连经日志管道传回的失败链（writeResponseBody 的包装形态）。
		return nil, nil, fmt.Errorf("write response body failed: %w", io.ErrClosedPipe)
	}

	RecordLog(context.Background(), RecordLogInput{
		ReqStart:     time.Now(),
		Reader:       io.NopCloser(strings.NewReader("x")),
		Processer:    proc,
		LogID:        logRow.ID,
		Before:       Before{Stream: false},
		ProviderName: "p",
	})

	ls := strings.TrimSpace(logs.String())
	if !strings.Contains(ls, "level=WARN") {
		t.Errorf("断连类留痕应为 WARN:\n%s", ls)
	}
	if strings.Contains(ls, "level=ERROR") {
		t.Errorf("断连类不应留 ERROR（会刷爆告警）:\n%s", ls)
	}
}
