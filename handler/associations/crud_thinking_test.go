package associations

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

func createAssocForThinkingTest(t *testing.T) models.ModelWithProvider {
	t.Helper()
	// 预置 model + provider，满足 Update handler 的存在性校验。
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)
	mp := models.ModelWithProvider{
		ModelID:         1,
		ProviderID:      1,
		ProviderModel:   "pm",
		Weight:          10,
		Priority:        10,
		Status:          boolPtr(true),
		MaxTokens:       intPtr(8192),
		WithHeader:      boolPtr(false),
		CustomerHeaders: map[string]string{"X-Test": "1"},
	}
	if err := repository.Default().ModelWithProvider.Create(t.Context(), &mp); err != nil {
		t.Fatalf("create model provider: %v", err)
	}
	return mp
}

// createProviderForThinkingTest 创建一个 provider（自增 ID=1）供 association 测试使用。
func createProviderForThinkingTest(t *testing.T) {
	t.Helper()
	p := models.Provider{Name: "test-provider", Type: "openai"}
	if err := repository.Default().Provider.Create(t.Context(), &p); err != nil {
		t.Fatalf("create provider: %v", err)
	}
}

func boolPtr(v bool) *bool { return &v }
func intPtr(v int) *int    { return &v }

// updateAssocViaHandler 走 Update handler 并返回响应体，便于断言响应形状；
// 只关心落库结果的用例可忽略返回值。
func updateAssocViaHandler(t *testing.T, id uint, body string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
	c.Request = httptest.NewRequest("PUT", "/model-providers/"+strconv.FormatUint(uint64(id), 10), strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateModelProvider(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// listAssocsViaHandler 走 List handler 并返回响应体，用于断言列表响应形状。
func listAssocsViaHandler(t *testing.T, modelID uint) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/model-providers?model_id="+strconv.FormatUint(uint64(modelID), 10), nil)

	GetModelProviders(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

func reloadAssoc(t *testing.T, id uint) *models.ModelWithProvider {
	t.Helper()
	got, err := repository.Default().ModelWithProvider.Get(t.Context(), id)
	if err != nil {
		t.Fatalf("reload model provider: %v", err)
	}
	return got
}

// TestUpdateModelProvider_SupportsThinkingTriState 覆盖三态契约：
// true/false 写 override；nil（缺省）写 NULL 改回"继承"。
// 同时回归"部分更新语义"：缺省字段（weight/priority/max_tokens 等）不被清零。
func TestUpdateModelProvider_SupportsThinkingTriState(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t)

	// 1) override = true
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192,"supports_thinking":true}`)
	got := reloadAssoc(t, mp.ID)
	if got.SupportsThinking == nil || !*got.SupportsThinking {
		t.Fatalf("supports_thinking = %v, want true", got.SupportsThinking)
	}

	// 2) override = false
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192,"supports_thinking":false}`)
	got = reloadAssoc(t, mp.ID)
	if got.SupportsThinking == nil || *got.SupportsThinking {
		t.Fatalf("supports_thinking = %v, want false", got.SupportsThinking)
	}

	// 3) 缺省（nil）→ 改回"继承"（NULL）
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false,"weight":10,"priority":10,"max_tokens":8192}`)
	got = reloadAssoc(t, mp.ID)
	if got.SupportsThinking != nil {
		t.Fatalf("supports_thinking = %v, want nil (inherit)", got.SupportsThinking)
	}
}

// TestUpdateModelProvider_PartialUpdateKeepsUntouchedFields 回归：
// struct 部分更新语义——缺省字段保持原值，不被清零。
func TestUpdateModelProvider_PartialUpdateKeepsUntouchedFields(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t)

	// 只提交最小字段集（缺省 weight/priority/max_tokens/customer_headers/supports_thinking）
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm","tool_call":true,"structured_output":true,"image":false,"with_header":false}`)

	got := reloadAssoc(t, mp.ID)
	if got.Weight != 10 {
		t.Fatalf("weight = %d, want 10 (untouched)", got.Weight)
	}
	if got.Priority != 10 {
		t.Fatalf("priority = %d, want 10 (untouched)", got.Priority)
	}
	if got.MaxTokens == nil || *got.MaxTokens != 8192 {
		t.Fatalf("max_tokens = %v, want 8192 (untouched)", got.MaxTokens)
	}
	if len(got.CustomerHeaders) != 1 {
		t.Fatalf("customer_headers = %v, want 1 entry (untouched)", got.CustomerHeaders)
	}
	// 契约：supports_thinking 缺省（nil）= 显式清空 override（改回继承），
	// 与 weight 等"缺省不改"的部分更新语义不同——本字段是三态且前端总是显式提交 inherit/true/false。
	if got.SupportsThinking != nil {
		t.Fatalf("supports_thinking = %v, want nil (omitted in request → cleared to inherit)", got.SupportsThinking)
	}
}
