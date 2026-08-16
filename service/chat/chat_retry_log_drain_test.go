package chat

import (
	"context"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/models"
)

// TestRecordRetryLogDrainedOnShutdown 验证优雅关闭能排空重试日志：
// runProviderRetryLoop 把 RecordRetryLog 登记到 bgtask，进程关闭时 Shutdown 必须等它
// 把通道剩余元素落库。若退回裸 `go RecordRetryLog(...)`，Shutdown 不等待，落库与
// 权重衰减会被进程退出硬切。
func TestRecordRetryLogDrainedOnShutdown(t *testing.T) {
	initChatRecordTestDB(t)

	mgr := bgtask.NewManager()
	bgtask.SetDefault(mgr)
	t.Cleanup(func() { bgtask.SetDefault(bgtask.NewManager()) })

	retryLog := make(chan models.ChatLog, 1)
	bgtask.Go(func(ctx context.Context) { RecordRetryLog(ctx, retryLog, nil) })

	retryLog <- models.ChatLog{Name: "m", ProviderName: "p", Status: "error", Error: "boom"}
	// 与 runProviderRetryLoop 的 defer close 对齐：关闭通道让 range 取完剩余元素后退出。
	close(retryLog)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := mgr.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}

	// Shutdown 返回即意味着排空完成，此处无需轮询等待。
	var count int64
	if err := models.DB.Model(&models.ChatLog{}).Where("error = ?", "boom").Count(&count).Error; err != nil {
		t.Fatalf("count retry logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("persisted retry logs = %d, want 1 (not drained before shutdown returned)", count)
	}
}
