package associations

import (
	"testing"

	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// responseKeyContract 描述 /api/model-providers 直返实体 models.ModelWithProvider
// 对外暴露的一个响应键的契约。
//
// key 必须与前端 webui/src/lib/api/modules/catalog/model-providers.ts 的
// ModelWithProvider interface 字段名逐字一致：该实体多数字段无 json tag，
// 序列化走 Go 字段名（PascalCase）。键名失配不会报错，前端只会静默读到
// undefined——这类断层唯一可见的地方就是响应体 JSON，故断言必须打在响应体上。
type responseKeyContract struct {
	key string
	// wantSetType 字段有值时的 JSON 类型。bool 字段的用例统一置 true，故写 gjson.True。
	wantSetType gjson.Type
	// nilable 标记该键承载「继承 / 未设置」这一有效语义：未设置时必须序列化成
	// JSON null 且键仍存在，禁止 omitempty 省略整键（省略会让前端把继承误判为
	// override，并在保存时回写错误值）。
	nilable bool
}

// modelWithProviderResponseKeys 是 /api/model-providers 响应契约的单一数据源。
// 给 models.ModelWithProvider 增删字段时只需往这张表加一行（OCP），
// 无需再写一个专属断言函数。
var modelWithProviderResponseKeys = []responseKeyContract{
	{key: "ID", wantSetType: gjson.Number},
	{key: "ModelID", wantSetType: gjson.Number},
	{key: "ProviderModel", wantSetType: gjson.String},
	{key: "ProviderID", wantSetType: gjson.Number},
	{key: "ToolCall", wantSetType: gjson.True, nilable: true},
	{key: "StructuredOutput", wantSetType: gjson.True, nilable: true},
	{key: "Image", wantSetType: gjson.True, nilable: true},
	{key: "WithHeader", wantSetType: gjson.True, nilable: true},
	{key: "Status", wantSetType: gjson.True, nilable: true},
	{key: "CustomerHeaders", wantSetType: gjson.JSON, nilable: true},
	{key: "Weight", wantSetType: gjson.Number},
	{key: "Priority", wantSetType: gjson.Number},
	{key: "MaxTokens", wantSetType: gjson.Number, nilable: true},
	{key: "ConsecutiveFailures", wantSetType: gjson.Number},
	{key: "SupportsThinking", wantSetType: gjson.True, nilable: true},
	{key: "ThinkingLevels", wantSetType: gjson.JSON, nilable: true},
}

// setAllFieldsBody 是把所有可写字段都置为非零/非 nil 的请求体，
// 用于断言"有值态"下每个响应键的存在性与 JSON 类型。
const setAllFieldsBody = `{"model_id":1,"provider_id":1,"provider_model":"pm-set",` +
	`"tool_call":true,"structured_output":true,"image":true,"with_header":true,` +
	`"customer_headers":{"X-Test":"1"},"weight":10,"priority":10,"max_tokens":8192,` +
	`"supports_thinking":true,"thinking_levels":["low","high"]}`

// TestModelWithProviderResponse_KeysPresent_WhenSet 覆盖 Create / Update / List
// 三条响应路径：前端这三处读的是同一个 interface，键名契约必须三处一致。
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
		name   string
		prefix string
		body   string
	}{
		{name: "create", prefix: "data.", body: createBody},
		{name: "update", prefix: "data.", body: updateAssocViaHandler(t, id, setAllFieldsBody)},
		{name: "list", prefix: "data.0.", body: listAssocsViaHandler(t, 1)},
	}

	for _, route := range routes {
		for _, kc := range modelWithProviderResponseKeys {
			t.Run(route.name+"/"+kc.key, func(t *testing.T) {
				assertResponseKeyShape(t, route.body, route.prefix+kc.key, kc.wantSetType)
			})
		}
	}
}

// TestModelWithProviderResponse_KeysPresent_WhenUnset 是 omitempty 类断层的主防线：
// 三态字段全为 nil 时，键必须仍在且为 JSON null。
//
// 记录经 repository 直造而非 handler：Create / Update 会把 ToolCall /
// StructuredOutput / Image / WithHeader 一律赋成非 nil 指针（请求 DTO 用的是 bool），
// 走 handler 构造不出"全继承"这一行。本用例只测序列化层形状，与写入路径无关。
func TestModelWithProviderResponse_KeysPresent_WhenUnset(t *testing.T) {
	testsupport.InitTestDB(t)

	mp := models.ModelWithProvider{ModelID: 1, ProviderID: 1, ProviderModel: "pm-unset"}
	if err := repository.Default().ModelWithProvider.Create(t.Context(), &mp); err != nil {
		t.Fatalf("create model provider: %v", err)
	}

	listBody := listAssocsViaHandler(t, 1)

	for _, kc := range modelWithProviderResponseKeys {
		t.Run(kc.key, func(t *testing.T) {
			want := kc.wantSetType
			if kc.nilable {
				want = gjson.Null
			}
			assertResponseKeyShape(t, listBody, "data.0."+kc.key, want)
		})
	}
}

// assertResponseKeyShape 断言响应体指定路径上的键存在，且 JSON 类型符合预期。
//
// 用 Exists() 而非"取值是否为空"：键缺失与键存在且为 null 在取值上无法区分，
// 只断取值会让 omitempty 类断层全部漏过。
func assertResponseKeyShape(t *testing.T, body, path string, wantType gjson.Type) {
	t.Helper()
	got := gjson.Get(body, path)
	if !got.Exists() {
		t.Fatalf("%s 键缺失：直返实体的响应键必须始终存在（三态字段禁 omitempty，nil 序列化为 null）。"+
			"键缺失时前端读到 undefined，会把继承误判成 override 并回写错误值。body=%s", path, body)
	}
	if got.Type != wantType {
		t.Fatalf("%s type = %v, want %v, raw=%s", path, got.Type, wantType, got.Raw)
	}
}
