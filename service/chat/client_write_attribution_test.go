package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

// writeTimeoutLikeError / newWriteCutoffErr 造一个形状与生产实测一致的「写回被写上限
// 掐断」错误：*net.OpError 且 Timeout()==true（socket deadline 到点时 net/http 就是
// 这么报的）。
//
// 用它而不是随手造个 error，正是因为它与「上游等超时」的形状**同形**——类型层分不开
// 方向（这正是要靠错误产生处的方向标记的理由，client_write_error.go）。若造一个普通
// error，下面的断言会因 isClientDisconnectErr 恰好判对而假绿。
type writeTimeoutLikeError struct{}

func (writeTimeoutLikeError) Error() string {
	return "write tcp 127.0.0.1:1234->127.0.0.1:7070: i/o timeout"
}
func (writeTimeoutLikeError) Timeout() bool { return true }
func (writeTimeoutLikeError) Temporary() bool {
	return true
}

func newWriteCutoffErr() error {
	return &net.OpError{Op: "write", Net: "tcp", Err: writeTimeoutLikeError{}}
}

// expiredCtx 造一个「总时限已耗尽」的 ctx（Err()==DeadlineExceeded），供打标判据与
// 提交后失败矩阵共用同一形状，避免各用例各造一遍而形状漂移。
func expiredCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	t.Cleanup(cancel)
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("前置条件不成立：过期 ctx 的 Err() = %v，want DeadlineExceeded", ctx.Err())
	}
	return ctx
}

// providerFailureCount 读该供应商的累计失败计数；没有行 = 一次都没记过（=0）。
func providerFailureCount(t *testing.T, providerName string) int64 {
	t.Helper()
	var row models.StatsProviderTotal
	err := models.DB.Where("provider_name = ?", providerName).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0
	}
	if err != nil {
		t.Fatalf("读供应商统计失败: %v", err)
	}
	return row.FailureCount
}

// TestMarkResponseWriteError_UpstreamPassthrough 锁方向标记的「上游优先」判据：nil 不包；
// **已带上游标记的读错误原样放过**（io.Copy 的错误分不清是读 src 失败还是写 dst 失败，
// 盖上写侧标记会把「上游腰斩」误判成「客户端不收」，粘性钉定该清的不清、供应商白背锅）；
// 其余才盖写侧标记，且 errors.Is/As 仍能触达内层（诊断能力不被标记层吞掉）。
func TestMarkResponseWriteError_UpstreamPassthrough(t *testing.T) {
	// 总时限未到：普通写错误该被打上写侧标记。
	ctx := context.Background()
	inner := errors.New("boom")
	// 已带上游标记的读错误：io.Copy 分不清读 src 失败还是写 dst 失败，生产里它必须
	// 原样穿过 MarkResponseWriteError（否则上游腰斩会被当成客户端不收）。
	upstreamMarked := fmt.Errorf("write response body failed: %w", upstreamReadErr{err: inner})

	if got := MarkResponseWriteError(ctx, nil); got != nil {
		t.Fatalf("MarkResponseWriteError(nil) = %v, want nil", got)
	}

	marked := MarkResponseWriteError(ctx, inner)
	if !IsClientWriteErr(marked) {
		t.Fatal("普通写错误未被打上写侧标记")
	}
	if !errors.Is(marked, inner) {
		t.Fatal("打标后 errors.Is 触达不到内层错误（诊断能力被标记层吞掉）")
	}

	// 断言必须打在**返回值**上：IsUpstreamReadErr 走 errors.As，能穿过 clientWriteErr 的
	// Unwrap 摸到内层的 upstreamReadErr，故「包没包」这一条两种实现都成立；查入参则
	// 与 MarkResponseWriteError 的行为完全无关，删掉它的「原样放过」保护也不会红。
	gotUp := MarkResponseWriteError(ctx, upstreamMarked)
	if !IsUpstreamReadErr(gotUp) {
		t.Fatal("上游读错误被写侧标记覆盖，上游方向丢失")
	}
	if IsClientWriteErr(gotUp) {
		t.Fatal("上游读错误被判成写客户端错误")
	}
}

// TestMarkResponseWriteError_BudgetExhausted_NotMarked 锁 2026-10-04 拍板口径：总时限
// 已耗尽时**不盖**写侧标记，保留既有的「超时归供应商」记账。
//
// 背景：写上限钉的就是 ctx 的总时限，预算到点那一刻 socket deadline 与 ctx deadline
// 同时触发。若不加这一条判据，「预算被烧完」会一律被当成「客户端不收」而不记供应商
// 失败——与手册「等上游答到点而超时 = 慢供应商，照常结算」直接矛盾。
//
// 只认 DeadlineExceeded、**不认 Canceled**：客户端中途断开是 ctx 被取消，那确实是
// 客户端侧，仍须打标（否则供应商要为客户端跑路背锅）。
//
// 变异自检验证——删掉 ctx.Err() 这条判据：本用例两条断言全红。
func TestMarkResponseWriteError_BudgetExhausted_NotMarked(t *testing.T) {
	exhausted := expiredCtx(t)
	// 用同一个哨兵实例比对：*net.OpError 按指针身份比 errors.Is，每次 new 一个比不了。
	cutoff := newWriteCutoffErr()

	got := MarkResponseWriteError(exhausted, cutoff)
	if IsClientWriteErr(got) {
		t.Fatal("总时限已耗尽仍被打上写侧标记（会把预算烧完算成客户端不收）")
	}
	if got != cutoff {
		t.Fatalf("不盖标记时必须原样返回同一个错误，实际 %v（want %v）", got, cutoff)
	}
	// 形状必须仍是超时类：判定层 isClientDisconnectErr 明确排除超时，靠它不被误判成
	// 断连，才落回「超时归供应商」。
	var netErr net.Error
	if !errors.As(got, &netErr) || !netErr.Timeout() {
		t.Fatalf("原样返回后丢了超时形态：%v", got)
	}

	// 客户端中途断开（ctx 被 Canceled，非 DeadlineExceeded）仍须打标。
	cancelled, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if !IsClientWriteErr(MarkResponseWriteError(cancelled, newWriteCutoffErr())) {
		t.Fatal("客户端断开（ctx=Canceled）被当成了非客户端侧")
	}
}

// TestMarkResponseWriteError_LogPipeTeardown_NotMarked 锁 tee 收尾这一格：
// handler/v1/chat.go 的 io.TeeReader 刻意在方向标记**之外**（管道写失败不是上游腰斩），
// 故日志侧提前收尾时 pw.Write 抛的 io.ErrClosedPipe 是**没有方向标记的第三种失败源**。
// 旧口径下它被兜成「客户端」，等于把「上游吐了让解析器提前退出的内容」记成客户端不收。
//
// 变异自检验证——删掉 ErrClosedPipe 这条判据：本用例红。
func TestMarkResponseWriteError_LogPipeTeardown_NotMarked(t *testing.T) {
	ctx := context.Background()
	// 真实形状是 fmt.Errorf 包过 io.ErrClosedPipe（TeeReader.Read 从写 pw 的失败里冒出来）。
	teeErr := fmt.Errorf("write response body failed: %w", io.ErrClosedPipe)

	got := MarkResponseWriteError(ctx, teeErr)
	if IsClientWriteErr(got) {
		t.Fatal("日志管道收尾的 io.ErrClosedPipe 被当成客户端侧（诱因通常在上游内容）")
	}
	if !errors.Is(got, io.ErrClosedPipe) {
		t.Fatal("原样返回后丢了 io.ErrClosedPipe，错误链被改动了")
	}
}

// TestRecordLog_ClientWriteCutoff_SkipsProviderFailureStat 锁 Stage 39 R3 的统计面：
// 带**写侧标记**的错误不记供应商失败；上游腰斩、客户端断连、以及 2026-10-04 拍板的
// 两格（总时限已耗尽 / 日志管道提前收尾）照旧记。
//
// 变异自检验证——去掉 `if !IsClientWriteErr(err)` 这一层（= 改动前）：
// 第一条用例 failure_count 由 0 变 1 → 红。
func TestRecordLog_ClientWriteCutoff_SkipsProviderFailureStat(t *testing.T) {
	// 标记的判定只在 MarkResponseWriteError 内部，故此处都用「总时限未到」的活 ctx 造
	// 标记（写侧标记要成立得过打标函数这一关，两格例外见后两条用例）。
	live := context.Background()
	exhausted, cancelExhausted := context.WithDeadline(live, time.Now().Add(-time.Second))
	defer cancelExhausted()

	cases := []struct {
		name        string
		procErr     error
		wantFailure int64
		why         string
	}{
		{
			name:        "写回被掐(客户端不收结果)→不记供应商失败",
			procErr:     fmt.Errorf("write response body failed: %w", MarkResponseWriteError(live, newWriteCutoffErr())),
			wantFailure: 0,
			why:         "供应商把响应完整产出了，记失败会污染面板上的供应商成功率",
		},
		{
			name:        "上游腰斩→照旧记一次失败",
			procErr:     fmt.Errorf("write response body failed: %w", io.ErrUnexpectedEOF),
			wantFailure: 1,
			why:         "供应商侧真故障，既有口径",
		},
		{
			name:        "客户端断连→照旧记一次失败",
			procErr:     fmt.Errorf("write response body failed: %w", context.Canceled),
			wantFailure: 1,
			why:         "既有口径（Stage 24/33），本次刻意不动；与写被掐的差异已登记 find-issues",
		},
		{
			name:        "总时限已耗尽→照旧记一次失败",
			procErr:     fmt.Errorf("write response body failed: %w", MarkResponseWriteError(exhausted, newWriteCutoffErr())),
			wantFailure: 1,
			why:         "2026-10-04 拍板：预算烧完算供应商的，与手册「超时归慢供应商」一致",
		},
		{
			name:        "日志管道提前收尾→照旧记一次失败",
			procErr:     fmt.Errorf("write response body failed: %w", MarkResponseWriteError(live, io.ErrClosedPipe)),
			wantFailure: 1,
			why:         "2026-10-04 拍板：tee 收尾的诱因通常在上游内容，不算客户端侧",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initChatRecordTestDB(t)

			RecordLog(context.Background(), RecordLogInput{
				ReqStart:     time.Now(),
				Reader:       io.NopCloser(strings.NewReader("x")),
				Processer:    failingProcesser(tc.procErr),
				LogID:        0,
				ProviderName: "p",
			})

			if got := providerFailureCount(t, "p"); got != tc.wantFailure {
				t.Fatalf("供应商失败计数 = %d, want %d（%s）", got, tc.wantFailure, tc.why)
			}
		})
	}
}

// captureSlogOutput 捕获 slog 默认 handler 的输出（留痕级别断言用）。
func captureSlogOutput(t *testing.T) *strings.Builder {
	t.Helper()
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &logs
}

// TestRecordLog_ClientWriteCutoff_LogsAtWarn 锁 Stage 18 的级别口径：客户端侧故障按
// ERROR 计数会把告警刷爆（「建连即断」同族）。带写侧标记的是客户端不收结果 → Warn；
// 上游真故障、以及 2026-10-04 拍板的两格（总时限已耗尽 / 日志管道提前收尾）仍 ERROR。
//
// 变异自检验证——把 `clientSideFailure(err)` 改回 `isClientDisconnectErr(err)`
// （= 改动前）：写超时不算断连 → 落 ERROR 分支 → 第一条用例红。
func TestRecordLog_ClientWriteCutoff_LogsAtWarn(t *testing.T) {
	live := context.Background()
	exhausted, cancelExhausted := context.WithDeadline(live, time.Now().Add(-time.Second))
	defer cancelExhausted()

	cases := []struct {
		name      string
		procErr   error
		wantLevel string
	}{
		{
			name:      "写回被掐→Warn",
			procErr:   fmt.Errorf("write response body failed: %w", MarkResponseWriteError(live, newWriteCutoffErr())),
			wantLevel: "level=WARN",
		},
		{
			name:      "上游腰斩→ERROR",
			procErr:   fmt.Errorf("write response body failed: %w", io.ErrUnexpectedEOF),
			wantLevel: "level=ERROR",
		},
		{
			name:      "总时限已耗尽→ERROR",
			procErr:   fmt.Errorf("write response body failed: %w", MarkResponseWriteError(exhausted, newWriteCutoffErr())),
			wantLevel: "level=ERROR",
		},
		{
			// 这一格的状态是**半实现**：写侧标记已按 2026-10-04 拍板不打（故上面
			// SkipsProviderFailureStat 矩阵里它记到了供应商失败），但清钉与留痕级别两处
			// 仍落客户端侧——chat_record.go 的 isClientDisconnectErr 自 Stage 24
			// （commit 4d1264a「断连类降 Warn」）起就把 io.ErrClosedPipe 直接算断连，
			// 与本 Stage 的标记是两条独立判据。改那一行等于动 Stage 24 的既有口径，
			// 待用户拍板，登记在 .local/find-issues。
			name:      "日志管道提前收尾→当前仍 Warn（待拍板）",
			procErr:   fmt.Errorf("write response body failed: %w", MarkResponseWriteError(live, io.ErrClosedPipe)),
			wantLevel: "level=WARN",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initChatRecordTestDB(t)
			logs := captureSlogOutput(t)

			RecordLog(context.Background(), RecordLogInput{
				ReqStart:     time.Now(),
				Reader:       io.NopCloser(strings.NewReader("x")),
				Processer:    failingProcesser(tc.procErr),
				LogID:        0,
				ProviderName: "p",
			})

			got := logs.String()
			if !strings.Contains(got, tc.wantLevel) {
				t.Fatalf("留痕级别不含 %s，实际输出：\n%s", tc.wantLevel, got)
			}
			// 反向断言：客户端侧失败不得同时留下 ERROR——一次故障一条痕，双份留痕会让
			// 按行计数的告警把一次故障算成两次（framework-conventions §5 边界③）。
			if tc.wantLevel == "level=WARN" && strings.Contains(got, "level=ERROR") {
				t.Fatalf("客户端侧失败留下了 ERROR 留痕：\n%s", got)
			}
		})
	}
}
