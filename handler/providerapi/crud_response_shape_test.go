package providerapi

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/tidwall/gjson"
)

// responseKeyContract 描述供应商结构化 API 响应键契约（打在响应体 JSON 上）。
type responseKeyContract struct {
	key     string
	boolKey bool // bool 字段：存在即可，值为 true/false 均合法
	nilable bool
}

// providerDetailResponseKeys Create / Update / Get-by-id 共用详情形状。
var providerDetailResponseKeys = []responseKeyContract{
	{key: "ID"},
	{key: "Name"},
	{key: "Type"},
	{key: "Config"},
	{key: "Console"},
	{key: "Proxy"},
	{key: "ModelEndpoint", boolKey: true},
	{key: "ModelFilterEnabled", boolKey: true},
	{key: "AuthType", nilable: true},
	{key: "blacklisted", boolKey: true},
	{key: "Protocols"},
	{key: "Endpoints"},
	{key: "Groups"},
}

var providerListItemResponseKeys = []responseKeyContract{
	{key: "ID"},
	{key: "Name"},
	{key: "Protocols"},
	{key: "EndpointCount"},
	{key: "GroupCount"},
}

var endpointNestedKeys = []responseKeyContract{
	{key: "ID"},
	{key: "ProviderID"},
	{key: "Protocol"},
	{key: "URL"},
	{key: "Enabled", boolKey: true},
}

var groupNestedKeys = []responseKeyContract{
	{key: "ID"},
	{key: "ProviderID"},
	{key: "Name"},
	{key: "Weight"},
	{key: "Models"},
	{key: "PoolID", nilable: true},
	{key: "Source"},
	{key: "InlineKeys"},
}

func assertResponseKeys(t *testing.T, raw, prefix string, keys []responseKeyContract) {
	t.Helper()
	for _, k := range keys {
		path := prefix + k.key
		got := gjson.Get(raw, path)
		if !got.Exists() {
			t.Fatalf("%s 键缺失（三态字段禁 omitempty）。raw=%s", path, raw)
		}
		if k.nilable && got.Type == gjson.Null {
			continue
		}
		if k.boolKey {
			if got.Type != gjson.True && got.Type != gjson.False {
				t.Fatalf("%s type=%v want bool raw=%s", path, got.Type, raw)
			}
			continue
		}
		if got.Type == gjson.Null {
			t.Fatalf("%s is null but not nilable raw=%s", path, raw)
		}
	}
}

func listProvidersViaHandler(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/providers", nil)
	GetProviders(c)
	if w.Code != 200 {
		t.Fatalf("GetProviders status=%d body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

func getProviderViaHandler(t *testing.T, id uint) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(id), 10)}}
	c.Request = httptest.NewRequest("GET", "/providers/"+strconv.FormatUint(uint64(id), 10), nil)
	GetProvider(c)
	if w.Code != 200 {
		t.Fatalf("GetProvider status=%d body=%s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

// TestProviderStructuredResponse_KeysPresent 锁定 Create/Update/List/Get 四路径键契约。
func TestProviderStructuredResponse_KeysPresent(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	createBody := createProviderViaHandler(t, structuredCreateBody("p-shape", "sk-shape-CCCC3333"))
	id := uint(gjson.Get(createBody, "data.ID").Uint())
	if id == 0 {
		t.Fatalf("create missing ID: %s", createBody)
	}

	updateBody := updateProviderViaHandler(t, id, structuredCreateBody("p-shape", "sk-shape-CCCC3333"))
	getBody := getProviderViaHandler(t, id)
	listBody := listProvidersViaHandler(t)

	routes := []struct {
		name   string
		prefix string
		body   string
		keys   []responseKeyContract
	}{
		{name: "create", prefix: "data.", body: createBody, keys: providerDetailResponseKeys},
		{name: "update", prefix: "data.", body: updateBody, keys: providerDetailResponseKeys},
		{name: "get", prefix: "data.", body: getBody, keys: providerDetailResponseKeys},
		{name: "list", prefix: "data.0.", body: listBody, keys: providerListItemResponseKeys},
	}
	for _, r := range routes {
		t.Run(r.name, func(t *testing.T) {
			assertResponseKeys(t, r.body, r.prefix, r.keys)
		})
	}

	// list 安全不变量：列表绝不展开 Groups / InlineKeys（明文 key 只出现在详情/写路径）。
	// 负向断言，防止未来把详情结构误并入 ProviderListItem 泄漏凭据。
	if gjson.Get(listBody, "data.0.Groups").Exists() {
		t.Fatalf("list 不应含 Groups：列表项不得展开分组/明文 key。body=%s", listBody)
	}
	if gjson.Get(listBody, "data.0.InlineKeys").Exists() {
		t.Fatalf("list 不应含 InlineKeys：列表项不得携带明文 key。body=%s", listBody)
	}

	// 嵌套 endpoints / groups（详情路径）
	for _, name := range []string{"create", "update", "get"} {
		var body string
		switch name {
		case "create":
			body = createBody
		case "update":
			body = updateBody
		default:
			body = getBody
		}
		t.Run(name+"/nested", func(t *testing.T) {
			assertResponseKeys(t, body, "data.Endpoints.0.", endpointNestedKeys)
			assertResponseKeys(t, body, "data.Groups.0.", groupNestedKeys)
			poolID := gjson.Get(body, "data.Groups.0.PoolID")
			if !poolID.Exists() || poolID.Type != gjson.Null {
				t.Fatalf("inline group PoolID must exist as null, got %s", poolID.Raw)
			}
		})
	}

	if gjson.Get(listBody, "data.0.EndpointCount").Int() != 1 {
		t.Fatalf("EndpointCount = %s", gjson.Get(listBody, "data.0.EndpointCount").Raw)
	}
	if gjson.Get(listBody, "data.0.GroupCount").Int() != 1 {
		t.Fatalf("GroupCount = %s", gjson.Get(listBody, "data.0.GroupCount").Raw)
	}
}
