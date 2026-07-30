package providerapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/atopos31/llmio/handler/testsupport"
	"github.com/atopos31/llmio/models"
	"github.com/gin-gonic/gin"
)

// TestGetProviderModels_ModelEndpointDisabledStillWorks 验证 providerapi.GetProviderModels
// 在 provider 的 ModelEndpoint 关闭时仍能正常拉取上游模型列表。
// 该测试原本误置于 handler/modelsync/logs_test.go，因其本质是 providerapi 行为，
// 在 Stage 4 修正测试归属，迁移至此。
func TestGetProviderModels_ModelEndpointDisabledStillWorks(t *testing.T) {
	testsupport.InitTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"m1","object":"model","created":0,"owned_by":"o"},{"id":"m2","object":"model","created":0,"owned_by":"o"}]}`))
	}))
	t.Cleanup(server.Close)

	disabled := false
	provider := models.Provider{
		Name:          "p1",
		Type:          "openai",
		Config:        `{"api_key":"k","base_url":"` + server.URL + `"}`,
		ModelEndpoint: &disabled,
	}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	c, w := testsupport.NewTestContext("GET", "/providers/models/"+strconv.FormatUint(uint64(provider.ID), 10)+"?source=upstream")
	c.Params = []gin.Param{{Key: "id", Value: strconv.FormatUint(uint64(provider.ID), 10)}}
	GetProviderModels(c)
	if w.Code != 200 {
		t.Fatalf("status code = %d, want 200, body=%s", w.Code, w.Body.String())
	}

	type providerModel struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	var payload testsupport.APIEnvelope[[]providerModel]
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%s", err, w.Body.String())
	}
	if payload.Code != 200 {
		t.Fatalf("payload code = %d, want 200, body=%s", payload.Code, w.Body.String())
	}
	if len(payload.Data) != 2 {
		t.Fatalf("models length = %d, want 2", len(payload.Data))
	}
	if payload.Data[0].ID != "m1" || payload.Data[1].ID != "m2" {
		t.Fatalf("models = %+v, want ids [m1 m2]", payload.Data)
	}
}