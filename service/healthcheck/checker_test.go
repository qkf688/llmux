package healthcheck

import (
	"context"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/models"
)

// newTestChecker 准备一个启用了健康检测的独立 checker，并把 bgtask 默认实例换成
// 用例私有的 Manager，使 Shutdown 只等本用例登记的 goroutine。
func newTestChecker(t *testing.T) (*HealthChecker, *bgtask.Manager) {
	t.Helper()
	initHealthCheckTestDB(t)

	if err := repos().Setting.SetBool(context.Background(), models.SettingKeyHealthCheckEnabled, true); err != nil {
		t.Fatalf("enable health check: %v", err)
	}

	mgr := bgtask.NewManager()
	bgtask.SetDefault(mgr)
	t.Cleanup(func() { bgtask.SetDefault(bgtask.NewManager()) })

	return &HealthChecker{}, mgr
}

// waitDrained 等后台任务排空。超时即说明 run 循环没能随 ctx 退出。
func waitDrained(t *testing.T, mgr *bgtask.Manager) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := mgr.Shutdown(ctx); err != nil {
		t.Fatalf("checker goroutine did not exit: %v", err)
	}
}

// TestHealthCheckerExitsOnProcessCtxCancel 进程级 ctx 取消后检测循环必须退出，
// 且退出是**可等待**的——这是优雅关闭能排空健康检测写库的前提。
func TestHealthCheckerExitsOnProcessCtxCancel(t *testing.T) {
	h, mgr := newTestChecker(t)

	ctx, cancel := context.WithCancel(context.Background())
	h.Start(ctx)
	if !h.IsRunning() {
		t.Fatal("checker not running after Start")
	}

	cancel()
	waitDrained(t, mgr)
}

// TestHealthCheckerRestartKeepsProcessCtx 回归用例：Restart 曾接收调用方传入的 ctx，
// 而 handler 传的是 context.Background()，导致重启过一次的 checker 脱离进程级取消、
// 优雅关闭再也等不到它。现在 Restart 必须沿用 Start 记录的 baseCtx。
func TestHealthCheckerRestartKeepsProcessCtx(t *testing.T) {
	h, mgr := newTestChecker(t)

	ctx, cancel := context.WithCancel(context.Background())
	h.Start(ctx)
	h.Restart()
	if !h.IsRunning() {
		t.Fatal("checker not running after Restart")
	}

	cancel()
	waitDrained(t, mgr)
}

// TestHealthCheckerRestartWithoutStart 从未 Start 就 Restart 时没有 baseCtx 可继承，
// 必须拒绝启动而非退回 context.Background() 静默脱管。
func TestHealthCheckerRestartWithoutStart(t *testing.T) {
	h, mgr := newTestChecker(t)

	h.Restart()
	if h.IsRunning() {
		t.Fatal("checker started without a process ctx")
	}
	waitDrained(t, mgr)
}
