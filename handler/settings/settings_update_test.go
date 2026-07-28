package settings

import (
	"context"
	"testing"

	"github.com/atopos31/llmio/models"
)

// TestUpdateSettingsFromRequest_PersistsValues 回归：settingStore 曾是包级变量
// `settings.NewStore(models.DB)`，在 models.Init 之前求值，永久持有 nil *gorm.DB，
// 于是任何设置保存都在 SetTyped 处空指针 panic（被 gin Recovery 兜成 500）。
func TestUpdateSettingsFromRequest_PersistsValues(t *testing.T) {
	initTestDB(t)
	ctx := context.Background()

	req := &UpdateSettingsRequest{}
	if err := models.ApplySettingDefaults(req); err != nil {
		t.Fatalf("ApplySettingDefaults: %v", err)
	}
	req.DisableAllLogs = true
	req.LogRetentionCount = 42
	// 保持关闭：置 true 会触发 batch import goroutine，与本用例无关
	req.AutoSaveTemplateOnAssociate = false

	if err := updateSettingsFromRequest(ctx, req); err != nil {
		t.Fatalf("updateSettingsFromRequest: %v", err)
	}

	store := settingStore()
	if got := store.GetBool(ctx, models.SettingKeyDisableAllLogs); !got {
		t.Errorf("DisableAllLogs = %v, want true", got)
	}
	if got := store.GetInt(ctx, models.SettingKeyLogRetentionCount); got != 42 {
		t.Errorf("LogRetentionCount = %d, want 42", got)
	}
}

// TestUpdateSettingsFromRequest_AutoSaveOldValueReadBeforeWrite 回归：
// oldAutoSave 曾在写循环之后才读，此时新值已落库，
// 「从关闭切到开启」这个跳变条件恒为 false，批量导入永远不触发。
func TestUpdateSettingsFromRequest_AutoSaveOldValueReadBeforeWrite(t *testing.T) {
	initTestDB(t)
	ctx := context.Background()

	store := settingStore()
	if err := store.SetTyped(ctx, models.SettingKeyAutoSaveTemplateOnAssociate, false); err != nil {
		t.Fatalf("seed old value: %v", err)
	}

	triggered := false
	original := triggerBatchImportForAutoSave
	triggerBatchImportForAutoSave = func() { triggered = true }
	t.Cleanup(func() { triggerBatchImportForAutoSave = original })

	req := &UpdateSettingsRequest{}
	if err := models.ApplySettingDefaults(req); err != nil {
		t.Fatalf("ApplySettingDefaults: %v", err)
	}
	req.AutoSaveTemplateOnAssociate = true

	if err := updateSettingsFromRequest(ctx, req); err != nil {
		t.Fatalf("updateSettingsFromRequest: %v", err)
	}

	if !triggered {
		t.Error("batch import was not triggered on false -> true transition")
	}
}

// TestUpdateSettingsFromRequest_ValidationFailureLeavesNoPartialWrites 回归：
// 旧实现边校验边写，校验到后面的字段失败时，前面已通过校验的字段已落库，
// 留下半更新状态。新实现先全量校验、再事务内写入，校验失败时不应有任何字段被改。
func TestUpdateSettingsFromRequest_ValidationFailureLeavesNoPartialWrites(t *testing.T) {
	initTestDB(t)
	ctx := context.Background()

	// 模拟客户端只发 {"disable_all_logs": true}：其余 int 字段为零值，
	// AutoWeightDecayDefault=0 违反 Min=1 约束。
	req := &UpdateSettingsRequest{
		DisableAllLogs: true,
		// AutoWeightDecayDefault 留零值 → 校验失败
	}

	err := updateSettingsFromRequest(ctx, req)
	if err == nil {
		t.Fatal("expected validation error for AutoWeightDecayDefault=0, got nil")
	}

	// 关键断言：校验失败不应写入任何字段，DisableAllLogs 应回到默认值 false。
	store := settingStore()
	if got := store.GetBool(ctx, models.SettingKeyDisableAllLogs); got {
		t.Errorf("DisableAllLogs was partially written (got true, want false): validation failure should not leave any writes")
	}
}
