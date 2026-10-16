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
// 同时断言读到干净 EOF（err == nil）：被测 body 在生产里是
// io.TeeReader(service.MarkUpstreamReader(resBody), pw)，「正常读完被判成读失败」时
// 这里读到的错误就是 EOF（2026-10-05 终止 EOF 误判即此形，本断言即那条缺陷的组合锁）。
func readBodyWithTimeout(t *testing.T, r io.Reader) string {
	t.Helper()
	done := make(chan string, 1)
	readErr := make(chan error, 1)
	go func() {
		b, err := io.ReadAll(r)
		readErr <- err
		done <- string(b)
	}()
	select {
	case s := <-done:
		if err := <-readErr; err != nil {
			t.Fatalf("读响应体 err = %v, want nil（正常收尾不得被判成读失败）", err)
		}
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("读响应体被挂住（丢弃窗口 tee 写端阻塞的同型死法）")
		return ""
	}
}

// TestAttachLogRecord_DiscardedBypassesPipeAndMarksUnfinished 丢弃窗口回归（Stage 35，
// 分片 #3 改终态）：bgtask 拒绝登记时必须当场放手——响应体直通客户端（不得 tee 进无人
// 消费的管道，那会把请求挂死到进程退出）、管道关闭、该行日志留「未能写完」痕迹且终态为
// interrupted（关机是本机行为，不是供应商故障）。
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

	proc := func(context.Context, io.Reader, bool, time.Time) (*models.ChatLog, *models.OutputUnion, error) {
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
	if row.Status != models.ChatLogStatusInterrupted || !strings.Contains(row.Error, "discarded") {
		t.Fatalf("日志行 = %q/%q, want interrupted/含 discarded（未能写完必须显式留痕，且不归因到供应商）", row.Status, row.Error)
	}
}

// TestAttachLogRecord_DiscardedReaderKeepsUpstreamDirection 丢弃窗口的响应体必须与登记
// 分支**同形**：上游读错要带读侧方向标记，否则 writeResponseBody 的 MarkResponseWriteError
// 会把它兜成「客户端不收结果」，方向整个反过来（2026-10-05 code-review S2 提出的正是这
// 一行）。本用例直接复现那个兜底：读丢弃分支返回的 reader 拿到上游读错，再喂给
// MarkResponseWriteError，断言它**不**被盖上写侧标记。
//
// 现实边界（别误读本用例的强度）：丢弃分支的 finishLog 是空操作、且 RecordLog 任务根本
// 没建，所以这个错误当前**没有消费者**——标记的作用是把「两侧同形」这条不变量钉住，将来
// 该错误获得消费者（例如丢弃窗口的失败也要留痕）时方向即刻是对的，而不是等那时再发现反了。
//
// 变异自检验证——把丢弃分支改回直接 `return resBody`：本用例红。
func TestAttachLogRecord_DiscardedReaderKeepsUpstreamDirection(t *testing.T) {
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

	proc := func(context.Context, io.Reader, bool, time.Time) (*models.ChatLog, *models.OutputUnion, error) {
		t.Error("被丢弃的任务不得执行 RecordLog")
		return nil, nil, nil
	}
	// 上游中途截断的真实形态（读侧打标只对「非身份 EOF」生效，故用 ErrUnexpectedEOF 而不是 EOF）。
	body, finishLog := attachLogRecord(t.Context(), service.RecordLogInput{
		ReqStart:  time.Now(),
		Processer: proc,
		LogID:     logRow.ID,
	}, errReader{err: io.ErrUnexpectedEOF})

	_, readErr := io.ReadAll(body)
	if readErr == nil {
		t.Fatal("丢弃窗口的响应体没有把上游读错透出来，本用例失去意义")
	}
	finishLog(readErr) // 丢弃路径是空操作：调用方收尾姿势不分支

	if service.IsClientWriteError(service.MarkResponseWriteError(t.Context(), readErr)) {
		t.Fatalf("丢弃窗口的上游读错被兜成「客户端不收结果」：%v（方向反了——该清的钉会不清、供应商失败会被漏记）", readErr)
	}
}

// TestAttachLogRecord_RegisteredTeesBodyToLogPipe 正常路径回归：登记成功时响应体
// 同时流向客户端与日志管道（tee），finishLog(nil) 以 EOF 收尾、finishLog(err) 把
// 断流错误传给日志侧。两条路径的收尾姿势必须一致（同一 finishLog 出口）。
func TestAttachLogRecord_RegisteredTeesBodyToLogPipe(t *testing.T) {
	testsupport.InitTestDB(t)

	saw := make(chan string, 1)
	proc := func(_ context.Context, pr io.Reader, _ bool, _ time.Time) (*models.ChatLog, *models.OutputUnion, error) {
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
