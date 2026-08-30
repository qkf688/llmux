package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// timeoutSettingsReader 测试用 settings.Reader stub：只覆写请求参数四键，
// 其余回退 defaultValue。超时最小粒度秒，测试用 1s 等头窗口 + 3s 总预算。
type timeoutSettingsReader struct{}

func (timeoutSettingsReader) Bool(_ context.Context, _ string, def bool) bool { return def }

func (timeoutSettingsReader) Int(_ context.Context, key string, def, _ int) int {
	switch key {
	case models.SettingKeyRequestHeaderTimeout:
		return 1 // 等头窗口 1s
	case models.SettingKeyRequestTotalTimeout:
		return 3 // 总预算 3s（> 1s 等头 + 第二候选窗口）
	case models.SettingKeyRequestMaxRetry:
		return 2 // 两次尝试（候选 1 → 候选 2）
	}
	return def
}

func (timeoutSettingsReader) String(_ context.Context, _ string, def string) string { return def }

// TestBalanceChat_HeaderTimeout_CandidateHasFullWindow 锁定「等头窗口与总预算解耦」
// 的行为（AC-8）：候选 1 等头超时后，候选 2 仍有完整窗口可继续尝试。
//
// 构造：等头=1s、总预算=3s、MaxRetry=2；候选 1 上游 1.5s 不回响应头（等头超时），
// 候选 2 立即成功。若总预算与等头同一值（旧双角色语义），候选 1 耗满 1s 后
// Deadline 同步耗尽，候选 2 必然 aborted、断言变红——本用例能区分新旧语义。
func TestBalanceChat_HeaderTimeout_CandidateHasFullWindow(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	SetSettingsReader(timeoutSettingsReader{})
	t.Cleanup(func() { SetSettingsReader(nil) })

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1500 * time.Millisecond) // 不回响应头：超过 1s 等头窗口
	}))
	defer slow.Close()

	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":0,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer fast.Close()

	// 真实路径常规构造：model + provider + 关联建行后经 buildCandidatePool 装配，
	// 避免内存构造 mwp 时 adjustment 的失败递增回查数据库产生 record not found 噪音。
	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	providerSlow := models.Provider{Name: "slow-provider", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, slow.URL)}
	providerFast := models.Provider{Name: "fast-provider", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, fast.URL)}
	for _, p := range []*models.Provider{&providerSlow, &providerFast} {
		if err := models.DB.Create(p).Error; err != nil {
			t.Fatalf("create provider: %v", err)
		}
		// S3-2 起凭据走加密 credentials 表：补老 Provider 等价形态（端点/分组/内联凭据）。
		addLegacyChannelRows(t, cipher, *p)
	}
	mwpSlow := models.ModelWithProvider{ModelID: model.ID, ProviderID: providerSlow.ID, ProviderModel: "slow-upstream", Weight: 10, Priority: 10}
	mwpFast := models.ModelWithProvider{ModelID: model.ID, ProviderID: providerFast.ID, ProviderModel: "fast-upstream", Weight: 10, Priority: 5}
	for _, m := range []*models.ModelWithProvider{&mwpSlow, &mwpFast} {
		if err := models.DB.Create(m).Error; err != nil {
			t.Fatalf("create association: %v", err)
		}
	}

	pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{mwpSlow, mwpFast})
	if err != nil {
		t.Fatalf("build candidate pool: %v", err)
	}

	// 真实路径 CandidatePool 经 buildCandidatePool 装配。
	result, err := BalanceChat(context.Background(), BalanceInput{
		Start:  time.Now(),
		Style:  "openai",
		Before: Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: "m1"},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err != nil {
		t.Fatalf("BalanceChat err = %v, want success（候选 1 等头超时后候选 2 应仍有完整窗口）", err)
	}
	if result.ProviderName != "fast-provider" {
		t.Fatalf("ProviderName = %q, want %q（应故障转移到候选 2）", result.ProviderName, "fast-provider")
	}
}
