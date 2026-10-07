package chat

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
)

// TestRecordLog_BgtaskShutdownDrainsWriteback 关机语义回归（Stage 30 AC-6）：RecordLog
// 经 bgtask 登记后，「关机时正在写的请求记录不丢」由登记本排空接替原 WaitGroup 丝线
// ——bgtask.Shutdown 返回之前 RecordLog 必须已写完落库（main 关闭序里排空先于关库）。
// 原口径对照：裸 go + handler 自等的两层接力，早退路径会把失败留痕丢在建行占位上。
func TestRecordLog_BgtaskShutdownDrainsWriteback(t *testing.T) {
	initChatRecordTestDB(t)

	logRow := testutil.NewRow(t, models.DB, models.ChatLog{Name: "m", ProviderModel: "pm", ProviderName: "p", Status: "success"})

	// processer 阻塞到放行：让 Shutdown 撞上「正在写」状态，验证排空等待而非甩手。
	release := make(chan struct{})
	proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error) {
		<-release
		return nil, nil, errors.New("parse boom") // 走失败留痕写库路径（updateChatLogByID）
	}

	mgr := bgtask.NewManager()
	mgr.Go(func(ctx context.Context) {
		RecordLog(ctx, RecordLogInput{
			ReqStart:     time.Now(),
			Reader:       io.NopCloser(strings.NewReader("x")),
			Processer:    proc,
			LogID:        logRow.ID,
			Before:       Before{Stream: false},
			ProviderName: "p",
		})
	})

	// Shutdown 与在写状态撞上：先断言它在等，再放行。
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- mgr.Shutdown(context.Background()) }()
	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown 提前返回（err=%v），未等在写记录收尾", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)

	if err := <-shutdownDone; err != nil {
		t.Fatalf("Shutdown err = %v, want 排空成功", err)
	}
	// Shutdown 返回即 RecordLog 已写完（同步断言，无轮询）。
	var got models.ChatLog
	if err := models.DB.First(&got, logRow.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "error" {
		t.Fatalf("chat_logs Status = %q, want error（排空必须等 RecordLog 写完落库）", got.Status)
	}
}
