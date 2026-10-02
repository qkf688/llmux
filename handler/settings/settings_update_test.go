package settings

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// getSettingsViaHandler 以 httptest 打真实 GetSettings handler，返回响应体 JSON。
// 契约断言必须打在 HTTP 响应体上（AGENTS §5）：绕过 handler 直调
// buildSettingsResponse 或 json.Marshal(response) 都测不出 handler 出线
// （信封层/组装）的键名与 omitempty 断层。
func getSettingsViaHandler(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/settings", nil)
	GetSettings(c)
	if w.Code != http.StatusOK {
		t.Fatalf("GetSettings status=%d body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

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

	// GET 响应体与写库一致，形成往返闭环。契约断言打在真实 HTTP 响应体上（AGENTS §5）：
	// gjson Exists() 区分「键缺失」与「键在值为 0」——键缺失正是 tag 写错/漏写、
	// omitempty 吞键这类断层的可见形态。
	body := getSettingsViaHandler(t)
	v := gjson.Get(body, "data.cred_health_auth_fail_threshold")
	if !v.Exists() {
		t.Fatalf("GET 响应体缺键 data.cred_health_auth_fail_threshold，body=%s", body)
	}
	if v.Int() != 5 {
		t.Errorf("GET 响应体 data.cred_health_auth_fail_threshold = %d, want 5，body=%s", v.Int(), body)
	}
}

// TestUpdateSettingsFromRequest_CacheAffinityEnabled_RoundTrip 覆盖 cache_affinity_enabled
// 的 PUT 往返：写入非默认值 false 后，数据库读回、GET 响应与 JSON 键名三处一致——
// bool false 不得被「零值=未设置」语义吞掉。
func TestUpdateSettingsFromRequest_CacheAffinityEnabled_RoundTrip(t *testing.T) {
	testsupport.InitTestDB(t)
	ctx := context.Background()

	req := &UpdateSettingsRequest{}
	if err := models.ApplySettingDefaults(req); err != nil {
		t.Fatalf("ApplySettingDefaults: %v", err)
	}
	req.CacheAffinityEnabled = false // 默认 true 的合法非默认值

	if err := updateSettingsFromRequest(ctx, req); err != nil {
		t.Fatalf("updateSettingsFromRequest: %v", err)
	}

	store := settingStore()
	if got := store.GetBool(ctx, models.SettingKeyCacheAffinityEnabled); got {
		t.Errorf("CacheAffinityEnabled = true, want false（false 不得被零值语义吞掉）")
	}

	// GET 响应体与写库一致，形成往返闭环。契约断言打在真实 HTTP 响应体上（AGENTS §5）：
	// gjson Exists() 区分「键缺失」与「键在值为 false」——false 不得被零值语义吞掉，
	// 键缺失正是 omitempty 吞键类断层的可见形态，故断 Type 必须是字面 false。
	body := getSettingsViaHandler(t)
	v := gjson.Get(body, "data.cache_affinity_enabled")
	if !v.Exists() {
		t.Fatalf("GET 响应体缺键 data.cache_affinity_enabled，body=%s", body)
	}
	if v.Type != gjson.False {
		t.Errorf("GET 响应体 data.cache_affinity_enabled = %s (type=%s), want 字面 false，body=%s", v.Raw, v.Type, body)
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

// TestUpdateSettingsFromRequest_ResponseLogMaxKiB_RejectsOutOfRange 回归（Stage 23）：
// 回复日志上限 schema Min=16/Max=16384，越界（0 或天文数字）应整请求校验失败
// （directClientError → 400），且不留下任何部分写入。
func TestUpdateSettingsFromRequest_ResponseLogMaxKiB_RejectsOutOfRange(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value int
	}{
		{"below min", 0},
		{"above max", 999999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testsupport.InitTestDB(t)
			ctx := context.Background()

			req := &UpdateSettingsRequest{}
			if err := models.ApplySettingDefaults(req); err != nil {
				t.Fatalf("ApplySettingDefaults: %v", err)
			}
			req.ResponseLogMaxKiB = tc.value
			req.LogRetentionCount = 42 // 合法哨兵：若部分写入会被读回

			err := updateSettingsFromRequest(ctx, req)
			if err == nil {
				t.Fatalf("expected validation error for ResponseLogMaxKiB=%d, got nil", tc.value)
			}
			var clientErr directClientError
			if !errors.As(err, &clientErr) {
				t.Fatalf("err = %v, want directClientError（越界是客户端校验错，应 400）", err)
			}

			store := settingStore()
			if got := store.GetInt(ctx, models.SettingKeyResponseLogMaxKiB); got != 2048 {
				t.Errorf("ResponseLogMaxKiB = %d, want default 2048 (out-of-range must not be written)", got)
			}
			if got := store.GetInt(ctx, models.SettingKeyLogRetentionCount); got != 100 {
				t.Errorf("LogRetentionCount = %d, want default 100 (validation failure must not leave partial writes)", got)
			}
		})
	}
}

// TestUpdateSettingsFromRequest_NonStreamResponseTimeout_RoundTrip 覆盖
// non_stream_response_timeout（Stage 38 新增的非流式整段预算窗口）的 PUT 往返：
// 写入合法非默认值后，数据库读回与 GET 响应都应命中该值。
// 这是「后台改这个数就生效」的写侧证据；读侧（按入站 stream 取哪个键）由
// service/chat 的窗口分流用例锁定。
func TestUpdateSettingsFromRequest_NonStreamResponseTimeout_RoundTrip(t *testing.T) {
	testsupport.InitTestDB(t)
	ctx := context.Background()

	req := &UpdateSettingsRequest{}
	if err := models.ApplySettingDefaults(req); err != nil {
		t.Fatalf("ApplySettingDefaults: %v", err)
	}
	if req.NonStreamResponseTimeout != 90 {
		t.Fatalf("schema 默认值 = %d, want 90（慢供应商现场证据支撑的余量）", req.NonStreamResponseTimeout)
	}
	req.NonStreamResponseTimeout = 120 // Min=1 的合法非默认值

	if err := updateSettingsFromRequest(ctx, req); err != nil {
		t.Fatalf("updateSettingsFromRequest: %v", err)
	}

	store := settingStore()
	if got := store.GetInt(ctx, models.SettingKeyNonStreamResponseTimeout); got != 120 {
		t.Errorf("NonStreamResponseTimeout = %d, want 120", got)
	}

	body := getSettingsViaHandler(t)
	v := gjson.Get(body, "data.non_stream_response_timeout")
	if !v.Exists() {
		t.Fatalf("GET 响应体缺键 data.non_stream_response_timeout，body=%s", body)
	}
	if v.Int() != 120 {
		t.Errorf("GET 响应体 data.non_stream_response_timeout = %d, want 120，body=%s", v.Int(), body)
	}
}
