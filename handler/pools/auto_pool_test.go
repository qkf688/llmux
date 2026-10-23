package pools

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/repository"
	"github.com/tidwall/gjson"
)

// autoImportViaHandler 走 CreateAutoPoolWithCredentials handler 并返回响应体与状态码。
func autoImportViaHandler(t *testing.T, body string) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/pools/auto/import", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	CreateAutoPoolWithCredentials(c)
	return w.Body.String(), w.Code
}

// autoImportBody 构造自动池导入请求体（key 内含控制字符时正确转义）。
func autoImportBody(name string, keys ...string) string {
	b, err := json.Marshal(map[string]any{"name": name, "keys": keys})
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestAutoPoolImport_CreatesPoolAndCredentials 覆盖主路径：一次请求建出专属号池
// （auto_created=true）并把 key 加密落库，响应给出池标识与逐行回显。
func TestAutoPoolImport_CreatesPoolAndCredentials(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	body, code := autoImportViaHandler(t, autoImportBody("供应商A·默认组", "sk-auto-1-ABCD1234", "sk-auto-2-ABCD1234"))
	if code != 200 {
		t.Fatalf("HTTP %d body=%s", code, body)
	}
	if gjson.Get(body, "code").Int() != 200 {
		t.Fatalf("code!=200 body=%s", body)
	}
	poolID := gjson.Get(body, "data.pool_id").Uint()
	if poolID == 0 {
		t.Fatalf("缺 pool_id body=%s", body)
	}
	if got := gjson.Get(body, "data.pool_name").String(); got != "供应商A·默认组" {
		t.Fatalf("pool_name=%q body=%s", got, body)
	}
	if gjson.Get(body, "data.total").Int() != 2 || gjson.Get(body, "data.imported").Int() != 2 {
		t.Fatalf("导入计数不符 body=%s", body)
	}

	// 库内：池带自动标记，凭据密文落库且可解密回明文
	pool, err := repository.Default().Pool.Get(t.Context(), uint(poolID))
	if err != nil {
		t.Fatalf("get pool: %v", err)
	}
	if !pool.AutoCreated {
		t.Fatal("新建的池应带 auto_created 标记")
	}
	if pool.TestModel != "" {
		t.Fatalf("自动池测试模型应留空（去号池页配），got=%q", pool.TestModel)
	}
	creds, err := repository.Default().Credential.List(t.Context(), repository.CredentialFilter{PoolID: &pool.ID})
	if err != nil {
		t.Fatalf("list creds: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("库内凭据=%d want 2", len(creds))
	}
	cipher := credentialcrypto.Default()
	for _, c := range creds {
		plain, err := cipher.Decrypt(c.Key)
		if err != nil {
			t.Fatalf("解密失败: %v", err)
		}
		if !strings.HasPrefix(plain, "sk-auto-") {
			t.Fatalf("解密回明文异常: %q", plain)
		}
	}
}

// TestAutoPoolImport_DuplicateNameGetsSuffix 覆盖重名兜底：同名再建不报错，
// 自动加 " (2)" 后缀（复用已存在的池 = 往别人的池里塞凭据，绝不能做）。
func TestAutoPoolImport_DuplicateNameGetsSuffix(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	first, _ := autoImportViaHandler(t, autoImportBody("重名池", "sk-dup-1-ABCD1234"))
	if gjson.Get(first, "data.pool_name").String() != "重名池" {
		t.Fatalf("首个池名应为原名 body=%s", first)
	}
	second, _ := autoImportViaHandler(t, autoImportBody("重名池", "sk-dup-2-ABCD1234"))
	if got := gjson.Get(second, "data.pool_name").String(); got != "重名池 (2)" {
		t.Fatalf("重名池名=%q want %q body=%s", got, "重名池 (2)", second)
	}
	if gjson.Get(second, "data.pool_id").Uint() == gjson.Get(first, "data.pool_id").Uint() {
		t.Fatal("重名时应建新池，不复用旧池")
	}
}

// TestAutoPoolImport_Validation 覆盖入参校验：空名/空 keys/超上限 400 且零落库，
// cipher 未配置 500。
func TestAutoPoolImport_Validation(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	cases := map[string]string{
		"空名":      `{"name":"","keys":["sk-v-1-ABCD1234"]}`,
		"缺名":      `{"keys":["sk-v-2-ABCD1234"]}`,
		"空keys":   `{"name":"v池","keys":[]}`,
		"缺keys":   `{"name":"v池"}`,
		"keys非数组": `{"name":"v池","keys":"sk-v-3-ABCD1234"}`,
	}
	for name, payload := range cases {
		body, _ := autoImportViaHandler(t, payload)
		if gjson.Get(body, "code").Int() != 400 {
			t.Fatalf("%s 应400 body=%s", name, body)
		}
	}

	// 超上限：400 且零落库
	tooMany := make([]string, maxCredentialImportBatch+1)
	for i := range tooMany {
		tooMany[i] = "sk-oversize-" + strconv.Itoa(i) + "-ABCD1234"
	}
	body, _ := autoImportViaHandler(t, autoImportBody("超限池", tooMany...))
	if gjson.Get(body, "code").Int() != 400 {
		t.Fatalf("超限应400 body=%s", body)
	}
	pools, _ := repository.Default().Pool.List(t.Context(), repository.PoolFilter{})
	for _, p := range pools {
		if p.Name == "超限池" {
			t.Fatal("超限请求不应建池")
		}
	}

	// cipher 未配置 500
	prev := credentialcrypto.Default()
	credentialcrypto.SetDefault(nil)
	t.Cleanup(func() { credentialcrypto.SetDefault(prev) })
	body2, _ := autoImportViaHandler(t, autoImportBody("无cipher池", "sk-nocipher-ABCD1234"))
	if gjson.Get(body2, "code").Int() != 500 {
		t.Fatalf("无 cipher 应500 body=%s", body2)
	}
}

// TestAutoPoolImport_RouteNotShadowingBatchImport 锁路由顺序契约：
// 新增的静态段 /pools/auto/import 与通配段 /pools/:id/credentials/batch/import
// 同层共存，必须各走各的 handler（静态优先匹配，二者互不捕获）。
func TestAutoPoolImport_RouteNotShadowingBatchImport(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)

	// 静态段走自动池导入（池不存在于请求体语义，靠 name+keys 建池）
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/pools/auto/import", strings.NewReader(autoImportBody("路由池", "sk-route-1-ABCD1234")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 200 || gjson.Get(w.Body.String(), "data.pool_id").Uint() == 0 {
		t.Fatalf("静态段未路由到自动池导入: code=%d body=%s", w.Code, w.Body.String())
	}

	// 通配段仍走批量导入（池 9999 不存在 → 池存在性 404）
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/pools/9999/credentials/batch/import", strings.NewReader(importKeysBody("sk-route-2-ABCD1234")))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if !strings.Contains(w2.Body.String(), "Pool not found") {
		t.Fatalf("通配段未路由到批量导入: body=%s", w2.Body.String())
	}
}

// TestAutoPoolImport_BatchFailureRemovesEmptyPool 覆盖「整批失败不留空池」：
// 查重查询异常（importCredentialRows 的唯一 error 出口，此时一行都没写）→ 500，
// 且刚建的池被删掉——留着它等于给用户塞一个没凭据的专属池，重试还会再撞同名。
func TestAutoPoolImport_BatchFailureRemovesEmptyPool(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	prev := repository.Default().Credential
	repository.Default().Credential = failingHashesRepo{prev}
	t.Cleanup(func() { repository.Default().Credential = prev })

	body, code := autoImportViaHandler(t, autoImportBody("整批失败池", "sk-batchfail-ABCD1234"))
	if code != 500 {
		t.Fatalf("整批失败应500 body=%s", body)
	}
	pools, _ := repository.Default().Pool.List(t.Context(), repository.PoolFilter{})
	for _, p := range pools {
		if p.Name == "整批失败池" {
			t.Fatalf("整批失败后不应留下空池（pool %d）", p.ID)
		}
	}
}

// TestAutoPoolImport_ManualPoolNotMarkedAuto 锁「手动建池不带自动标记」：
// 号池页据此把自动池与手建池分区，用户在号池页新建的池仍是主列表里的普通池。
func TestAutoPoolImport_ManualPoolNotMarkedAuto(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	body := createPoolViaHandler(t, `{"name":"手建池"}`)
	id := gjson.Get(body, "data.ID").Uint()
	if id == 0 {
		t.Fatalf("建池缺 ID body=%s", body)
	}
	pool, err := repository.Default().Pool.Get(t.Context(), uint(id))
	if err != nil {
		t.Fatalf("get pool: %v", err)
	}
	if pool.AutoCreated {
		t.Fatal("手建池不应带 auto_created 标记")
	}
}
