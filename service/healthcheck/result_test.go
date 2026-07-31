package healthcheck

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
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

	GetHealthChecker().handleCheckResult(ctx, &mp, "p1", true)

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

	GetHealthChecker().handleCheckResult(ctx, &mp, "p1", false)

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
