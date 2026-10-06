package providerapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
)

type providerModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// authCapturingModelsServer 起一个 /models 上游：只接受 wantAuth，其余一律 401
// （复现真实上游对缺 key 的判定），并记录收到的 Authorization 头与命中次数。
func authCapturingModelsServer(t *testing.T, wantAuth, body string) (*httptest.Server, *atomic.Value, *atomic.Int64) {
	t.Helper()
	var gotAuth atomic.Value
	gotAuth.Store("")
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		hits.Add(1)
		gotAuth.Store(r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") != wantAuth {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"You didn't provide an API key."}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &gotAuth, &hits
}

// seedPoolBackedProvider 建「单分组关联号池」的供应商（生产实际形态）：
// Config **不含 api_key**（S6 起凭据只存在于 credentials 表、运行时注入），
// base_url 刻意指向不可达主机——只有走端点 URL + 号池凭据才可能成功。
func seedPoolBackedProvider(t *testing.T, name, endpointURL, plainKey string, modelEndpoint *bool) models.Provider {
	t.Helper()
	cipher, err := credentialcrypto.New(testCredHexKey)
	if err != nil {
		t.Fatalf("credentialcrypto.New: %v", err)
	}

	provider := models.Provider{
		Name:          name,
		Type:          "openai",
		Config:        `{"base_url":"https://should-not-use.example"}`,
		ModelEndpoint: modelEndpoint,
	}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: "openai", URL: endpointURL, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	pool := models.Pool{Name: name + "-池"}
	if err := models.DB.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	group := models.KeyGroup{ProviderID: provider.ID, Name: "池组", Weight: 1, PoolID: &pool.ID}
	if err := models.DB.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	if plainKey != "" {
		enc, err := cipher.Encrypt(plainKey)
		if err != nil {
			t.Fatalf("encrypt key: %v", err)
		}
		cred := models.Credential{
			PoolID:  pool.ID,
			Key:     enc,
			KeyHash: cipher.Hash(plainKey),
			Status:  models.CredentialStatusActive,
		}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential: %v", err)
		}
	}
	return provider
}

func callGetProviderModels(t *testing.T, providerID uint) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, w := testsupport.NewTestContext("GET", "/providers/models/"+strconv.FormatUint(uint64(providerID), 10))
	c.Params = []gin.Param{{Key: "id", Value: strconv.FormatUint(uint64(providerID), 10)}}
	GetProviderModels(c)
	return w
}

// TestGetProviderModels_UsesPoolCredentialNotProviderConfig 是 401「You didn't provide
// an API key」的回归测试：分组关联号池时，管理端浏览上游模型必须用「协议端点 URL +
// 号池凭据明文」打上游。修复前 handler 直用 Provider.Config 组装客户端，而 config
// 自 S6 起不含 api_key，于是请求不带 Authorization → 上游恒 401。
func TestGetProviderModels_UsesPoolCredentialNotProviderConfig(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	server, gotAuth, hits := authCapturingModelsServer(t, "Bearer sk-pool-AAAA1111",
		`{"object":"list","data":[{"id":"m1","object":"model","created":0,"owned_by":"o"},{"id":"m2","object":"model","created":0,"owned_by":"o"}]}`)
	provider := seedPoolBackedProvider(t, "p-pool", server.URL, "sk-pool-AAAA1111", nil)

	w := callGetProviderModels(t, provider.ID)
	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	var payload testsupport.APIEnvelope[[]providerModel]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if len(payload.Data) != 2 || payload.Data[0].ID != "m1" || payload.Data[1].ID != "m2" {
		t.Fatalf("models = %+v, want ids [m1 m2]", payload.Data)
	}
	if auth := gotAuth.Load().(string); auth != "Bearer sk-pool-AAAA1111" {
		t.Fatalf("upstream Authorization = %q, want 号池凭据明文", auth)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1", hits.Load())
	}
}

// TestGetProviderModels_ModelEndpointDisabledStillWorks 保留原有契约：
// ModelEndpoint=false 只影响自动同步筛选，人工浏览上游模型仍可用。
// 该测试原本误置于 handler/modelsync/logs_test.go，因其本质是 providerapi 行为，
// 在 Stage 4 修正测试归属，迁移至此。
func TestGetProviderModels_ModelEndpointDisabledStillWorks(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	server, _, _ := authCapturingModelsServer(t, "Bearer sk-pool-BBBB2222",
		`{"object":"list","data":[{"id":"m1","object":"model","created":0,"owned_by":"o"}]}`)
	disabled := false
	provider := seedPoolBackedProvider(t, "p-disabled-ep", server.URL, "sk-pool-BBBB2222", &disabled)

	w := callGetProviderModels(t, provider.ID)
	var payload testsupport.APIEnvelope[[]providerModel]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if len(payload.Data) != 1 || payload.Data[0].ID != "m1" {
		t.Fatalf("models = %+v, want [m1]", payload.Data)
	}
}

// TestGetProviderModels_NoCredentialReportsReason 无可用凭据时给出可读原因，
// 而不是把「不带 key 打上游」换来的 401 原文丢给用户。
func TestGetProviderModels_NoCredentialReportsReason(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	server, _, hits := authCapturingModelsServer(t, "Bearer unused", `{"object":"list","data":[]}`)
	provider := seedPoolBackedProvider(t, "p-nokey", server.URL, "", nil)

	w := callGetProviderModels(t, provider.ID)
	var payload testsupport.APIEnvelope[[]providerModel]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 404 {
		t.Fatalf("payload code = %d, want 404, body=%s", payload.Code, w.Body.String())
	}
	if !strings.Contains(payload.Message, "no usable credential") {
		t.Fatalf("message = %q, want 含 'no usable credential'", payload.Message)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream hits = %d, want 0（无凭据不得打上游）", hits.Load())
	}
}

// TestGetProviderModels_ProviderNotFound 供应商不存在 → code=404 + "Provider not found"。
func TestGetProviderModels_ProviderNotFound(t *testing.T) {
	testsupport.InitTestDB(t)
	setupProviderCrypto(t)

	w := callGetProviderModels(t, 424242)
	var payload testsupport.APIEnvelope[[]providerModel]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 404 || payload.Message != "Provider not found" {
		t.Fatalf("payload = code %d msg %q, want 404 'Provider not found'", payload.Code, payload.Message)
	}
}
