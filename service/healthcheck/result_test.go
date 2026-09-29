package healthcheck

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/settings"
)

func initHealthCheckTestDB(t *testing.T) {
	t.Helper()
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmux-test.db"))
	// 与 models.DB 同步默认 Repositories，避免 Default 缓存上一个用例的连接
	repository.SetDefault(repository.New(models.DB))
	t.Cleanup(func() {
		repository.SetDefault(nil)
		sqlDB, err := models.DB.DB()
		if err != nil {
			return
		}
		_ = sqlDB.Close()
	})
}

// setSetting 覆盖指定设置项。必须是 upsert：健康检测各键已在 SettingSchemas 中声明，
// models.Init 的 seed 会先建好行，直接 Create 会撞 settings.key 唯一索引。
func setSetting(t *testing.T, key, value string) {
	t.Helper()
	if err := models.DB.
		Where("key = ?", key).
		Assign(&models.Setting{Key: key, Value: value}).
		FirstOrCreate(&models.Setting{}).Error; err != nil {
		t.Fatalf("set setting %s: %v", key, err)
	}
}

// TestHandleCheckResult_AutoEnableClearsConsecutiveFailures 回归：自动启用必须同时把连续失败计数清零。
// 曾用结构体 Updates 传 ConsecutiveFailures: 0，被 GORM 当零值跳过，导致关联恢复后
// 再失败一次即达阈值被重新禁用（自动恢复形同失效）。
func TestHandleCheckResult_AutoEnableClearsConsecutiveFailures(t *testing.T) {
	initHealthCheckTestDB(t)
	setSetting(t, models.SettingKeyHealthCheckAutoEnable, "true")

	ctx := context.Background()
	disabled := false
	mp := models.ModelWithProvider{
		ModelID:             1,
		ProviderID:          1,
		ProviderModel:       "pm",
		Weight:              10,
		Priority:            10,
		Status:              &disabled,
		ConsecutiveFailures: 3,
	}
	if err := models.DB.Create(&mp).Error; err != nil {
		t.Fatalf("create model provider: %v", err)
	}

	GetHealthChecker().handleCheckResult(ctx, &mp, "p1", models.HealthCheckStatusSuccess)

	got, err := repository.Default().ModelWithProvider.Get(ctx, mp.ID)
	if err != nil {
		t.Fatalf("reload model provider: %v", err)
	}
	if got.Status == nil || !*got.Status {
		t.Fatalf("status = %v, want enabled", got.Status)
	}
	if got.ConsecutiveFailures != 0 {
		t.Fatalf("consecutive_failures = %d, want 0", got.ConsecutiveFailures)
	}
}

// TestHandleCheckResult_DisableKeepsFailureCount 失败禁用路径不应顺带改动连续失败计数。
func TestHandleCheckResult_DisableKeepsFailureCount(t *testing.T) {
	initHealthCheckTestDB(t)
	setSetting(t, models.SettingKeyHealthCheckFailureThreshold, "1")

	ctx := context.Background()
	enabled := true
	mp := models.ModelWithProvider{
		ModelID:             1,
		ProviderID:          1,
		ProviderModel:       "pm",
		Weight:              10,
		Priority:            10,
		Status:              &enabled,
		ConsecutiveFailures: 2,
	}
	if err := models.DB.Create(&mp).Error; err != nil {
		t.Fatalf("create model provider: %v", err)
	}
	// getConsecutiveFailures 按健康检查日志统计，需要一条 error 日志才达阈值
	if err := models.DB.Create(&models.HealthCheckLog{ModelProviderID: mp.ID, Status: "error"}).Error; err != nil {
		t.Fatalf("create health check log: %v", err)
	}

	GetHealthChecker().handleCheckResult(ctx, &mp, "p1", models.HealthCheckStatusError)

	got, err := repository.Default().ModelWithProvider.Get(ctx, mp.ID)
	if err != nil {
		t.Fatalf("reload model provider: %v", err)
	}
	if got.Status == nil || *got.Status {
		t.Fatalf("status = %v, want disabled", got.Status)
	}
	if got.ConsecutiveFailures != 2 {
		t.Fatalf("consecutive_failures = %d, want 2 (unchanged)", got.ConsecutiveFailures)
	}
}

// captureWarnLogs 装一个只收本包留痕的日志器（Stage 36 的判停跳过必须可断言）。
// 同时清空设置域留痕限频状态：它是进程级的，跨用例/-count 轮次会互相压掉留痕。
func captureWarnLogs(t *testing.T) *strings.Builder {
	t.Helper()
	settings.ResetReadSkipLimiterForTest()

	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &logs
}

// createAssociation 建一条未设状态的关联（只给日志计数类用例提供 ID）。
func createAssociation(t *testing.T) models.ModelWithProvider {
	t.Helper()

	mp := models.ModelWithProvider{ModelID: 1, ProviderID: 1, ProviderModel: "pm"}
	if err := models.DB.Create(&mp).Error; err != nil {
		t.Fatalf("create model provider: %v", err)
	}
	return mp
}

// seedHealthCheckLogs 按传入顺序（时间升序）写入健康检测日志。
func seedHealthCheckLogs(t *testing.T, mpID uint, statuses ...string) {
	t.Helper()

	base := time.Now()
	for i, status := range statuses {
		if err := models.DB.Create(&models.HealthCheckLog{
			ModelProviderID: mpID, Status: status, CheckedAt: base.Add(time.Duration(i) * time.Minute),
		}).Error; err != nil {
			t.Fatalf("create health check log: %v", err)
		}
	}
}

// seedEnabledAssociationWithUpstreamFailures 建「启用中 + 已有 n 次上游失败」的关联：
// 判停相关用例的共用前置（默认阈值 3，够它在新旧行为下走向停用，差别只在读失败腿）。
func seedEnabledAssociationWithUpstreamFailures(t *testing.T, n int) models.ModelWithProvider {
	t.Helper()

	enabled := true
	mp := models.ModelWithProvider{
		ModelID: 1, ProviderID: 1, ProviderModel: "pm",
		Weight: 10, Priority: 10, Status: &enabled,
	}
	if err := models.DB.Create(&mp).Error; err != nil {
		t.Fatalf("create model provider: %v", err)
	}

	statuses := make([]string, n)
	for i := range statuses {
		statuses[i] = models.HealthCheckStatusError
	}
	seedHealthCheckLogs(t, mp.ID, statuses...)
	return mp
}

// TestHandleCheckResult_ThresholdReadError_SkipsDisableStep 判停跳过契约（Stage 36）：
// 阈值读不出来（生产三态之一：存值非法 / 查询失败）时禁止拿默认值冒充真配置去停用关联
// ——数据库不稳期正是最容易被假阈值误停用可用关联的时刻，与 Stage 33 结算侧同口径。
func TestHandleCheckResult_ThresholdReadError_SkipsDisableStep(t *testing.T) {
	initHealthCheckTestDB(t)
	// 存值非法 = 生产「读失败」三态中可由用例稳定复现的那一种（另有查询失败腿见下条用例）
	setSetting(t, models.SettingKeyHealthCheckFailureThreshold, "abc")
	logs := captureWarnLogs(t)

	ctx := context.Background()
	mp := seedEnabledAssociationWithUpstreamFailures(t, 3)

	GetHealthChecker().handleCheckResult(ctx, &mp, "p1", models.HealthCheckStatusError)

	got, err := repository.Default().ModelWithProvider.Get(ctx, mp.ID)
	if err != nil {
		t.Fatalf("reload model provider: %v", err)
	}
	if got.Status == nil || !*got.Status {
		t.Errorf("status = %v, want enabled（阈值读不出来时不得执行停用写）", got.Status)
	}
	if !strings.Contains(logs.String(), "dependent write step skipped") {
		t.Errorf("缺跳过留痕（读失败必须可事后回答「哪一步没做」）:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), models.SettingKeyHealthCheckFailureThreshold) {
		t.Errorf("留痕未带出读失败的键名:\n%s", logs.String())
	}
}

// TestHandleCheckResult_SwitchReadError_SkipsDisableStep 「允许停用」开关读失败同样跳过：
// 该键的默认值是 true（允许停用），读失败退默认值等于替用户按下停用开关。
func TestHandleCheckResult_SwitchReadError_SkipsDisableStep(t *testing.T) {
	initHealthCheckTestDB(t)
	// 整表不可读 = 生产「查询失败」腿（DB 不可用 / 锁争用）
	if err := models.DB.Exec("DROP TABLE settings").Error; err != nil {
		t.Fatalf("drop settings table: %v", err)
	}
	logs := captureWarnLogs(t)

	ctx := context.Background()
	mp := seedEnabledAssociationWithUpstreamFailures(t, 3)

	GetHealthChecker().handleCheckResult(ctx, &mp, "p1", models.HealthCheckStatusError)

	got, err := repository.Default().ModelWithProvider.Get(ctx, mp.ID)
	if err != nil {
		t.Fatalf("reload model provider: %v", err)
	}
	if got.Status == nil || !*got.Status {
		t.Errorf("status = %v, want enabled（开关读不出来时不得执行停用写）", got.Status)
	}
	if !strings.Contains(logs.String(), models.SettingKeyHealthCheckFailureDisableEnabled) {
		t.Errorf("留痕未带出读失败的键名:\n%s", logs.String())
	}
}

// TestGetConsecutiveFailures_IgnoresNotMeasuredRows 连续失败只数「上游给了坏结论」的那些：
// 本机没测成的记录既不是成功也不是失败，必须跳过继续往前数（不打断已有的连续失败），
// 否则数据库抖一阵子会把连续失败数静默清零，与「故障期旧账恢复后照结」的口径相反。
func TestGetConsecutiveFailures_IgnoresNotMeasuredRows(t *testing.T) {
	initHealthCheckTestDB(t)

	ctx := context.Background()
	mp := createAssociation(t)
	// 时间升序：error → not_measured → error → error（最新一条为 error）
	seedHealthCheckLogs(t, mp.ID,
		models.HealthCheckStatusError,
		models.HealthCheckStatusNotMeasured,
		models.HealthCheckStatusError,
		models.HealthCheckStatusError,
	)

	got, err := (&HealthChecker{}).getConsecutiveFailures(ctx, mp.ID)
	if err != nil {
		t.Fatalf("getConsecutiveFailures: %v", err)
	}
	if got != 3 {
		t.Fatalf("consecutive failures = %d, want 3（本机没测成的记录不该打断连续失败）", got)
	}
}

// TestGetConsecutiveFailures_NotMeasuredRowsDoNotEvictVerdicts 审查回批 W2 回归：
// 未完成的行**不占连续失败的窗口**——否则数据库长时间不稳（十几轮未完成）会把之前攒下的
// 真实失败全部挤出窗口，恢复后从 1 重新数，"故障期旧账、恢复后照阈值结账"静默失效。
func TestGetConsecutiveFailures_NotMeasuredRowsDoNotEvictVerdicts(t *testing.T) {
	initHealthCheckTestDB(t)

	ctx := context.Background()
	mp := createAssociation(t)

	statuses := []string{
		models.HealthCheckStatusError,
		models.HealthCheckStatusError,
		models.HealthCheckStatusError,
	}
	for i := 0; i < 12; i++ {
		statuses = append(statuses, models.HealthCheckStatusNotMeasured)
	}
	seedHealthCheckLogs(t, mp.ID, statuses...)

	got, err := (&HealthChecker{}).getConsecutiveFailures(ctx, mp.ID)
	if err != nil {
		t.Fatalf("getConsecutiveFailures: %v", err)
	}
	if got != 3 {
		t.Fatalf("consecutive failures = %d, want 3（12 轮未完成不得把 3 次真实失败挤出窗口）", got)
	}
}

// TestGetConsecutiveFailures_SuccessBreaksStreak 成功仍然截断连续失败（口径不能被跳过规则带松）。
func TestGetConsecutiveFailures_SuccessBreaksStreak(t *testing.T) {
	initHealthCheckTestDB(t)

	ctx := context.Background()
	mp := createAssociation(t)
	seedHealthCheckLogs(t, mp.ID,
		models.HealthCheckStatusError,
		models.HealthCheckStatusSuccess,
		models.HealthCheckStatusError,
	)

	got, err := (&HealthChecker{}).getConsecutiveFailures(ctx, mp.ID)
	if err != nil {
		t.Fatalf("getConsecutiveFailures: %v", err)
	}
	// 最新一条是 error（计 1），往前是 success（截断）——若截断失效会把更早那条 error 连起来变成 2
	if got != 1 {
		t.Fatalf("consecutive failures = %d, want 1（success 必须截断，不得跨过它继续数）", got)
	}
}
