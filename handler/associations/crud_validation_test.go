package associations

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// TestUpdateModelProvider_RejectsDanglingModelID 回归：Update 路径校验 model_id 存在，
// 不允许改写为不存在的 model（避免悬空关联）。
func TestUpdateModelProvider_RejectsDanglingModelID(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t) // 预置 model 1 + provider 1 + 关联

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(mp.ID), 10)}}
	// model_id=999 不存在
	c.Request = httptest.NewRequest("PUT", "/model-providers/"+strconv.FormatUint(uint64(mp.ID), 10),
		strings.NewReader(`{"model_id":999,"provider_id":1,"provider_name":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192}`))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateModelProvider(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200 (envelope), body=%s", w.Code, w.Body.String())
	}
	if code := gjson.Get(w.Body.String(), "code").Int(); code != 400 {
		t.Fatalf("envelope code = %d, want 400, body=%s", code, w.Body.String())
	}
}

// TestUpdateModelProvider_RejectsDanglingProviderID 回归：Update 路径校验 provider_id 存在。
func TestUpdateModelProvider_RejectsDanglingProviderID(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(mp.ID), 10)}}
	// provider_id=999 不存在
	c.Request = httptest.NewRequest("PUT", "/model-providers/"+strconv.FormatUint(uint64(mp.ID), 10),
		strings.NewReader(`{"model_id":1,"provider_id":999,"provider_name":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192}`))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateModelProvider(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200 (envelope), body=%s", w.Code, w.Body.String())
	}
	if code := gjson.Get(w.Body.String(), "code").Int(); code != 400 {
		t.Fatalf("envelope code = %d, want 400, body=%s", code, w.Body.String())
	}
}

// TestUpdateModelProvider_ZeroIDsSkipped 回归：model_id=0/provider_id=0 时不校验（GORM 跳过零值=不改）。
func TestUpdateModelProvider_ZeroIDsSkipped(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(mp.ID), 10)}}
	// model_id=0/provider_id=0 → 不校验，GORM 跳过零值保持原值
	c.Request = httptest.NewRequest("PUT", "/model-providers/"+strconv.FormatUint(uint64(mp.ID), 10),
		strings.NewReader(`{"model_id":0,"provider_id":0,"provider_name":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":20,"priority":20,"max_tokens":4096}`))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateModelProvider(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	got, err := repository.Default().ModelWithProvider.Get(t.Context(), mp.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.ModelID != 1 || got.ProviderID != 1 {
		t.Fatalf("ModelID=%d ProviderID=%d, want 1/1 (zero skipped)", got.ModelID, got.ProviderID)
	}
	if got.Weight != 20 {
		t.Fatalf("Weight=%d, want 20 (updated)", got.Weight)
	}
}

// TestCreateModelProvider_RejectsDanglingProviderID 回归：Create 路径校验 provider_id 存在。
func TestCreateModelProvider_RejectsDanglingProviderID(t *testing.T) {
	testsupport.InitTestDB(t)
	createModelForThinkingTest(t) // model 1 存在，provider 不存在

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/model-providers",
		strings.NewReader(`{"model_id":1,"provider_id":999,"provider_name":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10}`))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateModelProvider(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200 (envelope), body=%s", w.Code, w.Body.String())
	}
	if code := gjson.Get(w.Body.String(), "code").Int(); code != 400 {
		t.Fatalf("envelope code = %d, want 400, body=%s", code, w.Body.String())
	}
}

// TestCreateModelProvider_RejectsDanglingModelID 回归：Create 路径校验 model_id 存在。
func TestCreateModelProvider_RejectsDanglingModelID(t *testing.T) {
	testsupport.InitTestDB(t)
	// 不预置任何 model/provider

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/model-providers",
		strings.NewReader(`{"model_id":999,"provider_id":1,"provider_name":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10}`))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateModelProvider(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200 (envelope), body=%s", w.Code, w.Body.String())
	}
	if code := gjson.Get(w.Body.String(), "code").Int(); code != 400 {
		t.Fatalf("envelope code = %d, want 400, body=%s", code, w.Body.String())
	}
}
