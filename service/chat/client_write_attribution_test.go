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

// TestMarkResponseWriteError_三态 锁方向标记本身的语义：nil 不包；**已带上游标记的读
// 错误原样放过**（io.Copy 的错误分不清是读 src 失败还是写 dst 失败，盖上写侧标记会把
// 「上游腰斩」误判成「客户端不收」，粘性钉定该清的不清、供应商白背锅）；其余才盖写侧
// 标记，且 errors.Is/As 仍能触达内层（诊断能力不被标记层吞掉）。
func TestMarkResponseWriteError_UpstreamPassthrough(t *testing.T) {
	inner := errors.New("boom")
	// 已带上游标记的读错误：io.Copy 分不清读 src 失败还是写 dst 失败，生产里它必须
	// 原样穿过 MarkResponseWriteError（否则上游腰斩会被当成客户端不收）。
	upstreamMarked := fmt.Errorf("write response body failed: %w", upstreamReadErr{err: inner})

	if got := MarkResponseWriteError(nil); got != nil {
		t.Fatalf("MarkResponseWriteError(nil) = %v, want nil", got)
	}

	marked := MarkResponseWriteError(inner)
	if !IsClientWriteErr(marked) {
		t.Fatal("普通写错误未被打上写侧标记")
	}
	if !errors.Is(marked, inner) {
		t.Fatal("打标后 errors.Is 触达不到内层错误（诊断能力被标记层吞掉）")
	}

	// 断言必须打在**返回值**上：IsUpstreamReadErr 走 errors.As，能穿过 clientWriteErr 的
	// Unwrap 摸到内层的 upstreamReadErr，故「包没包」这一条两种实现都成立；查入参则
	// 与 MarkResponseWriteError 的行为完全无关，删掉它的「原样放过」保护也不会红。
	gotUp := MarkResponseWriteError(upstreamMarked)
	if !IsUpstreamReadErr(gotUp) {
		t.Fatal("上游读错误被写侧标记覆盖，上游方向丢失")
	}
	if IsClientWriteErr(gotUp) {
		t.Fatal("上游读错误被判成写客户端错误")
	}
}

// TestRecordLog_ClientWriteCutoff_SkipsProviderFailureStat 锁 Stage 39 R3 的统计面：
// 写回被掐（客户端不收结果）不记供应商失败；上游腰斩与客户端断连照旧记（既有口径不动）。
//
// 变异自检验证——去掉 `if !IsClientWriteErr(err)` 这一层（= 改动前）：
// 第一条用例 failure_count 由 0 变 1 → 红。
func TestRecordLog_ClientWriteCutoff_SkipsProviderFailureStat(t *testing.T) {
	cases := []struct {
		name        string
		procErr     error
		wantFailure int64
		why         string
	}{
		{
			name:        "写回被掐(客户端不收结果)→不记供应商失败",
			procErr:     fmt.Errorf("write response body failed: %w", MarkResponseWriteError(newWriteCutoffErr())),
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
// ERROR 计数会把告警刷爆（「建连即断」同族）。写回被掐是客户端不收结果，属客户端侧 →
// Warn；上游真故障仍 ERROR。
//
// 变异自检验证——把 `clientSideFailure(err)` 改回 `isClientDisconnectErr(err)`
// （= 改动前）：写超时不算断连 → 落 ERROR 分支 → 第一条用例红。
func TestRecordLog_ClientWriteCutoff_LogsAtWarn(t *testing.T) {
	cases := []struct {
		name      string
		procErr   error
		wantLevel string
	}{
		{
			name:      "写回被掐→Warn",
			procErr:   fmt.Errorf("write response body failed: %w", MarkResponseWriteError(newWriteCutoffErr())),
			wantLevel: "level=WARN",
		},
		{
			name:      "上游腰斩→ERROR",
			procErr:   fmt.Errorf("write response body failed: %w", io.ErrUnexpectedEOF),
			wantLevel: "level=ERROR",
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
