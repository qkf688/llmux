package modelapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// realModelsEnvelopeKeys 是 /api/models（真实模型目录）统一信封的键契约。
// 成功响应恒为 {code, message, data}——error/data 都带 omitempty，成功态不得出现
// error，非空列表必须出现 data（键集合逐字相等把这两个漂移一起锁死）。
var realModelsEnvelopeKeys = []testutil.APIResponseKey{
	{Key: "code", WantSetType: gjson.Number},
	{Key: "message", WantSetType: gjson.String},
	{Key: "data", WantSetType: gjson.JSON},
}

// realModelKeysWithLevels 是直返实体 models.Model「有值态」响应键契约的单一数据源
// （Stage 35 契约形状字面量断言）。
//
// 键名与前端 webui/src/lib/api/modules/catalog/models.ts 的 Model interface 逐字
// 核对：多数字段无 json tag 走 PascalCase，仅 auto_associate / supports_thinking /
// thinking_levels 三个存量字段带 snake_case tag，形状是**混合的**——不要假设整体
// 统一；同名 thinking 语义在 ModelWithProvider 上反而是 PascalCase（ThinkingLevels），
// 跨实体复制字段时逐一核对（AGENTS §5）。bool 字段的用例统一置 true（gjson.True 与
// gjson.False 是两种类型，类型断言不为「真假」服务，只为「字段类型没变」服务）。
var realModelKeysWithLevels = []testutil.APIResponseKey{
	{Key: "ID", WantSetType: gjson.Number},
	{Key: "CreatedAt", WantSetType: gjson.String},
	{Key: "UpdatedAt", WantSetType: gjson.String},
	{Key: "Name", WantSetType: gjson.String},
	{Key: "Remark", WantSetType: gjson.String},
	{Key: "IOLog", WantSetType: gjson.True, Nilable: true},
	{Key: "IsUpstream", WantSetType: gjson.True, Nilable: true},
	{Key: "auto_associate", WantSetType: gjson.True, Nilable: true},
	{Key: "supports_thinking", WantSetType: gjson.True},
	{Key: "thinking_levels", WantSetType: gjson.JSON},
	// 能力真值三项：Model 层为二态 bool（无三态语义），键名走 snake_case tag。
	{Key: "tool_call", WantSetType: gjson.True},
	{Key: "structured_output", WantSetType: gjson.True},
	{Key: "image", WantSetType: gjson.True},
}

// realModelKeysWithoutLevels 空值态表 = 有值态表去掉 omitempty 契约键 thinking_levels。
// 空切片时整键不出现是序列化契约的一部分（omitempty 在 models.Model 上），「键出不
// 出现」本身必须被字面量锁住：把表派生出来而不是复制一份，是防两张表悄悄走散。
var realModelKeysWithoutLevels = withoutKey(realModelKeysWithLevels, "thinking_levels")

func withoutKey(keys []testutil.APIResponseKey, drop string) []testutil.APIResponseKey {
	out := make([]testutil.APIResponseKey, 0, len(keys))
	for _, k := range keys {
		if k.Key == drop {
			continue
		}
		out = append(out, k)
	}
	return out
}

// serveModels 建「全字段置值 + 三态字段全 nil、白名单空」两行夹具并打一遍 GetModels。
// 两行分别覆盖 thinking_levels 的有值态（键在）与空值态（键不出现）。
func serveModels(t *testing.T) string {
	t.Helper()
	testsupport.InitTestDB(t)
	gin.SetMode(gin.TestMode)

	on := true
	full := models.Model{
		Name:             "m-full",
		Remark:           "r",
		IOLog:            &on,
		IsUpstream:       &on,
		AutoAssociate:    &on,
		SupportsThinking: true,
		ThinkingLevels:   []string{"low", "high"},
		ToolCall:         true,
		StructuredOutput: true,
		Image:            true,
	}
	if err := repository.Default().Model.Create(t.Context(), &full); err != nil {
		t.Fatalf("create full model: %v", err)
	}
	// 三态字段全 nil：IOLog / IsUpstream / auto_associate 必须序列化成 null 且键仍在
	//（禁 omitempty 类断层），thinking_levels 空切片整键不出现。
	// 能力真值三项在 Model 上是二态 bool（非三态，无 null 语义），按表头约定统一置 true。
	nulls := models.Model{
		Name:             "m-nulls",
		SupportsThinking: true,
		ToolCall:         true,
		StructuredOutput: true,
		Image:            true,
	}
	if err := repository.Default().Model.Create(t.Context(), &nulls); err != nil {
		t.Fatalf("create nulls model: %v", err)
	}
	// auto_associate 列带 gorm default:true：建行时 nil 会被数据库默认值回填成 true，
	// 构造不出「未设置」态——必须显式写 NULL 才能覆盖到 nil 序列化成 null 的契约面。
	if _, err := repository.Default().Model.UpdateFields(t.Context(), nulls.ID, map[string]any{"auto_associate": nil}); err != nil {
		t.Fatalf("null out auto_associate: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/models", nil)
	GetModels(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// TestGetModels_ResponseShapeLocked 锁 /api/models 信封与实体两级键集合：
// 缺键、多键、改名都红。此前该端点的测试只覆盖字段值，不锁响应键集合——
// 实体增删字段/改 json tag 时前端 interface 断层不会被任何测试抓到。
func TestGetModels_ResponseShapeLocked(t *testing.T) {
	body := serveModels(t)

	testutil.AssertAPIResponseKeys(t, body, "", realModelsEnvelopeKeys)
	testutil.AssertAPIResponseKeys(t, body, `data.#(Name=="m-full")`, realModelKeysWithLevels)
	testutil.AssertAPIResponseKeys(t, body, `data.#(Name=="m-nulls")`, realModelKeysWithoutLevels)
}
