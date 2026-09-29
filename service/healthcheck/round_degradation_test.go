package healthcheck

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/qkf688/llmux/models"
)

// 本文件锁 Stage 36 的两条「整轮降级」口径：
//   ① 本轮范围开关读不出来 → 这轮干脆不测（不按默认值把巡检范围放大到全量）；
//   ② 巡检循环里总开关读失败 → 不停机（停机是持久副作用，没有任何机制把它拉回来）。

// TestTickOnce_EnabledSwitchUnreadable_KeepsRunning 读失败不得把自己停掉。
func TestTickOnce_EnabledSwitchUnreadable_KeepsRunning(t *testing.T) {
	initHealthCheckTestDB(t)
	if err := models.DB.Exec("DROP TABLE settings").Error; err != nil {
		t.Fatalf("drop settings table: %v", err)
	}
	logs := captureWarnLogs(t)

	stop := (&HealthChecker{}).tickOnce(context.Background())

	if stop {
		t.Error("tickOnce = stop，want 继续跑（读失败退默认值停机 = 健康检测永久消失）")
	}
	if !strings.Contains(logs.String(), models.SettingKeyHealthCheckEnabled) {
		t.Errorf("读失败必须留痕并带键名:\n%s", logs.String())
	}
}

// TestTickOnce_Disabled_Stops 用户真的关掉时必须停（口径不能被上一条带松）。
func TestTickOnce_Disabled_Stops(t *testing.T) {
	initHealthCheckTestDB(t)
	if err := repos().Setting.SetBool(context.Background(), models.SettingKeyHealthCheckEnabled, false); err != nil {
		t.Fatalf("disable health check: %v", err)
	}

	if !(&HealthChecker{}).tickOnce(context.Background()) {
		t.Error("tickOnce = 继续跑，want 停止（用户关掉了健康检测）")
	}
}

// TestCheckAll_ScopeSwitchUnreadable_SkipsRoundWithoutProbing 本轮范围读不出来 → 整轮不测：
// 按默认「关」处理会把「只测已停用的」翻成「全量测」，对全部关联发真实探测请求（按量计费）。
func TestCheckAll_ScopeSwitchUnreadable_SkipsRoundWithoutProbing(t *testing.T) {
	h, mgr := newTestChecker(t)
	defer mgr.Shutdown(context.Background())
	cipher := setupHealthCheckCipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(chatHandlerByAuth(map[string]struct{}{"Bearer sk-1": {}}, &hits, nil))
	t.Cleanup(srv.Close)

	provider := models.Provider{Name: "p-scope", Type: "openai", Config: `{}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	seedEndpoint(t, provider.ID, "openai", srv.URL)
	seedInlineGroup(t, cipher, provider.ID, "g1", "sk-1")
	mp := seedModelAndAssociation(t, provider.ID)

	if err := models.DB.Exec("DROP TABLE settings").Error; err != nil {
		t.Fatalf("drop settings table: %v", err)
	}
	logs := captureWarnLogs(t)

	h.checkAll(context.Background())

	if hits.Load() != 0 {
		t.Errorf("upstream hits = %d, want 0（配置读不出来时不得扩大巡检范围）", hits.Load())
	}
	var count int64
	if err := models.DB.Model(&models.HealthCheckLog{}).Where("model_provider_id = ?", mp.ID).Count(&count).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	if count != 0 {
		t.Errorf("健康检查日志条数 = %d, want 0（这轮应整轮不测）", count)
	}
	if !strings.Contains(logs.String(), models.SettingKeyHealthCheckCheckDisabledOnly) {
		t.Errorf("整轮跳过必须留痕并带键名:\n%s", logs.String())
	}
}

// TestValidateRoundConfig_UnreadableSettings_Fails 启动前同步探配置（Ruling 1）：
// 手动「全部检测」是先返回批次号、干活在后台，读失败必须在启动前就被挡下，
// 否则前端轮询一个永远不完成的批次（无超时）。
func TestValidateRoundConfig_UnreadableSettings_Fails(t *testing.T) {
	initHealthCheckTestDB(t)
	if err := models.DB.Exec("DROP TABLE settings").Error; err != nil {
		t.Fatalf("drop settings table: %v", err)
	}
	_ = captureWarnLogs(t)

	err := (&HealthChecker{}).ValidateRoundConfig(context.Background())
	if err == nil {
		t.Fatal("ValidateRoundConfig = nil, want 报错（配置读不出来不得启动整轮检测）")
	}
}

// TestValidateRoundConfig_ReadableSettings_Passes 反向：配置可读时必须放行。
func TestValidateRoundConfig_ReadableSettings_Passes(t *testing.T) {
	initHealthCheckTestDB(t)
	_ = slog.Default()

	if err := (&HealthChecker{}).ValidateRoundConfig(context.Background()); err != nil {
		t.Fatalf("ValidateRoundConfig: %v, want nil", err)
	}
}
