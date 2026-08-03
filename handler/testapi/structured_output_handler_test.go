package testapi

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

// 守护契约：providers.New 失败时，structured 链路返回 code=200 信封，
// data.passed=false + data.error_type="provider"（该分支错误类型归属供应商配置）。
func TestTestStructuredOutputHandler_ProviderCreationFailure(t *testing.T) {
	testsupport.InitTestDB(t)

	provider := models.Provider{Name: "broken-provider", Type: "nonexistent-type"}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	mwp := models.ModelWithProvider{ModelID: 1, ProviderModel: "gpt-x", ProviderID: provider.ID}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model with provider: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/test/structured-output/1", nil)
	c.Params = []gin.Param{{Key: "id", Value: strconv.FormatUint(uint64(mwp.ID), 10)}}

	TestStructuredOutputHandler(c)

	var payload testsupport.APIEnvelope[map[string]any]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if passed, ok := payload.Data["passed"].(bool); !ok || passed {
		t.Fatalf("data.passed = %v, want false, body=%s", payload.Data["passed"], w.Body.String())
	}
	if errorType, ok := payload.Data["error_type"].(string); !ok || errorType != "provider" {
		t.Fatalf("data.error_type = %v, want provider, body=%s", payload.Data["error_type"], w.Body.String())
	}
	errorText, ok := payload.Data["error"].(string)
	if !ok || !strings.Contains(errorText, "创建提供商失败") {
		t.Fatalf("data.error = %q, want contains 创建提供商失败, body=%s", payload.Data["error"], w.Body.String())
	}
}
