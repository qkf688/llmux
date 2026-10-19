package associations

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/tidwall/gjson"
)

// 协议覆盖（Protocols / PrimaryProtocol）的写入契约：
//   - 有值态写进库、响应回读一致；
//   - 空切片归一为 nil（= 继承），不产生「一条协议都不许走」这种无意义状态；
//   - 主格式必须落在生效集合内（集合已设时比集合，未设时比供应商已启用协议）——
//     保存时就拒，避免「配置看着正常、请求恒 ErrEndpointUnavailable」；
//   - Update 缺省（null）= 显式清回继承（与 supports_thinking 同族）。

// patchProtocolsRaw 走窄端点（PATCH /:id/protocols）并返回响应体。
func patchProtocolsRaw(t *testing.T, id uint, body string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	idStr := strconv.FormatUint(uint64(id), 10)
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	c.Request = httptest.NewRequest("PATCH", "/model-providers/"+idStr+"/protocols", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateModelProviderProtocols(c)
	return w.Body.String()
}

// TestProtocolOverride_NarrowEndpointDoesNotTouchOtherFields 锁窄端点的存在理由：
// 只写协议两列，能力 / thinking / 请求头透传 / 自定义头一律保持原值。
// 若把一键写入改回全量 PUT，本用例会红（那正是 code-review 里的 Critical）。
func TestProtocolOverride_NarrowEndpointDoesNotTouchOtherFields(t *testing.T) {
	testsupport.InitTestDB(t)
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)

	createBody := postAssocRaw(t, true, 0, `{"model_id":1,"provider_id":1,"provider_model":"pm-narrow",`+
		`"tool_call":true,"structured_output":true,"image":true,"with_header":true,`+
		`"customer_headers":{"X-Test":"1"},"weight":10,"priority":10,"max_tokens":8192,`+
		`"supports_thinking":true,"thinking_levels":["low","high"]}`)
	if code := gjson.Get(createBody, "code").Int(); code != 200 {
		t.Fatalf("create failed: code=%d body=%s", code, createBody)
	}
	id := uint(gjson.Get(createBody, "data.ID").Uint())

	resp := patchProtocolsRaw(t, id, `{"protocols":["anthropic"],"primary_protocol":"anthropic"}`)
	if code := gjson.Get(resp, "code").Int(); code != 200 {
		t.Fatalf("patch failed: code=%d body=%s", code, resp)
	}

	got := reloadAssoc(t, id)
	if got.Protocols == nil || len(*got.Protocols) != 1 || (*got.Protocols)[0] != "anthropic" {
		t.Fatalf("Protocols = %v, want [anthropic]", got.Protocols)
	}
	if got.PrimaryProtocol == nil || *got.PrimaryProtocol != "anthropic" {
		t.Fatalf("PrimaryProtocol = %v, want anthropic", got.PrimaryProtocol)
	}
	// 以下任何一项被改写都说明窄端点退化成了全量 PUT。
	if got.WithHeader == nil || !*got.WithHeader {
		t.Fatalf("WithHeader = %v, want true（窄端点不得触碰透传开关）", got.WithHeader)
	}
	if got.SupportsThinking == nil || !*got.SupportsThinking {
		t.Fatalf("SupportsThinking = %v, want true", got.SupportsThinking)
	}
	if got.ThinkingLevels == nil || len(*got.ThinkingLevels) != 2 {
		t.Fatalf("ThinkingLevels = %v, want [low high]", got.ThinkingLevels)
	}
	if got.ToolCall == nil || !*got.ToolCall || got.StructuredOutput == nil || !*got.StructuredOutput ||
		got.Image == nil || !*got.Image {
		t.Fatalf("capabilities = %v/%v/%v, want true/true/true",
			got.ToolCall, got.StructuredOutput, got.Image)
	}
	if got.CustomerHeaders["X-Test"] != "1" {
		t.Fatalf("CustomerHeaders = %v, want X-Test=1", got.CustomerHeaders)
	}
}

func TestProtocolOverride_CreateAndReadBack(t *testing.T) {
	testsupport.InitTestDB(t)
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)

	body := postAssocRaw(t, true, 0, `{"model_id":1,"provider_id":1,"provider_model":"pm-protocol",`+
		`"weight":10,"priority":10,"protocols":["openai","anthropic"],"primary_protocol":"anthropic"}`)
	if code := gjson.Get(body, "code").Int(); code != 200 {
		t.Fatalf("create failed: code=%d body=%s", code, body)
	}
	got := reloadAssoc(t, uint(gjson.Get(body, "data.ID").Uint()))
	if got.Protocols == nil || len(*got.Protocols) != 2 {
		t.Fatalf("Protocols = %v, want [openai anthropic]", got.Protocols)
	}
	if got.PrimaryProtocol == nil || *got.PrimaryProtocol != "anthropic" {
		t.Fatalf("PrimaryProtocol = %v, want anthropic", got.PrimaryProtocol)
	}
}

func TestProtocolOverride_EmptySetNormalizesToInherit(t *testing.T) {
	testsupport.InitTestDB(t)
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)

	body := postAssocRaw(t, true, 0, `{"model_id":1,"provider_id":1,"provider_model":"pm-empty",`+
		`"weight":10,"priority":10,"protocols":[]}`)
	if code := gjson.Get(body, "code").Int(); code != 200 {
		t.Fatalf("create failed: code=%d body=%s", code, body)
	}
	if got := reloadAssoc(t, uint(gjson.Get(body, "data.ID").Uint())); got.Protocols != nil {
		t.Fatalf("Protocols = %v, want nil（空切片归一为继承）", *got.Protocols)
	}
}

func TestProtocolOverride_RejectsInvalidConfig(t *testing.T) {
	testsupport.InitTestDB(t)
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)

	cases := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{
			name:    "primary 不在集合内",
			body:    `{"model_id":1,"provider_id":1,"provider_model":"pm-bad","weight":10,"priority":10,"protocols":["openai"],"primary_protocol":"anthropic"}`,
			wantMsg: "not in protocols",
		},
		{
			name:    "未知协议",
			body:    `{"model_id":1,"provider_id":1,"provider_model":"pm-bad","weight":10,"priority":10,"protocols":["mystery"]}`,
			wantMsg: "unknown protocol",
		},
		{
			name:    "集合继承但主格式在供应商未启用",
			body:    `{"model_id":1,"provider_id":1,"provider_model":"pm-bad","weight":10,"priority":10,"primary_protocol":"anthropic"}`,
			wantMsg: "is not enabled on this provider",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertGateRejected(t, postAssocRaw(t, true, 0, tc.body), tc.wantMsg)
		})
	}

	if n := gjson.Get(listAssocsViaHandler(t, 1), "data.#").Int(); n != 0 {
		t.Fatalf("rejected creates persisted: list len = %d, want 0", n)
	}
}

func TestProtocolOverride_UpdateClearsToInherit(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t)

	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm",`+
		`"with_header":false,"weight":10,"priority":10,"protocols":["anthropic"],"primary_protocol":"anthropic"}`)
	if got := reloadAssoc(t, mp.ID); got.Protocols == nil || got.PrimaryProtocol == nil {
		t.Fatalf("override not persisted: Protocols=%v PrimaryProtocol=%v", got.Protocols, got.PrimaryProtocol)
	}

	// 缺省两项 = 显式清回继承（三态字段的 Update 契约，与 supports_thinking 同族）。
	updateAssocViaHandler(t, mp.ID, `{"model_id":1,"provider_id":1,"provider_model":"pm",`+
		`"with_header":false,"weight":10,"priority":10}`)
	got := reloadAssoc(t, mp.ID)
	if got.Protocols != nil || got.PrimaryProtocol != nil {
		t.Fatalf("override not cleared: Protocols=%v PrimaryProtocol=%v", got.Protocols, got.PrimaryProtocol)
	}
}
