package associations

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// 4a 写入闸契约：关联级 customer_headers 与 apikeys 走同一净化器
// （common.SanitizeOutboundHeaders，两域单源不分叉）。口径：非法名 / 控制字符值 /
// 规范化后撞名 → 信封 code 400 并指明原因；合法值落库为规范化 Title-Case 名。
// 闸为什么存在见 common/outboundheaders.go（map 迭代序随机 → 撞名出站非确定；
// 非法值让 Client.Do 失败并误伤凭据冷却/权重）。BadRequest 是 HTTP 200 + 信封
// code=400（项目统一信封约定），断言打在信封上。

// postAssocRaw 直连 Create/Update handler 返回响应体，不做 200 断言
// ——与 createAssocViaHandler 区分：本文件专测被拒与回显路径。
func postAssocRaw(t *testing.T, isCreate bool, id uint, body string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if isCreate {
		c.Request = httptest.NewRequest("POST", "/model-providers", strings.NewReader(body))
		CreateModelProvider(c)
	} else {
		idStr := strconv.FormatUint(uint64(id), 10)
		c.Params = gin.Params{{Key: "id", Value: idStr}}
		c.Request = httptest.NewRequest("PUT", "/model-providers/"+idStr, strings.NewReader(body))
		UpdateModelProvider(c)
	}
	return w.Body.String()
}

// assocBodyWithHeaders 把 customer_headers 序列化进最小合法请求体。
// 用 json.Marshal 而非手写字符串：CRLF 等控制字符在 Go 源码里是真字节，
// 交给编码器产生 JSON 转义，避免双重转义把测试自身的输入写错。
func assocBodyWithHeaders(t *testing.T, headers map[string]string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"model_id":         1,
		"provider_id":      1,
		"provider_model":   "pm",
		"customer_headers": headers,
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return string(b)
}

func assertGateRejected(t *testing.T, body, wantMsgFragment string) {
	t.Helper()
	if code := gjson.Get(body, "code"); code.Int() != 400 {
		t.Fatalf("envelope code = %d, want 400, body=%s", code.Int(), body)
	}
	if msg := gjson.Get(body, "message").String(); !strings.Contains(msg, wantMsgFragment) {
		t.Fatalf("message = %q, want contains %q", msg, wantMsgFragment)
	}
}

func TestCreateModelProvider_CustomerHeadersGate_Rejects(t *testing.T) {
	testsupport.InitTestDB(t)
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)

	cases := []struct {
		name    string
		headers map[string]string
		wantMsg string
	}{
		{"撞名对拒绝（迭代序随机下唯一确定的语义是拒）", map[string]string{"x-app": "1", "X-App": "2"}, "duplicate header names"},
		{"非法头名拒绝", map[string]string{"Bad Name": "v"}, "not allowed in RFC 7230"},
		{"CRLF 值拒绝（头注入）", map[string]string{"X-A": "a\r\nX-B: injected"}, "control characters"},
		{"NUL 值拒绝", map[string]string{"X-A": "a\x00b"}, "control characters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postAssocRaw(t, true, 0, assocBodyWithHeaders(t, tc.headers))
			assertGateRejected(t, resp, tc.wantMsg)
		})
	}

	// 被拒不落库：四条全部拒绝后列表仍为空。
	listBody := listAssocsViaHandler(t, 1)
	if n := gjson.Get(listBody, "data.#").Int(); n != 0 {
		t.Fatalf("rejected creates persisted: list len = %d, want 0, body=%s", n, listBody)
	}
}

// TestCreateModelProvider_CustomerHeadersGate_Normalizes 钉存储/回显形状：
// 合法小写名落库为 Title-Case 规范化键（与 Go 出站 Set 同口径，三处一套样子）。
func TestCreateModelProvider_CustomerHeadersGate_Normalizes(t *testing.T) {
	testsupport.InitTestDB(t)
	createModelForThinkingTest(t)
	createProviderForThinkingTest(t)

	resp := postAssocRaw(t, true, 0, assocBodyWithHeaders(t, map[string]string{"x-app": "cli"}))
	if code := gjson.Get(resp, "code").Int(); code != 200 {
		t.Fatalf("envelope code = %d, want 200, body=%s", code, resp)
	}
	if got := gjson.Get(resp, "data.CustomerHeaders.X-App").String(); got != "cli" {
		t.Fatalf("response CustomerHeaders not normalized: %s", gjson.Get(resp, "data.CustomerHeaders").Raw)
	}
	id := uint(gjson.Get(resp, "data.ID").Uint())
	reloaded, err := repository.Default().ModelWithProvider.Get(t.Context(), id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.CustomerHeaders["X-App"] != "cli" || len(reloaded.CustomerHeaders) != 1 {
		t.Fatalf("stored headers = %v, want {X-App:cli}", reloaded.CustomerHeaders)
	}
}

func TestUpdateModelProvider_CustomerHeadersGate(t *testing.T) {
	testsupport.InitTestDB(t)
	mp := createAssocForThinkingTest(t) // CustomerHeaders: {"X-Test":"1"}

	// 1) 非法值 → 400 且旧值原样保留（拒绝不产生半写）
	resp := postAssocRaw(t, false, mp.ID, assocBodyWithHeaders(t, map[string]string{"X-A": "a\x00b"}))
	assertGateRejected(t, resp, "control characters")
	if got := reloadAssoc(t, mp.ID); len(got.CustomerHeaders) != 1 || got.CustomerHeaders["X-Test"] != "1" {
		t.Fatalf("rejected update mutated stored headers: %v", got.CustomerHeaders)
	}

	// 2) 合法提交 → 整体替换并规范化
	resp = postAssocRaw(t, false, mp.ID, assocBodyWithHeaders(t, map[string]string{"x-new": "2"}))
	if code := gjson.Get(resp, "code").Int(); code != 200 {
		t.Fatalf("envelope code = %d, want 200, body=%s", code, resp)
	}
	got := reloadAssoc(t, mp.ID)
	if len(got.CustomerHeaders) != 1 || got.CustomerHeaders["X-New"] != "2" {
		t.Fatalf("stored headers = %v, want {X-New:2}", got.CustomerHeaders)
	}
}
