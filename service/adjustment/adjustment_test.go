package adjustment

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/atopos31/llmio/models"
)

// mapReader 是测试用 settings.Reader：命中 map 则返回覆盖值，否则走 default。
type mapReader struct {
	bools map[string]bool
	ints  map[string]int
}

func (m mapReader) Bool(_ context.Context, key string, defaultValue bool) bool {
	if m.bools != nil {
		if v, ok := m.bools[key]; ok {
			return v
		}
	}
	return defaultValue
}

func (m mapReader) Int(_ context.Context, key string, defaultValue, minValue int) int {
	v := defaultValue
	if m.ints != nil {
		if x, ok := m.ints[key]; ok {
			v = x
		}
	}
	if minValue > 0 && v < minValue {
		return minValue
	}
	return v
}

func (m mapReader) String(_ context.Context, _ string, defaultValue string) string {
	return defaultValue
}

func useSettings(t *testing.T, r mapReader) {
	t.Helper()
	SetSettingsReader(r)
	t.Cleanup(func() { SetSettingsReader(nil) })
}

func initAdjustmentTestDB(t *testing.T) {
	t.Helper()
	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmio-test.db"))
	t.Cleanup(func() {
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
