package chat

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/bgtask"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/credwrite"
	"github.com/qkf688/llmux/service/settings"
)

// 本文件是 chat 包测试**唯一**的包级单例装配/收尾入口。
//
// 为什么必须有它：被测代码把收尾工作交给**两个**后台系统，它们在请求返回**之后**才读
// 包级单例（settingsReader / models.DB / 默认 repository）：
//
//  1. bgtask 任务：SaveChatLog → cleanupLogsIfNeeded、runProviderRetryLoop → RecordRetryLog；
//  2. credwrite 凭据写队列的 worker：processJob → persistCredentialCooldown 等，读设置与库。
//
// 用例若在它们仍在飞时 restore 单例，就与它们构成 data race：
//
//   - `-race` 下 service/chat 整包 FAIL（DATA RACE 落在 models.Init / SetSettingsReader
//     与 models.GetSettingIntE / getLogRetentionCount 之间）；
//   - 非 race 构建下不报错，但上一个用例的遗留工作可能读到**下一个用例**的库与配置
//     ——结果串台的假绿/假红，比报错更难查。
//
// 因此收尾顺序是硬约束，由本入口统一保证：
//
//	先排空后台工作（credwrite 队列 + bgtask）→ 再 restore 单例 / 关库 / 归还默认 Manager
//
// 两个必须记住的细节：
//
//   - `credwrite.Flush` 在**同步模式**下直接返回 nil（等于不排空），所以「先切回同步
//     模式再排空」等于白做：排空必须发生在切回之前。
//   - 长驻循环（credwrite worker）不归 `bgtask.Shutdown` 驱动（它只在 requestStop 时
//     退出）。若它懒加载在某个用例专属 Manager 上，该用例收尾的 Shutdown 只能等到
//     超时并误报「有任务卡住」——故由 TestMain 在 m.Run() 之前把它固定到初始 Manager。
//
// **任何注入包级单例的用例都必须走本入口**（installChatTestSettings /
// initChatRecordTestDB），且任何 restore 单例 / 切同步模式 / 关库的收尾都必须以
// drainBackgroundWork 开头。不要手写 t.Cleanup(func() { SetSettingsReader(nil) })。
//
// 本包禁止 t.Parallel()（见 chat_channel_support_test.go：单例是进程级全局，并行会
// 互相踩），因此下表用普通 map 即可，无需考虑并发用例。

// TestMain 在跑用例前先把 credwrite 的 worker 固定到初始 Manager 上。
//
// worker 是**进程级单例**（包级 defaultQueue + workerOnce：懒启动一次、只在 requestStop
// 时退出）。若让它懒加载在某个用例专属 Manager 上，该用例收尾的 bgtask.Shutdown 会一直
// 等这个永不退出的循环，只能等到超时并误报「有任务卡住」。提前 Start 一次即可把它钉在
// 没人 Shutdown 的初始 Manager 上，生命周期也与生产一致（进程级）；收尾再按生产的关闭序
// Stop（Flush + 停收）。
func TestMain(m *testing.M) {
	credwrite.Start()

	code := m.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := credwrite.Stop(ctx); err != nil {
		// 队列排不空说明有用例留下了卡住的写任务。此处只报告：用例自己已经红过，
		// 不必再改退出码掩盖具体是哪个用例。
		fmt.Fprintf(os.Stderr, "chat TestMain: credwrite.Stop: %v\n", err)
	}
	os.Exit(code)
}

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

// drainCredWriteQueue 排空凭据写队列。Flush 排到哨兵为止，与已入队任务的落库构成
// happens-before。**必须在切回同步模式之前调用**：同步模式下 Flush 直接返回 nil。
func drainCredWriteQueue(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := credwrite.Flush(ctx); err != nil {
		t.Errorf("排空凭据写队列失败（有写任务卡住）: %v", err)
	}
}

// drainBgTasks 排空本用例经 bgtask 登记的任务，并归还一个干净的默认 Manager。
//
// 排空失败（超时）直接判用例失败：那是有任务真卡住，属缺陷信号，不该被静默吞掉。
// 必须归还新 Manager 而不是留着已 Shutdown 的实例：Shutdown 置位后 Go 会拒绝登记，
// 后续用例会静默走进「任务被丢弃」分支（chat_retry_loop 丢弃窗口有专门的兜底语义），
// 行为被悄悄改掉。可重复调用：第二次无实例可排空，只做归还。
//
// 只覆盖 bgtask 登记的**短任务**：长驻循环（credwrite worker）不由它驱动，见 TestMain。
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

// drainBackgroundWork 排空两个会读包级单例的后台系统。**任何 restore 单例 / 关库 /
// 切回同步模式的收尾都必须以它开头**，否则在飞工作会读到下一个用例的状态。
func drainBackgroundWork(t *testing.T) {
	t.Helper()
	drainCredWriteQueue(t) // 读 settings / models.DB，且必须仍在异步模式下才有效
	drainBgTasks(t)        // 读 settingsReader / models.DB
}

// enableCredHealthSwitches 把凭据健康四个开关置为 true（2026-10-27 起这四个开关
// **默认关**——自动冷却/自动停用不默认生效，凭据不会被系统擅自停用）。
//
// 依赖这些机制生效的用例必须显式声明这个前提：否则「机制没开」与「机制坏了」
// 在断言里同形，失败时定位不到真因（本仓已因此红过一轮：切换默认值的用例
// 静默失去冷却写库，断言报「CooldownUntil = nil」而真因是开关关着）。
//
// 写 settings 表（不是 stub）：这些用例走默认读库（modelsReader），与生产读法同源。
// 行已由 models.Init 的播种建好（值为默认的 false），故这里改值而不是插入。
func enableCredHealthSwitches(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		models.SettingKeyCredHealthCooldown429Enabled,
		models.SettingKeyCredHealthCooldownServerEnabled,
		models.SettingKeyCredHealthAuthFailStopEnabled,
		models.SettingKeyCredHealthProbeEnabled,
	} {
		res := models.DB.Model(&models.Setting{}).Where("key = ?", key).Update("value", "true")
		if res.Error != nil {
			t.Fatalf("enable cred health switch %s: %v", key, res.Error)
		}
		if res.RowsAffected == 0 {
			t.Fatalf("enable cred health switch %s: 行不存在（settings 播种未建该键？）", key)
		}
	}
}

// installChatTestSettings 注入包级 settingsReader 并注册收尾（先排空、再复位）。
// 取代手写的 `SetSettingsReader(x)` + `t.Cleanup(func() { SetSettingsReader(nil) })`
// 两行式——后者漏了「先排空」，正是本次竞态的成因。
func installChatTestSettings(t *testing.T, r settings.Reader) {
	t.Helper()
	testEnvManager(t)
	SetSettingsReader(r)
	t.Cleanup(func() {
		// 后台工作经 getLogRetentionCount / 冷却设置读的就是 settingsReader：
		// 必须先等它们跑完再复位，否则等于让它们读到复位后的默认 reader。
		drainBackgroundWork(t)
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

// blockingCooldownReader 在冷却窗口设置读取处阻塞，用来把凭据写 worker 卡在任务里，
// 以便确定性断言「排空必须等它」。
type blockingCooldownReader struct {
	credCooldownSettingsReader
	release chan struct{}
	hit     atomic.Bool
}

func (r *blockingCooldownReader) IntE(ctx context.Context, key string, def, min int) (int, error) {
	// 冷却窗口读的是 IntE（getCredHealthCooldown429Sec → settingsReader.IntE），
	// 不是 Int——挂错方法会静默不阻塞，用例就永远等不到 worker 进任务。
	if key == models.SettingKeyCredHealthCooldown429Sec {
		r.hit.Store(true)
		<-r.release
	}
	return r.credCooldownSettingsReader.IntE(ctx, key, def, min)
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

// TestChatTestEnv_DrainWaitsForCredWriteQueue 锁住 drainBackgroundWork 真的排空**凭据写
// 队列**（bgtask 之外的第二个后台系统）。用只在冷却设置读取处阻塞的 reader 把 worker 卡在
// 任务里：不排空的实现会立刻返回，本用例因此必红。
func TestChatTestEnv_DrainWaitsForCredWriteQueue(t *testing.T) {
	initChatRecordTestDB(t)

	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock) // 用例中途失败也不把 worker 永久卡住（否则后续用例的 Flush 会等它）

	reader := &blockingCooldownReader{release: release}
	installChatTestSettings(t, reader)
	credwrite.EnableTestSyncMode(false)
	t.Cleanup(func() { credwrite.EnableTestSyncMode(true) })

	cred := models.Credential{Key: "k", KeyHash: "h"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	// 入队后不给用例体内任何 Flush 的机会：排空只能来自收尾路径（被测行为）。
	applyCredentialCooldown(context.Background(), cred, cooldownReason429)

	// 等 worker 真的进了该任务：此时它卡在冷却设置读取上。
	deadline := time.Now().Add(3 * time.Second)
	for !reader.hit.Load() && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if !reader.hit.Load() {
		t.Fatal("凭据写 worker 未开始处理入队任务")
	}

	drained := make(chan struct{})
	go func() {
		drainBackgroundWork(t)
		close(drained)
	}()
	select {
	case <-drained:
		t.Fatal("排空在 worker 仍卡在任务时就返回了：drainBackgroundWork 没排空凭据写队列")
	case <-time.After(80 * time.Millisecond):
	}
	unblock()
	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		t.Fatal("放行后排空仍未返回")
	}
}
