package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
)

// TestMarkLogUnfinished_WritesFailureTrace 锁丢弃窗口的留痕契约（Stage 35）：
// 日志后处理任务被丢弃时，行不得停在建行值 processing——正文丢失是关机窄窗口的
// 共识代价，但「没记全」不能伪装成一条成功记录。痕迹文案要说清「任务被丢」，
// 与 processer 解析失败的 "processer error" 区分开。
func TestMarkLogUnfinished_WritesFailureTrace(t *testing.T) {
	initChatRecordTestDB(t)

	logRow := testutil.NewRow(t, models.DB, models.ChatLog{Name: "m", ProviderName: "p", Status: "success"})
	MarkLogUnfinished(context.Background(), logRow.ID, time.Now().Add(-200*time.Millisecond))

	var got models.ChatLog
	if err := models.DB.First(&got, logRow.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "error" {
		t.Fatalf("Status = %q, want error（未能写完必须显式留痕）", got.Status)
	}
	if !strings.Contains(got.Error, "discarded") {
		t.Fatalf("Error = %q, want 含 discarded（说清是任务被丢而非解析失败）", got.Error)
	}
	if strings.Contains(got.Error, "processer error") {
		t.Fatalf("Error = %q, 不得复用 processer 失败文案（会误导排障方向）", got.Error)
	}
	if got.ProxyTime < 150*time.Millisecond {
		t.Fatalf("ProxyTime = %v, want ≥150ms（丢弃时刻的端到端耗时回填）", got.ProxyTime)
	}
}
