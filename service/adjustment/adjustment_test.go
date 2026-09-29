package adjustment

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// mapReader 是测试用 settings.Reader：命中 map 则返回覆盖值，否则走 default。
//
// 两处必须按**生产三态**建模（Stage 36）：
//   - failKeys 命中 = 读失败，此时**裸变体与 E 变体一起失败**（生产里两者读同一张表，
//     不存在「只有 E 变体读不到」的世界）；旧 mock 让裸变体照旧返回覆写值，等于把
//     「读失败时裸读退默认值/覆写值驱动写」这条真实风险从测试里抹掉；
//   - 存值非法（低于 min）= 读失败（生产 GetSettingIntE 的语义），裸变体退 default，
//     不再静默钳到 min 冒充正常值。
type mapReader struct {
	bools    map[string]bool
	ints     map[string]int
	failKeys map[string]bool
}

func (m mapReader) readErr(key string) error {
	if m.failKeys[key] {
		return errors.New("simulated settings read failure")
	}
	return nil
}

func (m mapReader) Bool(_ context.Context, key string, defaultValue bool) bool {
	if m.readErr(key) != nil {
		return defaultValue
	}
	if m.bools != nil {
		if v, ok := m.bools[key]; ok {
			return v
		}
	}
	return defaultValue
}

func (m mapReader) Int(_ context.Context, key string, defaultValue, minValue int) int {
	v, err := m.intValue(key, defaultValue, minValue)
	if err != nil {
		return defaultValue
	}
	return v
}

func (m mapReader) String(_ context.Context, _ string, defaultValue string) string {
	return defaultValue
}

func (m mapReader) BoolE(ctx context.Context, key string, defaultValue bool) (bool, error) {
	if err := m.readErr(key); err != nil {
		return defaultValue, err
	}
	return m.Bool(ctx, key, defaultValue), nil
}

func (m mapReader) IntE(_ context.Context, key string, defaultValue, minValue int) (int, error) {
	if err := m.readErr(key); err != nil {
		return defaultValue, err
	}
	return m.intValue(key, defaultValue, minValue)
}

// intValue 按生产语义取值：覆盖值存在但非法（低于 min）→ 报错（不静默钳到 min）。
func (m mapReader) intValue(key string, defaultValue, minValue int) (int, error) {
	if err := m.readErr(key); err != nil {
		return defaultValue, err
	}

	v := defaultValue
	if m.ints != nil {
		if x, ok := m.ints[key]; ok {
			v = x
		}
	}
	if minValue > 0 && v < minValue {
		return defaultValue, fmt.Errorf("simulated invalid stored value %d for %s (min %d)", v, key, minValue)
	}
	return v, nil
}

func useSettings(t *testing.T, r mapReader) {
	t.Helper()
	SetSettingsReader(r)
	t.Cleanup(func() { SetSettingsReader(nil) })
}

func initAdjustmentTestDB(t *testing.T) {
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

func createModelProvider(t *testing.T, weight, priority, consecutive int, status *bool) models.ModelWithProvider {
	t.Helper()
	mp := models.ModelWithProvider{
		ModelID:             1,
		ProviderModel:       "pm",
		ProviderID:          1,
		Weight:              weight,
		Priority:            priority,
		ConsecutiveFailures: consecutive,
		Status:              status,
	}
	if err := models.DB.Create(&mp).Error; err != nil {
		t.Fatalf("create model provider: %v", err)
	}
	return mp
}

func loadModelProvider(t *testing.T, id uint) models.ModelWithProvider {
	t.Helper()
	var mp models.ModelWithProvider
	if err := models.DB.First(&mp, id).Error; err != nil {
		t.Fatalf("load model provider: %v", err)
	}
	return mp
}

func TestApplySuccessAdjustments_IncreasesWeightAndPriorityWithClamp(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyAutoSuccessIncrease: true,
		},
		ints: map[string]int{
			models.SettingKeyAutoWeightIncreaseStep:   10,
			models.SettingKeyAutoWeightIncreaseMax:    15,
			models.SettingKeyAutoPriorityIncreaseStep: 5,
			models.SettingKeyAutoPriorityIncreaseMax:  12,
		},
	})

	mp := createModelProvider(t, 10, 10, 0, nil)
	ApplySuccessAdjustments(context.Background(), mp.ID)

	got := loadModelProvider(t, mp.ID)
	if got.Weight != 15 {
		t.Fatalf("weight = %d, want 15 (clamped to max)", got.Weight)
	}
	if got.Priority != 12 {
		t.Fatalf("priority = %d, want 12 (clamped to max)", got.Priority)
	}
}

func TestApplyWeightDecayByModelProviderID_FloorAtOne(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyAutoWeightDecay: true,
		},
		ints: map[string]int{
			models.SettingKeyAutoWeightDecayStep: 5,
		},
	})

	mp := createModelProvider(t, 3, 100, 0, nil)
	ApplyWeightDecayByModelProviderID(context.Background(), mp.ID, "p", "pm")

	got := loadModelProvider(t, mp.ID)
	if got.Weight != 1 {
		t.Fatalf("weight = %d, want 1 (floor)", got.Weight)
	}
}

func TestApplyWeightDecayByModelProviderID_DisabledNoOp(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyAutoWeightDecay: false,
		},
		ints: map[string]int{
			models.SettingKeyAutoWeightDecayStep: 5,
		},
	})

	mp := createModelProvider(t, 10, 100, 0, nil)
	ApplyWeightDecayByModelProviderID(context.Background(), mp.ID, "p", "pm")

	got := loadModelProvider(t, mp.ID)
	if got.Weight != 10 {
		t.Fatalf("weight = %d, want 10 when decay disabled", got.Weight)
	}
}

func TestIncrementConsecutiveFailures_DisablesAtThreshold(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyConsecutiveFailureDisableEnabled: true,
		},
		ints: map[string]int{
			models.SettingKeyConsecutiveFailureThreshold: 3,
		},
	})

	trueVal := true
	mp := createModelProvider(t, 10, 100, 2, &trueVal)

	IncrementConsecutiveFailures(context.Background(), mp.ID, "p", "pm")

	got := loadModelProvider(t, mp.ID)
	if got.ConsecutiveFailures != 3 {
		t.Fatalf("consecutive_failures = %d, want 3", got.ConsecutiveFailures)
	}
	if got.Status == nil || *got.Status {
		t.Fatalf("status enabled = %v, want false after threshold", got.Status)
	}
}

func TestResetConsecutiveFailures(t *testing.T) {
	initAdjustmentTestDB(t)
	trueVal := true
	mp := createModelProvider(t, 10, 100, 4, &trueVal)

	ResetConsecutiveFailures(context.Background(), mp.ID)

	got := loadModelProvider(t, mp.ID)
	if got.ConsecutiveFailures != 0 {
		t.Fatalf("consecutive_failures = %d, want 0", got.ConsecutiveFailures)
	}
}

func TestAutoDecayEnabled_ReflectsSettings(t *testing.T) {
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyAutoWeightDecay:   true,
			models.SettingKeyAutoPriorityDecay: false,
		},
	})

	if !AutoWeightDecayEnabled(context.Background()) {
		t.Fatal("AutoWeightDecayEnabled = false, want true")
	}
	if AutoPriorityDecayEnabled(context.Background()) {
		t.Fatal("AutoPriorityDecayEnabled = true, want false")
	}
}

// 并发自增不应丢失计数——旧实现读改写会互相覆盖，10 并发只记 1。
func TestIncrementConsecutiveFailures_ConcurrentNoLostUpdates(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyConsecutiveFailureDisableEnabled: true,
		},
		ints: map[string]int{
			models.SettingKeyConsecutiveFailureThreshold: 100, // 高阈值避免禁用干扰计数验证
		},
	})

	trueVal := true
	mp := createModelProvider(t, 10, 100, 0, &trueVal)

	var wg sync.WaitGroup
	const N = 10
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			IncrementConsecutiveFailures(context.Background(), mp.ID, "p", "pm")
		}()
	}
	wg.Wait()

	got := loadModelProvider(t, mp.ID)
	if got.ConsecutiveFailures != N {
		t.Fatalf("consecutive_failures = %d, want %d (no lost updates)", got.ConsecutiveFailures, N)
	}
}

// 计数始终自增——即使 auto-disable 关闭，consecutive_failures 也应累加（数据列不寄生于策略开关）。
func TestIncrementConsecutiveFailures_IncrementsEvenWhenDisableOff(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyConsecutiveFailureDisableEnabled: false,
		},
	})

	trueVal := true
	mp := createModelProvider(t, 10, 100, 0, &trueVal)

	IncrementConsecutiveFailures(context.Background(), mp.ID, "p", "pm")
	IncrementConsecutiveFailures(context.Background(), mp.ID, "p", "pm")

	got := loadModelProvider(t, mp.ID)
	if got.ConsecutiveFailures != 2 {
		t.Fatalf("consecutive_failures = %d, want 2 (counter should increment even when disable is off)", got.ConsecutiveFailures)
	}
	if got.Status == nil || !*got.Status {
		t.Fatalf("status should remain enabled when disable is off, got %v", got.Status)
	}
}
