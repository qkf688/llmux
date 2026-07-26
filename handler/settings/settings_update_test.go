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
