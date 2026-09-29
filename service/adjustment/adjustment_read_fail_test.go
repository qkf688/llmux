package adjustment

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/models"
)

// 本文件是 Stage 33「结算中途设置读取失败 → 显式降级」的回归组：
// 读失败的设置键（failKeys）不得再用编造的默认值/覆盖值驱动写动作（误停用/误衰减），
// 但同一结算里未受影响的步骤照常执行（计数照记、可读配置驱动的写照做）。

// TestIncrementConsecutiveFailures_ReadFail_SkipsAutoDisable 读失败不得误停用：
// 停用开关与阈值都读不到时，计数照记、判停跳过（旧行为会拿 default/覆盖值判停）。
func TestIncrementConsecutiveFailures_ReadFail_SkipsAutoDisable(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyConsecutiveFailureDisableEnabled: true,
		},
		ints: map[string]int{
			models.SettingKeyConsecutiveFailureThreshold: 1,
		},
		failKeys: map[string]bool{
			models.SettingKeyConsecutiveFailureDisableEnabled: true,
			models.SettingKeyConsecutiveFailureThreshold:      true,
		},
	})

	mp := createModelProvider(t, 10, 10, 2, nil)
	IncrementConsecutiveFailures(context.Background(), mp.ID, "p", "pm")

	got := loadModelProvider(t, mp.ID)
	if got.ConsecutiveFailures != 3 {
		t.Fatalf("ConsecutiveFailures = %d, want 3（计数不因设置读失败而丢）", got.ConsecutiveFailures)
	}
	if got.Status != nil && !*got.Status {
		t.Fatal("Status = false, want 未停用（设置读不到 → 判停步骤跳过，不得拿编造阈值停用）")
	}
}

// priorityDecayReadFailSettings 优先级衰减粒度用例的共用配置骨架：
// 衰减与判停全开 + 危险覆写值（旧代码会拿它们写衰减/判停），failKeys 指定本次
// 模拟读失败的键——两个粒度用例只差这一维，骨架不重复。
func priorityDecayReadFailSettings(failKeys ...string) mapReader {
	fail := make(map[string]bool, len(failKeys))
	for _, k := range failKeys {
		fail[k] = true
	}
	return mapReader{
		bools: map[string]bool{
			models.SettingKeyAutoPriorityDecay:               true,
			models.SettingKeyAutoPriorityDecayDisableEnabled: true,
		},
		ints: map[string]int{
			models.SettingKeyAutoPriorityDecayStep:      2,
			models.SettingKeyAutoPriorityDecayThreshold: 10,
		},
		failKeys: fail,
	}
}

// TestApplyPriorityDecay_StepReadFail_DecaySkippedJudgmentProceeds 粒度锁「只跳过读不到的那一步」：
// 衰减幅度读不到 → 只跳过衰减写；停用判断的配置可读，判断照常执行。
func TestApplyPriorityDecay_StepReadFail_DecaySkippedJudgmentProceeds(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, priorityDecayReadFailSettings(models.SettingKeyAutoPriorityDecayStep))

	mp := createModelProvider(t, 10, 5, 0, nil)
	ApplyPriorityDecayByModelProviderID(context.Background(), mp.ID, "p", "pm")

	got := loadModelProvider(t, mp.ID)
	if got.Priority != 5 {
		t.Fatalf("Priority = %d, want 5（衰减幅度读不到 → 衰减写跳过）", got.Priority)
	}
	if got.Status == nil || *got.Status {
		t.Fatal("Status = 启用, want 已停用（阈值可读 → 停用判断照常：5 <= 10）")
	}
}

// TestApplyPriorityDecay_ThresholdReadFail_DecayProceedsJudgmentSkipped 反向粒度：
// 停用阈值读不到 → 只跳过判停；衰减写照做。
func TestApplyPriorityDecay_ThresholdReadFail_DecayProceedsJudgmentSkipped(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, priorityDecayReadFailSettings(models.SettingKeyAutoPriorityDecayThreshold))

	mp := createModelProvider(t, 10, 5, 0, nil)
	ApplyPriorityDecayByModelProviderID(context.Background(), mp.ID, "p", "pm")

	got := loadModelProvider(t, mp.ID)
	if got.Priority != 3 {
		t.Fatalf("Priority = %d, want 3（衰减步骤照做）", got.Priority)
	}
	if got.Status != nil && !*got.Status {
		t.Fatal("Status = false, want 未停用（阈值读不到 → 判停跳过，不得拿编造阈值停用）")
	}
}

// TestApplyWeightDecay_StepReadFail_SkipsDecay 权重衰减幅度读不到 → 衰减写跳过。
func TestApplyWeightDecay_StepReadFail_SkipsDecay(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyAutoWeightDecay: true,
		},
		ints: map[string]int{
			models.SettingKeyAutoWeightDecayStep: 5,
		},
		failKeys: map[string]bool{
			models.SettingKeyAutoWeightDecayStep: true,
		},
	})

	mp := createModelProvider(t, 10, 10, 0, nil)
	ApplyWeightDecayByModelProviderID(context.Background(), mp.ID, "p", "pm")

	got := loadModelProvider(t, mp.ID)
	if got.Weight != 10 {
		t.Fatalf("Weight = %d, want 10（衰减幅度读不到 → 不写衰减）", got.Weight)
	}
}

// TestApplySuccessAdjustments_ReadFail_SkipsIncrease 成功侧同口径：步长/上限读不到 → 不写提升。
func TestApplySuccessAdjustments_ReadFail_SkipsIncrease(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyAutoSuccessIncrease: true,
		},
		ints: map[string]int{
			models.SettingKeyAutoWeightIncreaseStep:   3,
			models.SettingKeyAutoWeightIncreaseMax:    100,
			models.SettingKeyAutoPriorityIncreaseStep: 3,
			models.SettingKeyAutoPriorityIncreaseMax:  100,
		},
		failKeys: map[string]bool{
			models.SettingKeyAutoWeightIncreaseStep:   true,
			models.SettingKeyAutoWeightIncreaseMax:    true,
			models.SettingKeyAutoPriorityIncreaseStep: true,
			models.SettingKeyAutoPriorityIncreaseMax:  true,
		},
	})

	mp := createModelProvider(t, 10, 10, 0, nil)
	ApplySuccessAdjustments(context.Background(), mp.ID)

	got := loadModelProvider(t, mp.ID)
	if got.Weight != 10 || got.Priority != 10 {
		t.Fatalf("Weight/Priority = %d/%d, want 10/10（配置读不到 → 提升写跳过）", got.Weight, got.Priority)
	}
}

// TestApplySuccessAdjustments_GateReadFail_SkipsIncrease 成功侧**闸门**读失败腿（Stage 36 补缺口）：
// 闸门本身读不到时不得按默认「计入成功」继续写提升——覆写值 true 让旧代码照样提升，
// 这条腿此前无人覆盖（步长/上限读失败是另一条腿，见上一条用例）。
func TestApplySuccessAdjustments_GateReadFail_SkipsIncrease(t *testing.T) {
	initAdjustmentTestDB(t)
	useSettings(t, mapReader{
		bools: map[string]bool{
			models.SettingKeyAutoSuccessIncrease: true, // 裸变体的危险覆写值
		},
		ints: map[string]int{
			models.SettingKeyAutoWeightIncreaseStep:   3,
			models.SettingKeyAutoWeightIncreaseMax:    100,
			models.SettingKeyAutoPriorityIncreaseStep: 3,
			models.SettingKeyAutoPriorityIncreaseMax:  100,
		},
		failKeys: map[string]bool{
			models.SettingKeyAutoSuccessIncrease: true,
		},
	})

	mp := createModelProvider(t, 10, 10, 0, nil)
	ApplySuccessAdjustments(context.Background(), mp.ID)

	got := loadModelProvider(t, mp.ID)
	if got.Weight != 10 || got.Priority != 10 {
		t.Fatalf("Weight/Priority = %d/%d, want 10/10（闸门读不到 → 提升写整体跳过）", got.Weight, got.Priority)
	}
}
