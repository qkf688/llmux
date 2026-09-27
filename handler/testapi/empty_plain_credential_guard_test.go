package testapi

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/tidwall/gjson"
)

// Stage 27 空凭据守卫回归（2026-09-27 复现实验翻转）：「空字符串的合法密文」形态的
// 存量凭据（Encrypt("") 解回来 = 空串，active 状态）此前能滑过诊断装配、五个 completion
// 类端点全部带着空凭据头打上游挨 401，把配置问题伪装成上游鉴权失败（复现实测 hits=1、
// error=上游 401 原文）。修复落在选路层单点（channel.DecryptCredentialKey 空明文守卫，
// 覆盖诊断 + 生产聊天 + 健康检查全域），本文件锁诊断侧可观察行为：装配失败走既有
// 「无可用凭据」出口（error_type=auth），一次上游请求都不发。
//
// 「Key 列存空串（非密文）」形态不进本回归：解密对空串直接报 ciphertext too short，
// 属装配期全局失败（buildLink 不换组直接上抛、hits=0），修复前后行为不变。
func TestEmptyPlainCredentialCompletionEndpointsDoNotHitUpstream(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	srv := httptest.NewServer(completionHandlerByAuth(map[string]string{}, &hits, nil, nil))
	t.Cleanup(srv.Close)

	// 空明文的合法密文（形态 B，复现实验证实能打上游的唯一形态）。
	encEmpty, err := cipher.Encrypt("")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}

	provider := seedProviderRow(t, "p-empty-plain", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	group := models.KeyGroup{ProviderID: provider.ID, Name: "空明文组", Weight: 1}
	if err := models.DB.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	groupID := group.ID
	cred := models.Credential{
		GroupID: &groupID,
		Key:     encEmpty,
		KeyHash: cipher.Hash(""),
		Status:  models.CredentialStatusActive,
	}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	mwp := seedAssociation(t, provider.ID, 1, "gpt-4.1")

	expectLocalFailure := func(t *testing.T, w *httptest.ResponseRecorder) {
		t.Helper()
		if hits.Load() != 0 {
			t.Fatalf("upstream hits = %d, want 0（空明文凭据不得打上游）", hits.Load())
		}
		if passed, _ := loadEnvelope(t, w).Data["passed"].(bool); passed {
			t.Fatal("data.passed = true, want false")
		}
		if got := gjson.Get(w.Body.String(), "data.error_type").String(); got != "auth" {
			t.Fatalf("error_type = %q, want auth（凭据问题归 auth）", got)
		}
		if got := gjson.Get(w.Body.String(), "data.error").String(); !strings.Contains(got, "未配置 API Key") {
			t.Fatalf("error = %q, want 含「未配置 API Key」（本地可执行结论，非上游 401 原文）", got)
		}
	}

	resetAndRun := func(run func()) {
		hits.Store(0)
		run()
	}

	t.Run("关联级_completion", func(t *testing.T) {
		resetAndRun(func() { callProviderTest(t, mwp.ID) })
		// /api/test/:id 失败路径自带 buildLink 的 skipped 文案（逐组原因）。
		if hits.Load() != 0 {
			t.Fatalf("upstream hits = %d, want 0", hits.Load())
		}
	})

	t.Run("供应商级_model_test", func(t *testing.T) {
		var w *httptest.ResponseRecorder
		resetAndRun(func() { w = callProviderModelTest(t, provider.ID, `{"model":"gpt-4.1"}`) })
		expectLocalFailure(t, w)
	})

	t.Run("thinking", func(t *testing.T) {
		var w *httptest.ResponseRecorder
		resetAndRun(func() {
			c, rec := testsupport.NewTestContext("GET", "/api/test/thinking/"+strconv.FormatUint(uint64(mwp.ID), 10))
			c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(mwp.ID), 10)}}
			TestThinkingHandler(c)
			w = rec
		})
		expectLocalFailure(t, w)
	})

	t.Run("vision", func(t *testing.T) {
		var w *httptest.ResponseRecorder
		resetAndRun(func() {
			c, rec := testsupport.NewTestContext("GET", "/api/test/vision/"+strconv.FormatUint(uint64(mwp.ID), 10))
			c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(mwp.ID), 10)}}
			TestVisionHandler(c)
			w = rec
		})
		expectLocalFailure(t, w)
	})

	t.Run("structured_output", func(t *testing.T) {
		var w *httptest.ResponseRecorder
		resetAndRun(func() {
			c, rec := testsupport.NewTestContext("GET", "/api/test/structured-output/"+strconv.FormatUint(uint64(mwp.ID), 10))
			c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(mwp.ID), 10)}}
			TestStructuredOutputHandler(c)
			w = rec
		})
		expectLocalFailure(t, w)
	})
}
