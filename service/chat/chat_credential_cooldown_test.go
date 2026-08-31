package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
)

func TestClassifyCredentialFailure(t *testing.T) {
	cases := []struct {
		name string
		code int
		want bool
	}{
		{"429 限流", http.StatusTooManyRequests, true},
		{"500 服务端", http.StatusInternalServerError, true},
		{"503 不可用", http.StatusServiceUnavailable, true},
		{"502 网关", http.StatusBadGateway, true},
		{"401 未授权", http.StatusUnauthorized, true},
		{"403 禁止", http.StatusForbidden, true},
		{"400 请求体被拒", http.StatusBadRequest, false},
		{"404 不存在", http.StatusNotFound, false},
		{"409 冲突", http.StatusConflict, false},
		{"418 茶壶", http.StatusTeapot, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyCredentialFailure(tc.code); got != tc.want {
				t.Fatalf("classifyCredentialFailure(%d) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}

func TestApplyCredentialCooldown(t *testing.T) {
	initChatRecordTestDB(t)

	grp := models.KeyGroup{ProviderID: 1, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	cred := models.Credential{GroupID: &grp.ID, Key: "enc", KeyHash: "hash"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	applyCredentialCooldown(context.Background(), cred, "http_429")

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.CooldownUntil == nil {
		t.Fatalf("CooldownUntil = nil, want 窗口内冷却已写入")
	}
	if time.Until(*got.CooldownUntil) > minimalCredentialCooldown+time.Second {
		t.Fatalf("CooldownUntil = %v, want within %v", time.Until(*got.CooldownUntil), minimalCredentialCooldown)
	}
	if got.CooldownReason != "http_429" {
		t.Fatalf("CooldownReason = %q, want http_429", got.CooldownReason)
	}
}

// 上游 429 → executeSingleProviderAttempt 走 handleNonOKProviderResponse 凭据级分支：
// 选中凭据的 CooldownUntil 落库非空 + reason 为 http_429。锁 AC-1 的「冷却写库」
// 半边（组内换 key 的端到端在 retry loop 集成测试）。
func TestExecuteSingleProviderAttempt_NonOK429_WritesCredentialCooldown(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	providerRow := models.Provider{Name: "p1", Type: providers.TypeOpenAI, Config: `{"base_url":"http://127.0.0.1:1/never"}`}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: providerRow.ID, Protocol: string(consts.ProtocolOpenAI), URL: srv.URL, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp := models.KeyGroup{ProviderID: providerRow.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	plain := "sk-cooldown"
	cred := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain), Note: "cooldown-key"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model-provider: %v", err)
	}

	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-cooldown"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	result := executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:               context.Background(),
		Start:             time.Now(),
		Style:             string(consts.StyleOpenAI),
		Before:            Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName:     "m1",
		ReqMeta:           models.ReqMeta{Header: http.Header{}},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		Selection: channel.SelectionResult{
			Endpoint:   ep,
			Group:      grp,
			Credential: cred,
		},
		ChatModel: chatModel,
		Client:    srv.Client(),
	}, make(chan models.ChatLog))

	if !result.CredentialFailure {
		t.Fatalf("CredentialFailure = false, want true（429 是凭据级失败）")
	}
	if !result.ReduceWeight {
		t.Fatalf("ReduceWeight = false, want true（单 key 阶段组织级标记保留现状语义）")
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.CooldownUntil == nil {
		t.Fatalf("CooldownUntil = nil, want 429 已写冷却")
	}
	if got.CooldownReason != "http_429" {
		t.Fatalf("CooldownReason = %q, want http_429", got.CooldownReason)
	}
}
