package v1

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service"
)

// readBodyWithTimeout 读完 r 并带挂死保险：丢弃窗口的死法就是「写端永远读不完」，
// 回归必须在超时里显式变红，而不是把测试挂住。
func readBodyWithTimeout(t *testing.T, r io.Reader) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	select {
	case s := <-done:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("读响应体被挂住（丢弃窗口 tee 写端阻塞的同型死法）")
		return ""
	}
}

// TestAttachLogRecord_DiscardedBypassesPipeAndMarksUnfinished 丢弃窗口回归（Stage 35）：
// bgtask 拒绝登记时必须当场放手——响应体直通客户端（不得 tee 进无人消费的管道，
// 那会把请求挂死到进程退出）、管道关闭、该行日志留「未能写完」失败痕迹。
func TestAttachLogRecord_DiscardedBypassesPipeAndMarksUnfinished(t *testing.T) {
	testsupport.InitTestDB(t)

	logRow := testutil.NewRow(t, models.DB, models.ChatLog{Name: "m", Status: "success"})

	mgr := bgtask.NewManager()
	bgtask.SetDefault(mgr)
	t.Cleanup(func() { bgtask.SetDefault(bgtask.NewManager()) })
	shutCtx, cancelShut := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelShut()
	if err := mgr.Shutdown(shutCtx); err != nil {
		t.Fatalf("Shutdown() = %v, want nil", err)
	}

	proc := func(context.Context, io.Reader, bool, time.Time, bool, bool) (*models.ChatLog, *models.OutputUnion, error) {
		t.Error("被丢弃的任务不得执行 RecordLog")
		return nil, nil, nil
	}
	reqStart := time.Now().Add(-200 * time.Millisecond)
	body, finishLog := attachLogRecord(t.Context(), service.RecordLogInput{
		ReqStart:  reqStart,
		Processer: proc,
		LogID:     logRow.ID,
	}, strings.NewReader("payload"))

	if got := readBodyWithTimeout(t, body); got != "payload" {
		t.Fatalf("响应体 = %q, want %q（直通不得丢字节）", got, "payload")
	}
	finishLog(nil) // 丢弃路径是空操作：调用方收尾姿势不分支

	var row models.ChatLog
	if err := models.DB.First(&row, logRow.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if row.Status != "error" || !strings.Contains(row.Error, "discarded") {
		t.Fatalf("日志行 = %q/%q, want error/含 discarded（未能写完必须显式留痕）", row.Status, row.Error)
	}
}

// TestAttachLogRecord_RegisteredTeesBodyToLogPipe 正常路径回归：登记成功时响应体
// 同时流向客户端与日志管道（tee），finishLog(nil) 以 EOF 收尾、finishLog(err) 把
// 断流错误传给日志侧。两条路径的收尾姿势必须一致（同一 finishLog 出口）。
func TestAttachLogRecord_RegisteredTeesBodyToLogPipe(t *testing.T) {
	testsupport.InitTestDB(t)

	saw := make(chan string, 1)
	proc := func(_ context.Context, pr io.Reader, _ bool, _ time.Time, _, _ bool) (*models.ChatLog, *models.OutputUnion, error) {
		b, _ := io.ReadAll(pr)
		saw <- string(b)
		return &models.ChatLog{}, &models.OutputUnion{}, nil
	}
	body, finishLog := attachLogRecord(t.Context(), service.RecordLogInput{
		ReqStart:  time.Now(),
		Processer: proc,
	}, strings.NewReader("payload"))

	if got := readBodyWithTimeout(t, body); got != "payload" {
		t.Fatalf("响应体 = %q, want %q", got, "payload")
	}
	finishLog(nil)

	select {
	case s := <-saw:
		if s != "payload" {
			t.Fatalf("日志侧读到 %q, want %q（tee 丢字节）", s, "payload")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("日志侧没读到 tee 的响应体（管道收尾没送达 EOF）")
	}
}
