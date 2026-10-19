package chat

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// TestBalanceChat_AssociationProtocolOverride_SelectsNarrowedEndpoint 锁生产主路径的接线：
// 关联把允许集合收窄成 anthropic 后，客户端发 OpenAI 格式必须被转换并发往 anthropic 端点。
//
// 为什么必须在 chat 层测：把 chat_retry_loop 的 `Assemble(…, &modelWithProvider)` 改成 nil，
// channel 包的单测仍然全绿（它自己构造 Snapshot）——只有端到端跑一次才能锁住这处接线。
func TestBalanceChat_AssociationProtocolOverride_SelectsNarrowedEndpoint(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	var oaHits, anHits atomic.Int32
	openaiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oaHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fakeOpenAIChatResponse))
	}))
	defer openaiSrv.Close()
	anthropicSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fakeAnthropicMessagesResponse))
	}))
	defer anthropicSrv.Close()

	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "dual", Type: providers.TypeOpenAI, Config: `{"base_url":"http://127.0.0.1:1/never"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	for _, ep := range []models.Endpoint{
		{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), URL: openaiSrv.URL, Enabled: true},
		{ProviderID: provider.ID, Protocol: string(consts.ProtocolAnthropic), URL: anthropicSrv.URL, Enabled: true},
	} {
		ep := ep
		if err := models.DB.Create(&ep).Error; err != nil {
			t.Fatalf("create endpoint: %v", err)
		}
	}
	_, _, mwp := seedGroupedChannel(t, cipher, provider.ID, "g1", model.ID, 10, 1, []string{"sk-any"}, nil)

	narrowed := []string{string(consts.ProtocolAnthropic)}
	mwp.Protocols = &narrowed
	if err := models.DB.Save(&mwp).Error; err != nil {
		t.Fatalf("persist protocol override: %v", err)
	}

	result := runBalanceOnce(t, mwp, string(consts.StyleOpenAI), `{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)
	if result.ProviderName != "dual" {
		t.Fatalf("ProviderName = %q, want dual", result.ProviderName)
	}
	if oaHits.Load() != 0 || anHits.Load() != 1 {
		t.Fatalf("oaHits=%d anHits=%d, want 0/1（关联收窄成 anthropic 后入站 OpenAI 必须走转换）",
			oaHits.Load(), anHits.Load())
	}
}
