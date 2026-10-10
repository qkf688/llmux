package chat

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
)

// 本文件是 chat 包测试**唯一**的包级单例装配/收尾入口。
//
// 为什么必须有它：被测代码把收尾工作经 bgtask 登记成后台任务
// （SaveChatLog → cleanupLogsIfNeeded、runProviderRetryLoop → RecordRetryLog），
// 这些任务在请求返回**之后**才读包级单例（settingsReader / models.DB / 默认
// repository）。用例若在任务仍在飞时 restore 单例，就与任务构成数据竞争：
//
//   - `-race` 构建下 service/chat 整包 FAIL（DATA RACE 落在 SetSettingsReader /
//     models.Init 与 getLogRetentionCount / models.GetSettingIntE 之间）；
//   - 非 race 构建下不报错，但上一个用例的后台任务可能读到**下一个用例**的库与
//     配置——结果串台的假绿/假红，比报错更难查。
//
// 因此收尾顺序是硬约束，由本入口统一保证：
//
//	先排空后台任务 → 再 restore 单例 / 关库 / 归还默认 Manager
//
// 顺序反了就等于让在飞任务读到下一个用例的状态。**任何注入包级单例的用例都必须
// 走本入口**（installChatTestSettings / initChatRecordTestDB），不要手写
// t.Cleanup(func() { SetSettingsReader(nil) })。
//
// 本包禁止 t.Parallel()（见 chat_channel_support_test.go：单例是进程级全局，并行会
// 互相踩），因此下表用普通 map 即可，无需考虑并发用例。

var (
	testEnvMu sync.Mutex
	// testEnvManagers 按用例记录共用的后台任务 Manager：settings 与 DB 两个装配
	// 入口必须排空**同一个**实例，否则先执行的那个收尾只排空自己那半，另一个
	// 入口登记的任务仍在飞（故不各自 NewManager，而是首个调用者创建、共用）。
	testEnvManagers = map[*testing.T]*bgtask.Manager{}
)

// testEnvManager 返回本用例共用的后台任务 Manager，首个调用者创建并设为默认。
func testEnvManager(t *testing.T) *bgtask.Manager {
	t.Helper()
	testEnvMu.Lock()
	defer testEnvMu.Unlock()
	mgr, ok := testEnvManagers[t]
	if !ok {
		mgr = bgtask.NewManager()
		testEnvManagers[t] = mgr
		bgtask.SetDefault(mgr)
	}
	return mgr
}

// drainBgTasks 排空本用例登记的后台任务，并归还一个干净的默认 Manager。
//
// 排空失败（超时）直接判用例失败：那是有任务真卡住，属缺陷信号，不该被静默吞掉。
// 必须归还新 Manager 而不是留着已 Shutdown 的实例：Shutdown 置位后 Go 会拒绝登记，
// 后续用例会静默走进「任务被丢弃」分支（chat_retry_loop 丢弃窗口有专门的兜底语义），
// 行为被悄悄改掉。可重复调用：第二次无实例可排空，只做归还。
func drainBgTasks(t *testing.T) {
	t.Helper()
	testEnvMu.Lock()
	mgr := testEnvManagers[t]
	delete(testEnvManagers, t)
	testEnvMu.Unlock()

	if mgr != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := mgr.Shutdown(ctx); err != nil {
			t.Errorf("排空后台任务失败（有任务卡住未退出）: %v", err)
		}
	}
	bgtask.SetDefault(bgtask.NewManager())
}

// installChatTestSettings 注入包级 settingsReader 并注册收尾（先排空、再复位）。
// 取代手写的 `SetSettingsReader(x)` + `t.Cleanup(func() { SetSettingsReader(nil) })`
// 两行式——后者漏了「先排空」，正是本次竞态的成因。
func installChatTestSettings(t *testing.T, r settings.Reader) {
	t.Helper()
	testEnvManager(t)
	SetSettingsReader(r)
	t.Cleanup(func() {
		// 后台任务经 getLogRetentionCount 读的就是 settingsReader：
		// 必须先等它们跑完再复位，否则等于让任务读到复位后的默认 reader。
		drainBgTasks(t)
		SetSettingsReader(nil)
	})
}

// probeSettingsReader 记录「有没有被读过」，用于把收尾顺序错误变成确定性红灯。
// 其余方法委托给一个零值 stub：本用例只走 getLogRetentionCount 的 Int 路径。
type probeSettingsReader struct {
	settings.Reader
	hit atomic.Bool
}

func (p *probeSettingsReader) Int(_ context.Context, _ string, def, _ int) int {
	p.hit.Store(true)
	return def
}

// TestChatTestEnv_DrainsBeforeRestoringSettings 锁住 installChatTestSettings 的收尾
// 顺序。本机 Windows 跑不了 -race（缺 cgo 运行时，exit status 0xc0000139），本用例用
// 「故意慢」的后台任务把窗口拉宽，把顺序错误变成**确定性**红灯：
//
//	先排空（正确）：任务在复位前跑完，读到本用例注入的 stub → hit=true；
//	先复位（错误）：任务读到复位后的默认 reader / 还没跑完 → hit=false。
//
// 断言注册在装配**之前**，故按 Cleanup 逆序在装配收尾之后执行。
func TestChatTestEnv_DrainsBeforeRestoringSettings(t *testing.T) {
	probe := &probeSettingsReader{Reader: credCooldownSettingsReader{}}
	t.Cleanup(func() {
		if !probe.hit.Load() {
			t.Error("后台任务读到的不是本用例注入的 reader：收尾顺序反了（先复位才排空），或任务未被排空")
		}
	})
	installChatTestSettings(t, probe)

	if !bgtask.Go(func(ctx context.Context) {
		time.Sleep(50 * time.Millisecond) // 拉宽「先复位」实现的窗口，使失败确定化
		getLogRetentionCount(ctx)         // 与生产清理任务同一条读路径
	}) {
		t.Fatal("登记后台任务失败")
	}
}

// TestChatTestEnv_DrainsBeforeClosingDB 同理锁住 initChatRecordTestDB：后台任务必须在
// 库被关掉、默认 repository 被清掉之前跑完，否则会撞上已关闭的库——旧实现
// （未排空就 SetDefault(nil) + Close）下这条必红。
func TestChatTestEnv_DrainsBeforeClosingDB(t *testing.T) {
	done := make(chan error, 1)
	t.Cleanup(func() {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("后台任务撞上已关闭的库（收尾没等它就关库）: %v", err)
			}
		default:
			t.Error("后台任务未被排空：收尾没等它就关库了")
		}
	})
	initChatRecordTestDB(t)

	if !bgtask.Go(func(ctx context.Context) {
		time.Sleep(50 * time.Millisecond)
		var n int64
		done <- models.DB.Model(&models.ChatLog{}).Count(&n).Error
	}) {
		t.Fatal("登记后台任务失败")
	}
}
