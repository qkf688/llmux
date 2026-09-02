package providerapi

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// getModelCatalogViaHandler 走 GetProviderModelCatalog handler 并返回响应体。
func getModelCatalogViaHandler(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/providers/model-catalog", nil)

	GetProviderModelCatalog(c)

	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// TestGetProviderModelCatalog_MergesWhitelistsAndCustom 覆盖 #8-4 AC-4a：
// 每供应商返回 Upstream（分组白名单并集，CSV 去空格去重）+ Custom
// （config.custom_models）；空值显式空数组（非 null）。
func TestGetProviderModelCatalog_MergesWhitelistsAndCustom(t *testing.T) {
	testsupport.InitTestDB(t)
	repo := repository.Default()
	p1 := &models.Provider{Name: "一号商", Type: "openai", Config: `{"custom_models":["x-1"]}`}
	p2 := &models.Provider{Name: "二号商", Type: "anthropic"}
	if err := repo.Provider.Create(t.Context(), p1); err != nil {
		t.Fatalf("seed provider1: %v", err)
	}
	if err := repo.Provider.Create(t.Context(), p2); err != nil {
		t.Fatalf("seed provider2: %v", err)
	}
	for _, g := range []*models.KeyGroup{
		{ProviderID: p1.ID, Name: "低价组", Models: "gpt-4o, gpt-4o-mini"},
		{ProviderID: p1.ID, Name: "走量组", Models: "gpt-4o-mini,claude-3"},
	} {
		if err := repo.KeyGroup.Create(t.Context(), g); err != nil {
			t.Fatalf("seed key group: %v", err)
		}
	}

	body := getModelCatalogViaHandler(t)

	if got := gjson.Get(body, "data.#").Int(); got != 2 {
		t.Fatalf("目录条目数 = %d, want 2, body=%s", got, body)
	}

	// 一号商：白名单并集 3 个（CSV 空格 trim + 去重）+ custom 1 个
	p1Base := "data.#(ProviderID==" + uitoa(p1.ID) + ")."
	if got := gjson.Get(body, p1Base+"Upstream.#").Int(); got != 3 {
		t.Fatalf("一号商 Upstream 数量 = %d, want 3, body=%s", got, body)
	}
	if got := gjson.Get(body, p1Base+"Custom.#").Int(); got != 1 {
		t.Fatalf("一号商 Custom 数量 = %d, want 1, body=%s", got, body)
	}
	if got := gjson.Get(body, p1Base+"Upstream.0").String(); got != "gpt-4o" {
		t.Fatalf("一号商 Upstream.0 = %q, want gpt-4o", got)
	}
	if got := gjson.Get(body, p1Base+"Upstream.1").String(); got != "gpt-4o-mini" {
		t.Fatalf("一号商 Upstream.1 = %q, want gpt-4o-mini（CSV 空格 trim）", got)
	}
	if got := gjson.Get(body, p1Base+"Custom.0").String(); got != "x-1" {
		t.Fatalf("一号商 Custom.0 = %q, want x-1", got)
	}

	// 二号商：无分组无 custom → 双空数组（Type=JSON 非 null）
	p2Base := "data.#(ProviderID==" + uitoa(p2.ID) + ")."
	for _, key := range []string{"Upstream", "Custom"} {
		v := gjson.Get(body, p2Base+key)
		if !v.Exists() || v.Type != gjson.JSON {
			t.Fatalf("二号商 %s 应为空数组, got exists=%v type=%v, body=%s", key, v.Exists(), v.Type, body)
		}
		if got := gjson.Get(body, p2Base+key+".#").Int(); got != 0 {
			t.Fatalf("二号商 %s 数量 = %d, want 0", key, got)
		}
	}
}

// TestGetProviderModelCatalog_EmptyDB 覆盖边界：空库时 data 为空数组（非 null）。
func TestGetProviderModelCatalog_EmptyDB(t *testing.T) {
	testsupport.InitTestDB(t)

	body := getModelCatalogViaHandler(t)

	data := gjson.Get(body, "data")
	if !data.Exists() || data.Type != gjson.JSON {
		t.Fatalf("data 应为数组, got exists=%v type=%v, body=%s", data.Exists(), data.Type, body)
	}
	if got := gjson.Get(body, "data.#").Int(); got != 0 {
		t.Fatalf("空库 data 数量 = %d, want 0", got)
	}
}

// uitoa 测试内拼 gjson 查询用。
func uitoa(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
