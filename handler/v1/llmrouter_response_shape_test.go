package v1

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// llmRouterListKeys / llmRouterModelItemKeys 是 /v1/models（LLM 路由对外模型列表）
// 响应契约的单一数据源（Stage 35 契约形状字面量断言）。
//
// 该端点按 OpenAI 协议直返（httpresp.SuccessRaw 不包统一信封），键名是协议
// snake_case 而非 Go 字段名；任何按 OpenAI /v1/models 消费的客户端都依赖此形状，
// 改形状必须先改本表再改实现（AGENTS §5：直返实体 JSON 形状两侧同源）。
var llmRouterListKeys = []testutil.APIResponseKey{
	{Key: "object", WantSetType: gjson.String},
	{Key: "data", WantSetType: gjson.JSON},
}

var llmRouterModelItemKeys = []testutil.APIResponseKey{
	{Key: "id", WantSetType: gjson.String},
	{Key: "object", WantSetType: gjson.String},
	{Key: "created", WantSetType: gjson.Number},
	{Key: "owned_by", WantSetType: gjson.String},
}

// serveV1Models 建「真实 + 虚拟各一行」的夹具并打一遍 ModelsHandler，返回响应体。
// 两行夹具保证 owned_by 两种来源都被走到。
func serveV1Models(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	prevDB := models.DB
	models.DB = testutil.New(t, &models.Model{}, &models.VirtualModel{})
	t.Cleanup(func() { models.DB = prevDB })

	realRow := models.Model{Name: "real-1"}
	if err := models.DB.Create(&realRow).Error; err != nil {
		t.Fatalf("create real model: %v", err)
	}
	virtualRow := models.VirtualModel{Name: "virt-1"}
	if err := models.DB.Create(&virtualRow).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ModelsHandler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// TestModelsHandler_ResponseShapeLocked 锁 /v1/models 顶层与元素两级键集合：
// 缺键、多键、改名都红。此前该端点零测试覆盖，响应形状不被任何字面量断言锁定。
func TestModelsHandler_ResponseShapeLocked(t *testing.T) {
	body := serveV1Models(t)

	testutil.AssertAPIResponseKeys(t, body, "", llmRouterListKeys)

	items := gjson.Get(body, "data").Array()
	if len(items) != 2 {
		t.Fatalf("data 长度 = %d, want 2（真实模型 + 虚拟模型各一行）, body=%s", len(items), body)
	}
	for i := range items {
		testutil.AssertAPIResponseKeys(t, body, "data."+strconv.Itoa(i), llmRouterModelItemKeys)
	}
}

// TestModelsHandler_ModelItemValues 锁元素里对外有语义的字面量值：
// object 恒为 "model"、created 是 Unix 秒（数字），owned_by 区分真实/虚拟来源。
// 形状表管「键在不在」，本用例管「值的口径」——两层都属于对外契约。
func TestModelsHandler_ModelItemValues(t *testing.T) {
	body := serveV1Models(t)

	for _, item := range gjson.Get(body, "data").Array() {
		if got := item.Get("object").String(); got != "model" {
			t.Fatalf("object = %q, want %q, item=%s", got, "model", item.Raw)
		}
		created := item.Get("created")
		if created.Type != gjson.Number || created.Int() > time.Now().Unix() {
			t.Fatalf("created 不是 Unix 秒数字: %s, item=%s", created.Raw, item.Raw)
		}
	}
	if got := gjson.Get(body, "data.0.owned_by").String(); got != "llmux" {
		t.Fatalf("真实模型 owned_by = %q, want llmux", got)
	}
	if got := gjson.Get(body, "data.1.owned_by").String(); got != "llmux-virtual" {
		t.Fatalf("虚拟模型 owned_by = %q, want llmux-virtual", got)
	}
}
