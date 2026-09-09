package pools

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// callTestCredential 直接驱动 handler（与包内其它凭据 handler 测试同姿势）：
// 只填 :id / :credId 两个 path 参数，POST 空体。返回响应体与 HTTP 状态码。
func callTestCredential(t *testing.T, poolID, credID uint) (string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "id", Value: strconv.FormatUint(uint64(poolID), 10)},
		{Key: "credId", Value: strconv.FormatUint(uint64(credID), 10)},
	}
	base := "/pools/" + strconv.FormatUint(uint64(poolID), 10) + "/credentials/" + strconv.FormatUint(uint64(credID), 10)
	c.Request = httptest.NewRequest(http.MethodPost, base+"/test", strings.NewReader("{}"))
	c.Request.Header.Set("Content-Type", "application/json")
	TestCredential(c)
	return w.Body.String(), w.Code
}

// credTestUpstream 假上游：仅放行登记过的完整凭据头（含 Bearer 前缀），其余 401。
// 一把同时钉住「用了哪把 key」「打到哪个 URL/方法」「打了几次」，明文只可能从上游侧证明。
type credTestUpstream struct {
	hits       atomic.Int64
	lastAuth   atomic.Value
	lastMethod atomic.Value
	lastPath   atomic.Value
	lastBody   atomic.Value
}

func startCredTestUpstream(t *testing.T, allowed map[string]struct{}, sink *credTestUpstream) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sink.hits.Add(1)
		sink.lastMethod.Store(r.Method)
		sink.lastPath.Store(r.URL.Path)
		reqBody, _ := io.ReadAll(r.Body)
		sink.lastBody.Store(string(reqBody))
		auth := r.Header.Get("Authorization")
		if auth == "" {
			auth = r.Header.Get("x-api-key")
		}
		sink.lastAuth.Store(auth)
		if _, ok := allowed[auth]; !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
			return
		}
		// completion 探针：成功回一条 chat.completion 形状（handler 只判 2xx，内容不影响结论，
		// 但形状与 #models 的 list 区分开——配合 lastPath 断言证明测的是补全而非轻探）。
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-probe","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"length"}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// setPoolTestModel 给号池配置池级测试模型（#24-B 迁移：测试模型归属号池，非分组）。
func setPoolTestModel(t *testing.T, poolID uint, model string) {
	t.Helper()
	if _, err := repos().Pool.UpdateFields(t.Context(), poolID, map[string]any{"test_model": model}); err != nil {
		t.Fatalf("set pool test_model: %v", err)
	}
}

// seedCredTestUpstreamChain 建「供应商 + 主协议端点（URL 指向假上游）+ 引用该池的分组」，
// 并把**号池**的测试模型配为 gpt-probe（#24-B 迁移：模型来自池，链路仍由分组定位）。
// Provider.Config 刻意指向不可达主机：只有端点 URL + 凭据明文真正生效才算走对链路，
// 任何绕过 channel 装配的回归（直用 Config）都会立刻打不通而非静默通过。
func seedCredTestUpstreamChain(t *testing.T, poolID uint, providerName, providerType, upstreamURL string) models.KeyGroup {
	t.Helper()
	provider := models.Provider{
		Name:   providerName,
		Type:   providerType,
		Config: `{"base_url":"https://should-not-use.example"}`,
	}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	endpoint := models.Endpoint{
		ProviderID: provider.ID,
		Protocol:   string(consts.ProtocolOfType(providerType)),
		URL:        upstreamURL,
		Enabled:    true,
	}
	if err := models.DB.Create(&endpoint).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	group := models.KeyGroup{ProviderID: provider.ID, Name: providerName + "-组", Weight: 1, PoolID: &poolID}
	if err := models.DB.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	setPoolTestModel(t, poolID, "gpt-probe")
	return group
}

// TestTestCredential_HitsUpstreamWithTargetKey（happy path）
// 用被点测的那把凭据明文 + 端点 URL 打上游（非 Provider.Config、非组内首个可用），
// 成功回 passed:true，且响应回显命中的 group/provider，且只回掩码不泄漏明文。
func TestTestCredential_HitsUpstreamWithTargetKey(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	const plain = "sk-target-ABCD1234"
	sink := &credTestUpstream{}
	srv := startCredTestUpstream(t, map[string]struct{}{"Bearer " + plain: {}}, sink)

	poolID := createPoolAndGetID(t, "链路池")
	credBody, _ := createCredentialViaHandler(t, poolID, `{"key":"`+plain+`"}`)
	credID := uint(gjson.Get(credBody, "data.ID").Uint())
	group := seedCredTestUpstreamChain(t, poolID, "链路供应商", "openai", srv.URL)

	body, httpCode := callTestCredential(t, poolID, credID)
	if httpCode != http.StatusOK {
		t.Fatalf("HTTP = %d, want 200; body=%s", httpCode, body)
	}
	if code := gjson.Get(body, "code").Int(); code != 200 {
		t.Fatalf("envelope code = %d, want 200; body=%s", code, body)
	}
	if !gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("data.passed = false; error=%v", gjson.Get(body, "data.error").String())
	}
	if got := sink.lastAuth.Load().(string); got != "Bearer "+plain {
		t.Fatalf("上游凭据头 = %q, want Bearer %s（必须是被点测的那把 key）", got, plain)
	}
	if sink.hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（打端点 URL，一次即止）", sink.hits.Load())
	}
	if gjson.Get(body, "data.group_id").Uint() != uint64(group.ID) {
		t.Fatalf("回显 group_id = %d, want %d; body=%s", gjson.Get(body, "data.group_id").Uint(), group.ID, body)
	}
	// 测的是真实 completion，不是 /models 轻探：POST …/chat/completions，body 带分组
	// test_model（gpt-probe）与探针内容。回显 model 也应为该模型。
	if m := sink.lastMethod.Load().(string); m != http.MethodPost {
		t.Fatalf("上游方法 = %q, want POST（completion 探针须是 POST，非 /models GET）", m)
	}
	if p := sink.lastPath.Load().(string); !strings.Contains(p, "chat/completions") {
		t.Fatalf("上游路径 = %q, want 含 chat/completions; body=%s", p, body)
	}
	if b := sink.lastBody.Load().(string); !strings.Contains(b, `"model":"gpt-probe"`) || !strings.Contains(b, "hi") {
		t.Fatalf("上游请求体未带分组 test_model/探针内容: %q", b)
	}
	if got := gjson.Get(body, "data.model").String(); got != "gpt-probe" {
		t.Fatalf("回显 model = %q, want gpt-probe; body=%s", got, body)
	}
	if strings.Contains(body, plain) || strings.Contains(body, "should-not-use") {
		t.Fatalf("响应泄漏明文/占位主机; body=%s", body)
	}
}

// TestTestCredential_DoesNotFallBackToFirstUsableCredential 锁「按凭据 ≠ 按组首个」：
// 组内 ID 更小的凭据故意不让上游放行，只有目标凭据放行。若 handler 误用
// ProbeGroup/FirstUsableCredential 必然 401 失败。
func TestTestCredential_DoesNotFallBackToFirstUsableCredential(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	sink := &credTestUpstream{}
	srv := startCredTestUpstream(t, map[string]struct{}{"Bearer sk-target-XY99OK": {}}, sink)

	poolID := createPoolAndGetID(t, "两把钥匙池")
	firstBody, _ := createCredentialViaHandler(t, poolID, `{"key":"sk-first-DEADBEEF"}`)
	_ = firstBody // ID 更小、故意是坏 key
	targetBody, _ := createCredentialViaHandler(t, poolID, `{"key":"sk-target-XY99OK"}`)
	targetID := uint(gjson.Get(targetBody, "data.ID").Uint())

	seedCredTestUpstreamChain(t, poolID, "两把钥匙供应商", "openai", srv.URL)

	body, _ := callTestCredential(t, poolID, targetID)
	if !gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("data.passed = false，说明打的是组内首个而非指定凭据; body=%s", body)
	}
	if got := sink.lastAuth.Load().(string); got != "Bearer sk-target-XY99OK" {
		t.Fatalf("上游凭据头 = %q, want Bearer sk-target-XY99OK", got)
	}
}

// TestTestCredential_UpstreamAuthFailureReportsNotWriteBack：
// 目标 key 不被放行 → passed:false + error_type=auth，且**绝不回写凭据状态/FailCount**
// （只报不改），且只打一次上游（不重试）。
func TestTestCredential_UpstreamAuthFailureReportsNotWriteBack(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	const plain = "sk-broken-KEYZ9999"
	sink := &credTestUpstream{}
	srv := startCredTestUpstream(t, nil, sink) // 全不放行 → 必 401

	poolID := createPoolAndGetID(t, "坏钥匙池")
	credBody, _ := createCredentialViaHandler(t, poolID, `{"key":"`+plain+`"}`)
	credID := uint(gjson.Get(credBody, "data.ID").Uint())
	seedCredTestUpstreamChain(t, poolID, "坏钥匙供应商", "openai", srv.URL)

	body, httpCode := callTestCredential(t, poolID, credID)
	if httpCode != http.StatusOK {
		t.Fatalf("HTTP = %d, want 200（失败的测试仍是成功响应）; body=%s", httpCode, body)
	}
	if gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("data.passed = true, want false; body=%s", body)
	}
	if got := gjson.Get(body, "data.error_type").String(); got != "auth" {
		t.Fatalf("error_type = %q, want auth; body=%s", got, body)
	}
	if sink.hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1（失败不重试）", sink.hits.Load())
	}

	saved, err := repos().Credential.Get(t.Context(), credID)
	if err != nil {
		t.Fatalf("get saved credential: %v", err)
	}
	if saved.Status != models.CredentialStatusActive || saved.FailCount != 0 {
		t.Fatalf("失败测试回写了凭据状态：status=%q fail_count=%d（要求只报不改）", saved.Status, saved.FailCount)
	}
}

// TestTestCredential_CrossPoolIsNotFound：越池 404（信封 code=404，非 2xx），且零上游请求。
func TestTestCredential_CrossPoolIsNotFound(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	sink := &credTestUpstream{}
	_ = startCredTestUpstream(t, nil, sink)

	poolA := createPoolAndGetID(t, "越池A")
	body, _ := createCredentialViaHandler(t, poolA, `{"key":"sk-cross-ABCD1234"}`)
	credID := uint(gjson.Get(body, "data.ID").Uint())
	poolB := createPoolAndGetID(t, "越池B")

	resp, httpCode := callTestCredential(t, poolB, credID)
	if httpCode != http.StatusOK {
		t.Fatalf("HTTP = %d, want 200（404 走信封）; body=%s", httpCode, resp)
	}
	if code := gjson.Get(resp, "code").Int(); code != 404 {
		t.Fatalf("越池 code = %d, want 404; body=%s", code, resp)
	}
	if sink.hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0（越池不得打上游）", sink.hits.Load())
	}
}

// TestTestCredential_PoolWithoutReferencingGroup：池未被任何分组引用 → passed:false +
// error_type=provider（链路无从定位），零上游请求；与越池 404 区分（这是配置态结论）。
func TestTestCredential_PoolWithoutReferencingGroup(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	sink := &credTestUpstream{}
	_ = startCredTestUpstream(t, nil, sink)

	poolID := createPoolAndGetID(t, "无组池")
	body, _ := createCredentialViaHandler(t, poolID, `{"key":"sk-nogroup-ABCD1234"}`)
	credID := uint(gjson.Get(body, "data.ID").Uint())

	resp, httpCode := callTestCredential(t, poolID, credID)
	if httpCode != http.StatusOK {
		t.Fatalf("HTTP = %d, want 200; body=%s", httpCode, resp)
	}
	if code := gjson.Get(resp, "code").Int(); code != 200 {
		t.Fatalf("code = %d, want 200（无引用组是一次失败的测试结论，非入参错误）; body=%s", code, resp)
	}
	if gjson.Get(resp, "data.passed").Bool() {
		t.Fatal("data.passed = true, want false")
	}
	if got := gjson.Get(resp, "data.error_type").String(); got != "provider" {
		t.Fatalf("error_type = %q, want provider; body=%s", got, resp)
	}
	if sink.hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0（链路未选定不得打上游）", sink.hits.Load())
	}
}

// TestTestCredential_PicksLowestProviderThenGroup：多引用组时取 ProviderID/GroupID
// 最小者的主协议端点，并把该 group 回显进响应。
//
// 故意交叉创建顺序：两个供应商 + 两个分组，让「分组 ID 最小的那条」归属「ProviderID 较大
// 的供应商」。ListByPoolIDs 只按分组 ID 升序返回，若 handler 直接取首行就会选中错误供应商；
// 正确实现须按 (ProviderID, GroupID) 排序 → 命中 ProviderID 最小的供应商（其分组 ID 反而较大）。
// 两个上游 sink 分立，只有被选中的链路才应收到请求。
func TestTestCredential_PicksLowestProviderThenGroup(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	const plain = "sk-multi-AB12CD34"
	// winSrv = ProviderID 最小供应商的上游；loseSrv = ProviderID 较大供应商的上游。
	winSink := &credTestUpstream{}
	winSrv := startCredTestUpstream(t, map[string]struct{}{"Bearer " + plain: {}}, winSink)
	loseSink := &credTestUpstream{}
	loseSrv := startCredTestUpstream(t, map[string]struct{}{"Bearer " + plain: {}}, loseSink)

	poolID := createPoolAndGetID(t, "多组池")
	credBody, _ := createCredentialViaHandler(t, poolID, `{"key":"`+plain+`"}`)
	credID := uint(gjson.Get(credBody, "data.ID").Uint())

	// providerWin 先建 → ProviderID 更小；但其分组后建 → GroupID 更大。
	providerWin := models.Provider{Name: "低ID供应商", Type: "openai", Config: `{"base_url":"https://win.example"}`}
	if err := models.DB.Create(&providerWin).Error; err != nil {
		t.Fatalf("create providerWin: %v", err)
	}
	if err := models.DB.Create(&models.Endpoint{ProviderID: providerWin.ID, Protocol: string(consts.ProtocolOfType("openai")), URL: winSrv.URL, Enabled: true}).Error; err != nil {
		t.Fatalf("create endpointWin: %v", err)
	}

	// providerLose 后建 → ProviderID 更大；但其分组先建 → GroupID 更小（ListByPoolIDs 首行）。
	providerLose := models.Provider{Name: "高ID供应商", Type: "openai", Config: `{"base_url":"https://lose.example"}`}
	if err := models.DB.Create(&providerLose).Error; err != nil {
		t.Fatalf("create providerLose: %v", err)
	}
	if err := models.DB.Create(&models.Endpoint{ProviderID: providerLose.ID, Protocol: string(consts.ProtocolOfType("openai")), URL: loseSrv.URL, Enabled: true}).Error; err != nil {
		t.Fatalf("create endpointLose: %v", err)
	}

	loseGroup := models.KeyGroup{ProviderID: providerLose.ID, Name: "高ID供应商-组", Weight: 1, PoolID: &poolID}
	if err := models.DB.Create(&loseGroup).Error; err != nil {
		t.Fatalf("create loseGroup: %v", err)
	}
	winGroup := models.KeyGroup{ProviderID: providerWin.ID, Name: "低ID供应商-组", Weight: 1, PoolID: &poolID}
	if err := models.DB.Create(&winGroup).Error; err != nil {
		t.Fatalf("create winGroup: %v", err)
	}
	setPoolTestModel(t, poolID, "gpt-probe")
	if winGroup.ID < loseGroup.ID {
		t.Fatalf("测试前置不成立：期望 winGroup.ID(%d) > loseGroup.ID(%d) 才能证伪『取 ListByPoolIDs 首行』", winGroup.ID, loseGroup.ID)
	}

	body, _ := callTestCredential(t, poolID, credID)
	if !gjson.Get(body, "data.passed").Bool() {
		t.Fatalf("data.passed = false; body=%s", body)
	}
	if got := gjson.Get(body, "data.group_id").Uint(); got != uint64(winGroup.ID) {
		t.Fatalf("回显 group_id = %d, want ProviderID 最小供应商的分组 %d（若命中 loseGroup=%d 说明按分组 ID 而非 ProviderID 排序）; body=%s",
			got, winGroup.ID, loseGroup.ID, body)
	}
	if winSink.hits.Load() != 1 {
		t.Fatalf("应测供应商上游 hits = %d, want 1", winSink.hits.Load())
	}
	if loseSink.hits.Load() != 0 {
		t.Fatalf("非首个供应商上游 hits = %d, want 0（不得打到 ProviderID 较大的链路）", loseSink.hits.Load())
	}
}

// TestTestCredential_PoolWithoutTestModel_FailsValidationNoUpstream（Q4 迁移后口径）：
// 号池未配置 test_model → passed:false + error_type=validation，**不打上游、不装配 client**。
// 锁「不回退硬编码默认模型」：宁可明确报「未配置测试模型」，也不能拿一个未必被这把 key
// 授权的默认模型去测、把「配置缺失」误判成「key 坏」。仍回显命中的 group/provider。
func TestTestCredential_PoolWithoutTestModel_FailsValidationNoUpstream(t *testing.T) {
	testsupport.InitTestDB(t)
	setupCipherForCredTest(t)

	sink := &credTestUpstream{}
	srv := startCredTestUpstream(t, nil, sink)

	// 池故意不设测试模型（createPoolAndGetID 只给 name）。
	poolID := createPoolAndGetID(t, "缺测试模型池")
	credBody, _ := createCredentialViaHandler(t, poolID, `{"key":"sk-notm-ABCD1234"}`)
	credID := uint(gjson.Get(credBody, "data.ID").Uint())

	provider := models.Provider{Name: "缺模型供应商", Type: "openai", Config: `{"base_url":"https://should-not-use.example"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if err := models.DB.Create(&models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), URL: srv.URL, Enabled: true}).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	group := models.KeyGroup{ProviderID: provider.ID, Name: "缺模型组", Weight: 1, PoolID: &poolID}
	if err := models.DB.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}

	resp, httpCode := callTestCredential(t, poolID, credID)
	if httpCode != http.StatusOK {
		t.Fatalf("HTTP = %d, want 200; body=%s", httpCode, resp)
	}
	if code := gjson.Get(resp, "code").Int(); code != 200 {
		t.Fatalf("code = %d, want 200（未配置是一次失败的测试结论）; body=%s", code, resp)
	}
	if gjson.Get(resp, "data.passed").Bool() {
		t.Fatalf("data.passed = true, want false; body=%s", resp)
	}
	if got := gjson.Get(resp, "data.error_type").String(); got != "validation" {
		t.Fatalf("error_type = %q, want validation; body=%s", got, resp)
	}
	if gjson.Get(resp, "data.group_id").Uint() != uint64(group.ID) {
		t.Fatalf("回显 group_id = %d, want %d; body=%s", gjson.Get(resp, "data.group_id").Uint(), group.ID, resp)
	}
	if sink.hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0（未配置测试模型不得装配/打上游）", sink.hits.Load())
	}
}
