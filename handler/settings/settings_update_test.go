package settings

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

// TestUpdateSettingsFromRequest_PersistsValues 回归：settingStore 曾是包级变量
// `settings.NewStore(models.DB)`，在 models.Init 之前求值，永久持有 nil *gorm.DB，
// 于是任何设置保存都在 SetTyped 处空指针 panic（被 gin Recovery 兜成 500）。
func TestUpdateSettingsFromRequest_PersistsValues(t *testing.T) {
	testsupport.InitTestDB(t)
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
	testsupport.InitTestDB(t)
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
	testsupport.InitTestDB(t)
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

// TestUpdateSettingsFromRequest_CredHealthAuthFailThreshold_RoundTrip 覆盖
// cred_health_auth_fail_threshold 的 PUT 往返：写入合法非默认值后，
// 数据库读回与 GET 响应都应命中该值（#11）。
func TestUpdateSettingsFromRequest_CredHealthAuthFailThreshold_RoundTrip(t *testing.T) {
	testsupport.InitTestDB(t)
	ctx := context.Background()

	req := &UpdateSettingsRequest{}
	if err := models.ApplySettingDefaults(req); err != nil {
		t.Fatalf("ApplySettingDefaults: %v", err)
	}
	req.CredHealthAuthFailThreshold = 5 // Min=1 的合法非默认值

	if err := updateSettingsFromRequest(ctx, req); err != nil {
		t.Fatalf("updateSettingsFromRequest: %v", err)
	}

	store := settingStore()
	if got := store.GetInt(ctx, models.SettingKeyCredHealthAuthFailThreshold); got != 5 {
		t.Errorf("CredHealthAuthFailThreshold = %d, want 5", got)
	}

	// GET 响应与写库一致，形成往返闭环。
	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	response, err := buildSettingsResponse(list)
	if err != nil {
		t.Fatalf("buildSettingsResponse: %v", err)
	}
	if response.CredHealthAuthFailThreshold != 5 {
		t.Errorf("GET CredHealthAuthFailThreshold = %d, want 5", response.CredHealthAuthFailThreshold)
	}
	// 序列化断层检查：真实 GET 响应键名来自 json tag，只断言 Go 字段会绕过序列化层
	//（tag 写错/漏写时测试仍绿）——补一次 JSON 键名与值断言闭合「往返」。
	b, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal(response): %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v, body=%s", err, b)
	}
	if v, ok := m["cred_health_auth_fail_threshold"]; !ok {
		t.Errorf("GET JSON 缺键 cred_health_auth_fail_threshold，body=%s", b)
	} else if v.(float64) != 5 {
		t.Errorf("GET JSON cred_health_auth_fail_threshold = %v, want 5", v)
	}
}

// TestUpdateSettingsFromRequest_CredHealthAuthFailThreshold_RejectsBelowMin 回归：
// CredHealthAuthFailThreshold 的 schema Min=1，0 意味着「永不判停」，语义无意义。
// 越界请求应整请求校验失败，且不留下任何部分写入。
func TestUpdateSettingsFromRequest_CredHealthAuthFailThreshold_RejectsBelowMin(t *testing.T) {
	testsupport.InitTestDB(t)
	ctx := context.Background()

	req := &UpdateSettingsRequest{}
	if err := models.ApplySettingDefaults(req); err != nil {
		t.Fatalf("ApplySettingDefaults: %v", err)
	}
	req.CredHealthAuthFailThreshold = 0 // 越界：Min=1
	req.LogRetentionCount = 42          // 合法哨兵：若部分写入会被读回

	err := updateSettingsFromRequest(ctx, req)
	if err == nil {
		t.Fatal("expected validation error for CredHealthAuthFailThreshold=0, got nil")
	}
	// 校验错必须是客户端直返错误（directClientError）：handler 层据此走 400 而非 500，
	// 不锁类型的话将来校验错被意外包装成内部错误时本测试仍绿灯。
	var clientErr directClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("err = %v, want directClientError（Min 越界是客户端校验错，应 400 而非 500）", err)
	}

	store := settingStore()
	if got := store.GetInt(ctx, models.SettingKeyCredHealthAuthFailThreshold); got != 3 {
		t.Errorf("CredHealthAuthFailThreshold = %d, want default 3 (below-min must not be written)", got)
	}
	if got := store.GetInt(ctx, models.SettingKeyLogRetentionCount); got != 100 {
		t.Errorf("LogRetentionCount = %d, want default 100 (validation failure must not leave partial writes)", got)
	}
}
