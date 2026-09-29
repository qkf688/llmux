package associations

import (
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// TestGetModelProviderHealthStatus_TriStateWireContract 关联健康趋势的 wire 契约（Stage 36）：
// 返回的是**状态字符串数组**（success / error / not_measured），按时间升序。
// 压成布尔会让前端把「本机没测成」画成上游失败，与判停口径（本机原因不算上游失败）
// 自相矛盾；断言必须打在响应体 JSON 上，只断 Go 值测不出形状漂移。
func TestGetModelProviderHealthStatus_TriStateWireContract(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createDisabledAssoc(t, 0)

	base := time.Now()
	for i, status := range []string{
		models.HealthCheckStatusSuccess,
		models.HealthCheckStatusError,
		models.HealthCheckStatusNotMeasured,
	} {
		if err := models.DB.Create(&models.HealthCheckLog{
			ModelProviderID: mp.ID, Status: status, CheckedAt: base.Add(time.Duration(i) * time.Minute),
		}).Error; err != nil {
			t.Fatalf("create health check log: %v", err)
		}
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET",
		"/model-providers/health-status?model_provider_id="+strconv.FormatUint(uint64(mp.ID), 10)+"&limit=10", nil)

	GetModelProviderHealthStatus(c)

	body := w.Body.String()
	if got := gjson.Get(body, "data.0"); got.Type != gjson.String {
		t.Fatalf("data.0 类型 = %v (%s)，want 字符串状态（布尔数组会把本机没测成画成上游失败）:\n%s",
			got.Type, got.String(), body)
	}

	want := []string{
		models.HealthCheckStatusSuccess,
		models.HealthCheckStatusError,
		models.HealthCheckStatusNotMeasured,
	}
	for i, wantStatus := range want {
		path := fmt.Sprintf("data.%d", i)
		if got := gjson.Get(body, path).String(); got != wantStatus {
			t.Errorf("%s = %q, want %q（时间升序且三态原样透出）:\n%s", path, got, wantStatus, body)
		}
	}
}
