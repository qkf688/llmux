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
// 只认「到点」、**不认 Canceled**：客户端中途断开（预算未到）的 ctx 被取消，那确实是
// 客户端侧，仍须打标（否则供应商要为客户端跑路背锅）；取消与到点并存时按到点算，与
// chat_settlement.requestAbandoned 同一口径。到点判据取时刻比较（deadlineElapsed），
// 不读 ctx.Err()——标志可观测时机见 TestMarkResponseWriteError_DeadlineElapsedBeforeCancelFlag。
//
// 变异自检验证——删掉到点判据（连 Err() 兜底一起）：本用例两条断言全红。
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

// stubDeadlineCtx 把「deadline」与「取消标志」解耦，用来构造真 ctx 无法稳定复现的两种
// 组合：
//   - Err()==nil + deadline 已过：写上限到点那一刻、取消标志还没被独立 goroutine 写上；
//   - Err()==Canceled + deadline 已过：客户端断开与预算到点同刻并存。
//
// 两个信号的写入时机在生产由调度决定（socket deadline 走 netpoll 计时器，取消标志走
// time.AfterFunc 的 goroutine），真 ctx 上「到点但标志未写」只是概率出现，单测用桩把它
// 钉死才能稳定覆盖。
type stubDeadlineCtx struct {
	context.Context
	deadline time.Time
	err      error
}

func (c stubDeadlineCtx) Deadline() (time.Time, bool) { return c.deadline, true }
func (c stubDeadlineCtx) Err() error                  { return c.err }

// TestMarkResponseWriteError_DeadlineElapsedBeforeCancelFlag 锁「到点判据不依赖取消标志的
// 可观测时机」：deadline 已到点而 ctx.Err() 仍为 nil 时必须豁免（归供应商），否则负载下
// 同一现场会出现两种归属——实测约 1/120 漏豁免，被误判成客户端侧后不记供应商失败统计、
// 不清粘性钉定（该格按 2026-10-04 拍板归供应商）。
//
// 变异自检验证——把判据换回只读 ctx.Err()（= 改动前）：本用例红。
func TestMarkResponseWriteError_DeadlineElapsedBeforeCancelFlag(t *testing.T) {
	ctx := stubDeadlineCtx{Context: context.Background(), deadline: time.Now().Add(-time.Second)}
	if !deadlineElapsed(ctx) {
		t.Fatal("前置条件不成立：桩 ctx 的 deadline 未被判为已到点")
	}

	cutoff := newWriteCutoffErr()
	got := MarkResponseWriteError(ctx, cutoff)
	if IsClientWriteErr(got) {
		t.Fatal("总时限已到点（取消标志尚未可观测）仍被打上写侧标记（会把预算烧完算成客户端不收）")
	}
	if got != cutoff {
		t.Fatalf("不盖标记时必须原样返回同一个错误，实际 %v（want %v）", got, cutoff)
	}
}

// TestMarkResponseWriteError_CancelledAtDeadline_NotMarked 锁「取消与到点并存按到点算」：
// 客户端断开与预算到点同刻出现在同一现场时按超时归供应商——与结算侧口径
// （chat_settlement.requestAbandoned 用同一个 deadlineElapsed）一致，两处对该现场不再给出
// 相反归属；预算未到的纯断连仍是客户端侧（见 BudgetExhausted 用例的 cancelled 断言）。
func TestMarkResponseWriteError_CancelledAtDeadline_NotMarked(t *testing.T) {
	ctx := stubDeadlineCtx{
		Context:  context.Background(),
		deadline: time.Now().Add(-time.Second),
		err:      context.Canceled,
	}
	if !errors.Is(ctx.Err(), context.Canceled) || !deadlineElapsed(ctx) {
		t.Fatal("前置条件不成立：want 取消与到点并存")
	}
	if IsClientWriteErr(MarkResponseWriteError(ctx, newWriteCutoffErr())) {
		t.Fatal("取消与到点并存被打上写侧标记（结算侧按超时算，此处必须同口径）")
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

// TestRecordLog_PostCommitFailure_ProviderFailureStat 锁提交后失败在**统计面**的归属
// （单源判据 providerAtFault，Stage 44 拍板 2026-10-16）：供应商侧真故障记一次失败；
// 调用方撤走整族（写回被掐 / 客户端点停止断连）都不记——供应商把内容吐了一半是调用方
// 自己不要了，记它会让面板上的成功率跟着调用方的操作习惯波动。
//
// 2026-10-04 拍板的两格（总时限已耗尽 / 日志管道提前收尾）在此口径下仍记供应商侧：
// 前者是「预算烧完算慢供应商」，后者归属待取样结论（见 tee 那格的待办）。
//
// 变异自检验证——把守卫改回 `!IsClientWriteErr(err)`（= 改动前）：
// 「客户端撤走」一条 failure_count 由 0 变 1 → 红。
func TestRecordLog_PostCommitFailure_ProviderFailureStat(t *testing.T) {
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
			name:        "客户端撤走(点停止/关页面)→不记供应商失败",
			procErr:     fmt.Errorf("write response body failed: %w", context.Canceled),
			wantFailure: 0,
			why:         "2026-10-16 拍板：与写回被掐同族，同一个「调用方不要了」不再按代码路径分两种归属",
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

// consecutiveFailures 读该关联的连续失败计数（行不存在时 FATAL：本矩阵的每一格都必须
// 打在真实写出的行上，读不到行说明夹具没了，不能静默当 0 放过）。
func consecutiveFailures(t *testing.T, mwpID uint) int {
	t.Helper()
	var row models.ModelWithProvider
	if err := models.DB.First(&row, mwpID).Error; err != nil {
		t.Fatalf("读关联连续失败计数失败（id=%d）: %v", mwpID, err)
	}
	return row.ConsecutiveFailures
}

// credentialsAccountingSnapshot 把 credentials 表的账务列拍成一份可比对的文本快照
// （状态 / 冷却窗口 / 鉴权失败计数 / 终身统计）。用于断言「某条失败路径没碰凭据」——
// 只断连败列的用例挡不住将来有人在该出口顺手加冷却（Stage 44 验收明确要求两处都断）。
func credentialsAccountingSnapshot(t *testing.T) string {
	t.Helper()
	var rows []models.Credential
	if err := models.DB.Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("读 credentials 失败: %v", err)
	}
	var b strings.Builder
	for _, r := range rows {
		cooldown := "nil"
		if r.CooldownUntil != nil {
			cooldown = r.CooldownUntil.UTC().Format(time.RFC3339Nano)
		}
		fmt.Fprintf(&b, "%d|%s|%s|%d|%d|%d|%d\n", r.ID, r.Status, cooldown, r.FailCount, r.TotalRequests, r.TotalErrors, r.TotalTokens)
	}
	return b.String()
}

// TestRecordLog_PostCommitFailure_SettlesConsecutiveFailures 锁 Stage 44 拍板（2026-10-16）：
// 响应已开写后的**供应商侧**失败要计入调度账——关联连续失败 +1（它也是「连败达阈值自动
// 停用该关联」的输入），而调用方撤走整族（客户端点停止 / 写回被掐）不记。与上面的统计面
// 矩阵共用同一判据（providerAtFault），两本账不再对同一现场给出相反归属。
//
// 该分支此前是全仓唯一「没有结算调用点」的失败出口，于是「稳定腰斩的供应商永不被降权」
// （.local/find-issues/2026-10-01-提交后断流不进结算腰斩供应商永不被降权.md）。
//
// 变异自检验证：去掉 settlePostCommitFailure 调用 → 供应商侧三格由 1 变 0 红；把守卫换回
// `!IsClientWriteErr(err)`（= 改动前）→ 客户端撤走一格由 0 变 1 红。
func TestRecordLog_PostCommitFailure_SettlesConsecutiveFailures(t *testing.T) {
	live := context.Background()

	cases := []struct {
		name        string
		procErr     error
		wantSettled int
		why         string
	}{
		{
			name:        "上游腰斩→计一次连败",
			procErr:     fmt.Errorf("write response body failed: %w", io.ErrUnexpectedEOF),
			wantSettled: 1,
			why:         "上游开了流又截断是供应商侧真故障，稳定这样做的家该被降权/停用",
		},
		{
			name:        "上游RST(读侧已打标)→计一次连败",
			procErr:     fmt.Errorf("write response body failed: %w", upstreamReadErr{err: rstLikeError()}),
			wantSettled: 1,
			why:         "RST 是非超时 net.Error，类型反推会误判成客户端撤走；读侧标记是错误产生处记下的方向事实，优先",
		},
		{
			name:        "客户端撤走(点停止/关页面)→不结算",
			procErr:     fmt.Errorf("write response body failed: %w", context.Canceled),
			wantSettled: 0,
			why:         "调用方自己不要了，不该让供应商离自动停用更近一步（连点三次停止不该停掉一家）",
		},
		{
			name:        "写回被掐(客户端不收结果)→不结算",
			procErr:     fmt.Errorf("write response body failed: %w", MarkResponseWriteError(live, newWriteCutoffErr())),
			wantSettled: 0,
			why:         "与客户端撤走同族；两本账的口径必须一致，否则同一现场统计记、调度不记",
		},
		{
			name:        "日志管道提前收尾→计一次连败（与统计面同源，待取样后再定归属）",
			procErr:     fmt.Errorf("write response body failed: %w", MarkResponseWriteError(live, io.ErrClosedPipe)),
			wantSettled: 1,
			why:         "2026-10-04 拍板归供应商侧；该形态是否真能走到记账点尚待取样，故此处只锁「两本账一致」",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initChatRecordTestDB(t)
			providerRow, mwp := createAttributionFixtureProvider(t, "p-settle")
			// 带非零失败计数/冷却原因的凭据：本出口拿不到 credential，任何凭据写都属越界，
			// 且这样造让「顺手加冷却」必然改变快照（Stage 44 验收要求连凭据面一起断）。
			if err := models.DB.Create(&models.Credential{
				Key: "k-settle", KeyHash: "h-settle", Note: "settle-fixture",
				FailCount: 2, TotalErrors: 7, CooldownReason: "server_error",
			}).Error; err != nil {
				t.Fatalf("create credential: %v", err)
			}
			credentialsBefore := credentialsAccountingSnapshot(t)

			RecordLog(context.Background(), RecordLogInput{
				ReqStart:                   time.Now(),
				Reader:                     io.NopCloser(strings.NewReader("x")),
				Processer:                  failingProcesser(tc.procErr),
				LogID:                      0,
				ProviderName:               providerRow.Name,
				RealModelName:              "m1",
				ServingModelWithProviderID: mwp.ID,
			})

			if got := consecutiveFailures(t, mwp.ID); got != tc.wantSettled {
				t.Fatalf("关联连续失败计数 = %d, want %d（%s）", got, tc.wantSettled, tc.why)
			}
			if after := credentialsAccountingSnapshot(t); after != credentialsBefore {
				t.Fatalf("提交后失败出口动了凭据账务（该出口拿不到 credential，写凭据即越界——拍板明确「不进凭据冷却」）：\nbefore=%s\nafter=%s", credentialsBefore, after)
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
			// 这一格是**既有口径的不一致**（2026-10-17 独立审查提出，已登记 find-issues）：
			// 上游 RST 带读侧标记，统计面与调度面都按供应商记（`providerAtFault` 看标记），
			// 但留痕级别走 `clientSideFailure`——它经 `upstreamReadErr.Unwrap` 摸到内层非超时
			// `net.Error`，判成「调用方撤走」→ 落 Warn，而 §5 约定 ERROR = 服务器/供应商故障。
			// 锁现状不是认可它，是让「想把三格统一」的人看见代价：改这格必须同时决定
			// `io.ErrClosedPipe` 那一格（两者共用同一个终判函数），不可单方面改。
			name:      "上游RST(读侧已打标)→当前仍 Warn（既有口径，待与 tee 那格一并决策）",
			procErr:   fmt.Errorf("write response body failed: %w", upstreamReadErr{err: rstLikeError()}),
			wantLevel: "level=WARN",
		},
		{
			// 这一格的状态是**半实现**：写侧标记已按 2026-10-04 拍板不打（故上面
			// ProviderFailureStat 矩阵里它记到了供应商失败），但清钉与留痕级别两处
			// 仍落客户端侧——chat_record.go 的 isClientDisconnectErr 自 Stage 24
			// （commit 4d1264a「断连类降 Warn」）起就把 io.ErrClosedPipe 直接算断连，
			// 与写侧标记是两条独立判据（`callerWithdrawn` 刻意不含它，见该函数注释）。
			// 该形态是否真能走到这个出口尚待线上取样（Stage 44，2026-10-16 起在
			// handler/v1 插了临时取样日志，只对 ErrClosedPipe 打），取样结论出来前维持现状；
			// 登记在 .local/find-issues/2026-10-04-tee收尾…md。
			name:      "日志管道提前收尾→当前仍 Warn（待取样）",
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

// TestPostCommitAttributionPredicates_NotComplements 结构锁：把「提交后失败」两个判据在
// 各错误形态下的取值**逐格写死**。它防的是最容易被顺手做掉的一种「简化」——把
// `providerAtFault` 当成 `!clientSideFailure`（或反之），因为两判据**不是**互补关系，
// 有两格同时为真（读侧已打标的上游 RST、日志管道收尾的 io.ErrClosedPipe），各有归属理由：
// 前者是「读侧标记 vs Unwrap 后的非超时 net.Error」之争，后者归属待线上取样（Stage 44）。
// 谁把两者对齐，本表立刻红——比注释更硬。
//
// 变异自检验证：把 providerAtFault 改成 `!clientSideFailure(err)` → RST 与 ErrClosedPipe
// 两行的 providerAtFault 期望翻面，本用例红。
func TestPostCommitAttributionPredicates_NotComplements(t *testing.T) {
	live := context.Background()
	exhausted, cancelExhausted := context.WithDeadline(live, time.Now().Add(-time.Second))
	defer cancelExhausted()

	cases := []struct {
		name           string
		err            error
		wantProvider   bool
		wantClientSide bool
		why            string
	}{
		{
			name: "上游腰斩(无标记)", err: io.ErrUnexpectedEOF,
			wantProvider: true, wantClientSide: false,
			why: "互补格：供应商侧真故障",
		},
		{
			name: "上游RST(读侧已打标)", err: upstreamReadErr{err: rstLikeError()},
			wantProvider: true, wantClientSide: true,
			why: "**非互补格**：读侧标记让归属判供应商，而 Unwrap 后的非超时 net.Error 让 clientSideFailure 判调用方撤走（留痕因此落 Warn，缺口已登记 find-issues）",
		},
		{
			name: "写回被掐(写侧标记)", err: MarkResponseWriteError(live, newWriteCutoffErr()),
			wantProvider: false, wantClientSide: true,
			why: "互补格：客户端不收结果（Stage 39 R3）",
		},
		{
			name: "客户端撤走(Canceled)", err: context.Canceled,
			wantProvider: false, wantClientSide: true,
			why: "互补格：点停止/关页面（Stage 44 拍板后不再记供应商失败）",
		},
		{
			name: "总时限已耗尽", err: MarkResponseWriteError(exhausted, newWriteCutoffErr()),
			wantProvider: true, wantClientSide: false,
			why: "互补格：预算烧完算供应商（2026-10-04 拍板）",
		},
		{
			name: "日志管道提前收尾", err: io.ErrClosedPipe,
			wantProvider: true, wantClientSide: true,
			why: "**非互补格**：归属待线上取样（Stage 44 第 3 项），取样期间刻意两处不同步",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := providerAtFault(tc.err); got != tc.wantProvider {
				t.Fatalf("providerAtFault = %v, want %v（%s）", got, tc.wantProvider, tc.why)
			}
			if got := clientSideFailure(tc.err); got != tc.wantClientSide {
				t.Fatalf("clientSideFailure = %v, want %v（%s）", got, tc.wantClientSide, tc.why)
			}
		})
	}
}
