package healthcheck

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// TestGetBatchHealthCheckStatus_NotMeasuredCountedSeparately 批量检测结果的 wire 契约（Stage 36）：
// 「本机没测成」必须单列 `not_measured`，不得并进 `failed`——并进去等于把本机问题记在供应商头上，
// 而这正是本 Stage 要消灭的误导。断言打在响应体 JSON 上（键存在性 + 计数），并核数字与总数加得平。
func TestGetBatchHealthCheckStatus_NotMeasuredCountedSeparately(t *testing.T) {
	testsupport.InitTestDB(t)

	statuses := []string{
		models.HealthCheckStatusSuccess,
		models.HealthCheckStatusError,
		models.HealthCheckStatusNotMeasured,
	}
	const batchID = "batch-tri-state"
	for i, status := range statuses {
		mp := models.ModelWithProvider{
			ModelID: uint(i + 1), ProviderID: uint(i + 1), ProviderModel: "pm", Weight: 10, Priority: 10,
		}
		if err := models.DB.Create(&mp).Error; err != nil {
			t.Fatalf("create association: %v", err)
		}
		if err := models.DB.Create(&models.HealthCheckLog{
			BatchID: batchID, ModelProviderID: mp.ID, Status: status,
		}).Error; err != nil {
			t.Fatalf("create health check log: %v", err)
		}
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "batchId", Value: batchID}}
	c.Request = httptest.NewRequest("GET", "/health-check/batch/"+batchID, nil)

	GetBatchHealthCheckStatus(c)

	body := w.Body.String()
	if !gjson.Get(body, "data.not_measured").Exists() {
		t.Fatalf("响应缺 not_measured 键（三态必须单列，键恒在）:\n%s", body)
	}

	success := gjson.Get(body, "data.success").Int()
	failed := gjson.Get(body, "data.failed").Int()
	notMeasured := gjson.Get(body, "data.not_measured").Int()
	pending := gjson.Get(body, "data.pending").Int()
	total := gjson.Get(body, "data.total_count").Int()

	if success != 1 || failed != 1 || notMeasured != 1 {
		t.Errorf("success/failed/not_measured = %d/%d/%d，want 1/1/1（未完成不得并进失败）:\n%s",
			success, failed, notMeasured, body)
	}
	if sum := success + failed + notMeasured + pending; sum != total {
		t.Errorf("四项之和 = %d, want total_count = %d（单列后必须仍与总数加得平）:\n%s", sum, total, body)
	}
	if !gjson.Get(body, "data.completed").Bool() {
		t.Errorf("completed = false, want true（三个关联都已测过）:\n%s", body)
	}
}

// TestRunHealthCheckAll_SettingsUnreadable_FailsBeforeStarting 启动前同步探配置（Stage 36 Ruling 1）：
// 这个接口是先返回批次号、真正干活在后台，读失败若留到后台，前端只会看到一个永远不完成的批次
// （轮询无超时）。故配置读不出来时必须当场失败、且**不得**返回批次号。
func TestRunHealthCheckAll_SettingsUnreadable_FailsBeforeStarting(t *testing.T) {
	testsupport.InitTestDB(t)
	if err := models.DB.Exec("DROP TABLE settings").Error; err != nil {
		t.Fatalf("drop settings table: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/health-check/run-all", nil)

	RunHealthCheckAll(c)

	if w.Code != 500 {
		t.Fatalf("status = %d, want 500（配置读不出来不得启动整轮检测）body=%s", w.Code, w.Body.String())
	}
	if gjson.Get(w.Body.String(), "data.batch_id").Exists() {
		t.Errorf("返回了批次号，前台会去轮询一个永不完成的批次:\n%s", w.Body.String())
	}
}
