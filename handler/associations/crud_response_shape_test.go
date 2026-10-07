package associations

import (
	"testing"

	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// modelWithProviderResponseKeys 是 /api/model-providers 响应契约的单一数据源。
// 给 models.ModelWithProvider 增删字段时只需往这张表加一行（OCP），
// 无需再写一个专属断言函数。
//
// key 必须与前端 webui/src/lib/api/modules/catalog/model-providers.ts 的
// ModelWithProvider interface 字段名逐字一致：该实体多数字段无 json tag，
// 序列化走 Go 字段名（PascalCase）。键名失配不会报错，前端只会静默读到
// undefined——这类断层唯一可见的地方就是响应体 JSON，故断言必须打在响应体上。
//
// 键集合的**双向**锁定（少键与多键都红）由 testutil.AssertAPIResponseKeysInState
// 提供：只逐键断「存在 + 类型」时，实体新增字段导致的响应多键无人发现。
var modelWithProviderResponseKeys = []testutil.APIResponseKey{
	{Key: "ID", WantSetType: gjson.Number},
	// gorm.Model 的三个托管键：本实体嵌的是 gorm.Model，故响应必然含这三个键
	// （models.Model 则显式声明 ID/CreatedAt/UpdatedAt、根本没有 DeletedAt）。
	// 只逐键断业务字段时它们「不在表里也全绿」——键集合逐字相等一上就把这 3 个缺键暴露出来。
	{Key: "CreatedAt", WantSetType: gjson.String},
	{Key: "UpdatedAt", WantSetType: gjson.String},
	{Key: "DeletedAt", WantSetType: gjson.String, Nilable: true, AlwaysNull: true},
	{Key: "ModelID", WantSetType: gjson.Number},
	{Key: "ProviderModel", WantSetType: gjson.String},
	{Key: "ProviderID", WantSetType: gjson.Number},
	{Key: "ToolCall", WantSetType: gjson.True, Nilable: true},
	{Key: "StructuredOutput", WantSetType: gjson.True, Nilable: true},
	{Key: "Image", WantSetType: gjson.True, Nilable: true},
	{Key: "WithHeader", WantSetType: gjson.True, Nilable: true},
	{Key: "Status", WantSetType: gjson.True, Nilable: true},
	{Key: "CustomerHeaders", WantSetType: gjson.JSON, Nilable: true},
	{Key: "Weight", WantSetType: gjson.Number},
	{Key: "Priority", WantSetType: gjson.Number},
	{Key: "MaxTokens", WantSetType: gjson.Number, Nilable: true},
	{Key: "ConsecutiveFailures", WantSetType: gjson.Number},
	{Key: "SupportsThinking", WantSetType: gjson.True, Nilable: true},
	{Key: "ThinkingLevels", WantSetType: gjson.JSON, Nilable: true},
}

// setAllFieldsBody 是把所有可写字段都置为非零/非 nil 的请求体，
// 用于断言「有值态」下每个响应键的存在性、JSON 类型与三态方向。
const setAllFieldsBody = `{"model_id":1,"provider_id":1,"provider_model":"pm-set",` +
	`"tool_call":true,"structured_output":true,"image":true,"with_header":true,` +
	`"customer_headers":{"X-Test":"1"},"weight":10,"priority":10,"max_tokens":8192,` +
	`"supports_thinking":true,"thinking_levels":["low","high"]}`

// TestModelWithProviderResponse_KeysPresent_WhenSet 覆盖 Create / Update / List
// 三条响应路径：前端这三处读的是同一个 interface，键名契约必须三处一致。
//
// 走 KeyStateSet：三态键在有值态下必须真有值，写回 null 也要红——否则「有值」这个
// 前提没人守，键的类型漂移能从「null 也算过」这条出口溜过去。
func TestModelWithProviderResponse_KeysPresent_WhenSet(t *testing.T) {
	testsupport.InitTestDB(t)
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)

	createBody := createAssocViaHandler(t, setAllFieldsBody)
	id := uint(gjson.Get(createBody, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("create 响应未返回 ID，body=%s", createBody)
	}

	routes := []struct {
		name string
		path string
		body string
	}{
		{name: "create", path: "data", body: createBody},
		{name: "update", path: "data", body: updateAssocViaHandler(t, id, setAllFieldsBody)},
		{name: "list", path: "data.0", body: listAssocsViaHandler(t, 1)},
	}

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			testutil.AssertAPIResponseKeysInState(t, route.body, route.path,
				modelWithProviderResponseKeys, testutil.KeyStateSet)
		})
	}
}

// TestModelWithProviderResponse_KeysPresent_WhenUnset 是 omitempty 类断层的主防线：
// 三态字段全为 nil 时，键必须仍在且为 JSON null。
//
// 走 KeyStateUnset：三态键在空值态下必须为 null——被悄悄赋上值（继承语义写成显式值）
// 也要红，那正是前端会把「继承」误判成 override 的形态。
//
// 记录经 repository 直造而非 handler：Create / Update 会把 ToolCall /
// StructuredOutput / Image / WithHeader 一律赋成非 nil 指针（请求 DTO 用的是 bool），
// 走 handler 构造不出「全继承」这一行。本用例只测序列化层形状，与写入路径无关。
func TestModelWithProviderResponse_KeysPresent_WhenUnset(t *testing.T) {
	testsupport.InitTestDB(t)

	mp := models.ModelWithProvider{ModelID: 1, ProviderID: 1, ProviderModel: "pm-unset"}
	if err := repository.Default().ModelWithProvider.Create(t.Context(), &mp); err != nil {
		t.Fatalf("create model provider: %v", err)
	}

	testutil.AssertAPIResponseKeysInState(t, listAssocsViaHandler(t, 1), "data.0",
		modelWithProviderResponseKeys, testutil.KeyStateUnset)
}
