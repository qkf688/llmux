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
	{Key: "auto_associate", WantSetType: gjson.True},
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

// serveModels 建「全字段置值 + 真三态字段全 nil、白名单空」两行夹具并打一遍 GetModels。
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
		AutoAssociate:    true,
		SupportsThinking: true,
		ThinkingLevels:   []string{"low", "high"},
		ToolCall:         true,
		StructuredOutput: true,
		Image:            true,
	}
	if err := repository.Default().Model.Create(t.Context(), &full); err != nil {
		t.Fatalf("create full model: %v", err)
	}
	// 三态字段全 nil：IOLog / IsUpstream 必须序列化成 null 且键仍在（禁 omitempty 类断层），
	// thinking_levels 空切片整键不出现。
	// 能力真值三项在 Model 上是二态 bool（非三态，无 null 语义），按表头约定统一置 true；
	// auto_associate 同为二态（假三态已收回），按表头约定置 true。
	nulls := models.Model{
		Name:             "m-nulls",
		AutoAssociate:    true,
		SupportsThinking: true,
		ToolCall:         true,
		StructuredOutput: true,
		Image:            true,
	}
	if err := repository.Default().Model.Create(t.Context(), &nulls); err != nil {
		t.Fatalf("create nulls model: %v", err)
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
//
// 两条实体断言各自带三态方向（有值态 / 空值态）：`m-full` 行的两个 Nilable 键
// （IOLog / IsUpstream）必须真有值、`m-nulls` 行必须为 null；auto_associate 与能力
// 真值三项同为二态，标 Nilable 会被方向校验放过 null（假三态已收回，见表内键定义）。
// 只断键集合时，把它们在有值行里悄悄写成 null 也是绿的——与 Stage 41 给
// model-providers 修掉的是同一类假绿，此处一并关掉。
func TestGetModels_ResponseShapeLocked(t *testing.T) {
	body := serveModels(t)

	testutil.AssertAPIResponseKeys(t, body, "", realModelsEnvelopeKeys)
	testutil.AssertAPIResponseKeysInState(t, body, `data.#(Name=="m-full")`, realModelKeysWithLevels, testutil.KeyStateSet)
	testutil.AssertAPIResponseKeysInState(t, body, `data.#(Name=="m-nulls")`, realModelKeysWithoutLevels, testutil.KeyStateUnset)
}

// TestGetModels_EmptyListKeepsDataKey 锁「空列表时 data 键仍在且是数组」。
//
// 背景（2026-10-12，线 B #2）：find-issues `2026-09-29-httpresp-Data-omitempty空列表缺data键.md`
// 声称空列表会让 `data` 键整个消失（前端读到 undefined、`.map()` 炸）。**实测是误报**——
// 该文档把「类型化 nil 切片」与「untyped nil」当成一回事。三层取证：
//  1. `Data any` 上的 omitempty 判的是**接口是否为 nil**，不是底层值：类型化 nil 切片
//     序列化成 `"data":null`（键在），只有「从未赋值」才省略键；
//  2. GORM `Find` 零结果返回**非 nil 空切片**（`rows == nil` 为 false）→ `"data":[]`；
//  3. 本用例即端到端实测：空库打 GetModels，`data` 键在且是数组。
//
// 键缺失只发生在 Data 从未赋值的错误响应上（BadRequest / NotFound / Unauthorized /
// InternalErrorMessage 都不写 Data），那是**有意**契约（成功带 data、失败带 error）。
//
// 所以本用例不修任何生产代码，而是把这个既成形状**锁住**：将来若有人给 Response 改标签、
// 或用 `c.JSON(200, gin.H{...})` 绕开 httpresp 手搓，空列表形状先在这里变红。
// 断言同时要 `Exists()` 与 `IsArray()`：前者防键缺失，后者防退化成 `null`。
func TestGetModels_EmptyListKeepsDataKey(t *testing.T) {
	testsupport.InitTestDB(t) // 建空库，不插任何模型
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/models", nil)
	GetModels(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()

	data := gjson.Get(body, "data")
	if !data.Exists() {
		t.Fatalf("空列表响应缺 data 键（前端会读到 undefined）: %s", body)
	}
	if !data.IsArray() {
		t.Fatalf("空列表的 data 不是数组（应为 [] 而非 null）: type=%v body=%s", data.Type, body)
	}
	if n := len(data.Array()); n != 0 {
		t.Fatalf("空库列表长度 = %d, want 0; body=%s", n, body)
	}
}
