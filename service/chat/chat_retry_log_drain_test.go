package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/models"
)

// TestRecordRetryLogDrainedOnShutdown 验证优雅关闭能排空重试日志：
// runProviderRetryLoop 把 RecordRetryLog 登记到 bgtask，进程关闭时 Shutdown 必须等它
// 把通道剩余元素落库。若退回裸 `go RecordRetryLog(...)`，Shutdown 不等待，落库会被进程
// 退出硬切。（Stage 24 起该 goroutine 只做留痕，权重/优先级衰减在产出侧结算。）
func TestRecordRetryLogDrainedOnShutdown(t *testing.T) {
	initChatRecordTestDB(t)

	mgr := bgtask.NewManager()
	bgtask.SetDefault(mgr)
	t.Cleanup(func() { bgtask.SetDefault(bgtask.NewManager()) })

	retryLog := make(chan models.ChatLog, 1)
	bgtask.Go(func(ctx context.Context) { RecordRetryLog(ctx, retryLog) })

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

// TestRunProviderRetryLoop_LogTaskDiscardedClosesChannelAndAborts 丢弃窗口回归
// （Stage 35）：bgtask 拒绝登记（关机收尾段）时，重试循环必须当场收手——通道随
// 丢弃分支关闭（防悬垂）、不发起任何尝试、快速返回 aborted。此前无兜底：
// RecordRetryLog 被丢后 retryLog 无人消费，循环继续投递，缓冲一满 send 即挂死。
func TestRunProviderRetryLoop_LogTaskDiscardedClosesChannelAndAborts(t *testing.T) {
	initChatRecordTestDB(t)

	mgr := bgtask.NewManager()
	bgtask.SetDefault(mgr)
	t.Cleanup(func() { bgtask.SetDefault(bgtask.NewManager()) })

	// 先关 Manager，让 runProviderRetryLoop 的登记落入丢弃窗口。
	shutCtx, cancelShut := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelShut()
	if err := mgr.Shutdown(shutCtx); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}

	done := make(chan retryLoopOutcome, 1)
	go func() {
		done <- runProviderRetryLoop(retryLoopInput{
			Ctx:      context.Background(),
			Start:    time.Now(),
			MaxRetry: 2,
		})
	}()

	select {
	case out := <-done:
		if out.Status != retryLoopAborted {
			t.Fatalf("Status = %v, want retryLoopAborted（丢弃窗口不得发起尝试）", out.Status)
		}
		if !errors.Is(out.Err, errRetryLogTaskDiscarded) {
			t.Fatalf("Err = %v, want errRetryLogTaskDiscarded", out.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("丢弃窗口里重试循环被挂住（无人消费的 retryLog 阻塞 send 的同型死法）")
	}
}
