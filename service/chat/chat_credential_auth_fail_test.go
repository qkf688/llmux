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

// TestClassifyAuthFailure 锁「鉴权失败（401/403）走判停路径」的分类（#6-2）：
// 401/403 是凭据级失败的子类，但不走冷却——改为连续失败计数 + 达阈值判停。
func TestClassifyAuthFailure(t *testing.T) {
	cases := []struct {
		name string
		code int
		want bool
	}{
		{"401 未授权", http.StatusUnauthorized, true},
		{"403 禁止", http.StatusForbidden, true},
		{"429 限流（走冷却）", http.StatusTooManyRequests, false},
		{"500 服务端（走冷却）", http.StatusInternalServerError, false},
		{"400 请求体被拒（组织级）", http.StatusBadRequest, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyAuthFailure(tc.code); got != tc.want {
				t.Fatalf("classifyAuthFailure(%d) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}

// TestApplyCredentialAuthFailure_CountsThenStops 锁 #6-2 判停语义：
// 401/403 只计数不写冷却（Status 保持 active、CooldownUntil 不写）；
// 连续失败达阈值 N → temp_unsched + reason=auth_fail + 清 cooldown_until。
func TestApplyCredentialAuthFailure_CountsThenStops(t *testing.T) {
	initChatRecordTestDB(t)
	SetSettingsReader(credCooldownSettingsReader{Cooldown429Sec: 60, CooldownServerSec: 60, AuthFailThreshold: 3})
	t.Cleanup(func() { SetSettingsReader(nil) })

	grp := models.KeyGroup{ProviderID: 1, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	cred := models.Credential{GroupID: &grp.ID, Key: "enc", KeyHash: "hash"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	// 前 2 次：只计数，不冷却不判停
	for i := 1; i <= 2; i++ {
		applyCredentialAuthFailure(context.Background(), cred)
		var got models.Credential
		if err := models.DB.First(&got, cred.ID).Error; err != nil {
			t.Fatalf("reload credential (step %d): %v", i, err)
		}
		if got.Status != models.CredentialStatusActive {
			t.Fatalf("step %d Status = %q, want active（未达阈值只计数）", i, got.Status)
		}
		if got.FailCount != i {
			t.Fatalf("step %d FailCount = %d, want %d", i, got.FailCount, i)
		}
		if got.CooldownUntil != nil {
			t.Fatalf("step %d CooldownUntil != nil, want 鉴权失败不写冷却", i)
		}
	}

	// 第 3 次：达阈值 → 判停
	applyCredentialAuthFailure(context.Background(), cred)
	var stopped models.Credential
	if err := models.DB.First(&stopped, cred.ID).Error; err != nil {
		t.Fatalf("reload credential (stopped): %v", err)
	}
	if stopped.Status != models.CredentialStatusTempUnsched {
		t.Fatalf("Status = %q, want temp_unsched（连败 x3 判停）", stopped.Status)
	}
	if stopped.CooldownReason != cooldownReasonAuthFail {
		t.Fatalf("CooldownReason = %q, want %q（UI 详情行区分用机器码）", stopped.CooldownReason, cooldownReasonAuthFail)
	}
	if stopped.CooldownUntil != nil {
		t.Fatalf("CooldownUntil != nil, want nil（temp_unsched 不靠冷却恢复）")
	}
	if stopped.FailCount != 3 {
		t.Fatalf("FailCount = %d, want 3（判停后保留计数，恢复时由 #6-3/成功路径清零）", stopped.FailCount)
	}
}

// TestApplyCredentialAuthFailure_ConfigurableThreshold 锁「阈值 N 可配」（#6-2）：
// N=1 首败即判停；N=5 第 5 次才停——阈值经设置项读取（stub 注入），非硬编码。
func TestApplyCredentialAuthFailure_ConfigurableThreshold(t *testing.T) {
	for _, n := range []int{1, 2, 5} {
		t.Run(fmt.Sprintf("threshold=%d", n), func(t *testing.T) {
			initChatRecordTestDB(t)
			SetSettingsReader(credCooldownSettingsReader{AuthFailThreshold: n})
			t.Cleanup(func() { SetSettingsReader(nil) })

			cred := models.Credential{Key: "enc", KeyHash: "hash"}
			if err := models.DB.Create(&cred).Error; err != nil {
				t.Fatalf("create credential: %v", err)
			}
			for i := 1; i < n; i++ {
				applyCredentialAuthFailure(context.Background(), cred)
				var got models.Credential
				if err := models.DB.First(&got, cred.ID).Error; err != nil {
					t.Fatalf("reload credential (step %d): %v", i, err)
				}
				if got.Status != models.CredentialStatusActive {
					t.Fatalf("threshold=%d step %d Status = %q, want active", n, i, got.Status)
				}
			}
			applyCredentialAuthFailure(context.Background(), cred)
			var stopped models.Credential
			if err := models.DB.First(&stopped, cred.ID).Error; err != nil {
				t.Fatalf("reload credential (stopped): %v", err)
			}
			if stopped.Status != models.CredentialStatusTempUnsched {
				t.Fatalf("threshold=%d 连败 %d 次后 Status = %q, want temp_unsched", n, n, stopped.Status)
			}
		})
	}
}

// 上游 401 → executeSingleProviderAttempt 走鉴权失败判停路径（端到端）：
// attempt 结果语义与现状一致（CredentialFailure + 组织级淘汰标记），
// 凭据侧不写冷却只计数，连败 x3 → temp_unsched（AC-1/AC-2/AC-6）。
func TestExecuteSingleProviderAttempt_NonOK401_AuthFailStopsCredential(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	SetSettingsReader(credCooldownSettingsReader{Cooldown429Sec: 60, CooldownServerSec: 60, AuthFailThreshold: 3})
	t.Cleanup(func() { SetSettingsReader(nil) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
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
	plain := "sk-authfail"
	cred := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain), Note: "authfail-key"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model-provider: %v", err)
	}

	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-authfail"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	runAttempt := func() singleProviderAttemptResult {
		return executeSingleProviderAttempt(singleProviderAttemptInput{
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
	}

	// 第 1 次 401：attempt 结果语义不变，凭据侧只计数不冷却
	result := runAttempt()
	if !result.CredentialFailure {
		t.Fatalf("CredentialFailure = false, want true（401 是凭据级失败）")
	}
	if !result.RemoveWeight || !result.RemovePriority {
		t.Fatalf("RemoveWeight/RemovePriority = %v/%v, want true（组织级淘汰标记语义不变）", result.RemoveWeight, result.RemovePriority)
	}
	var after1 models.Credential
	if err := models.DB.First(&after1, cred.ID).Error; err != nil {
		t.Fatalf("reload credential (after 1st): %v", err)
	}
	if after1.Status != models.CredentialStatusActive || after1.FailCount != 1 {
		t.Fatalf("第 1 次 401 后 Status=%q FailCount=%d, want active/1（未达阈值只计数）", after1.Status, after1.FailCount)
	}
	if after1.CooldownUntil != nil {
		t.Fatalf("第 1 次 401 后 CooldownUntil != nil, want 鉴权失败不写冷却")
	}

	// 连败至 3 → 判停
	for range 2 {
		runAttempt()
	}
	var stopped models.Credential
	if err := models.DB.First(&stopped, cred.ID).Error; err != nil {
		t.Fatalf("reload credential (stopped): %v", err)
	}
	if stopped.Status != models.CredentialStatusTempUnsched {
		t.Fatalf("连败 3 次 401 后 Status = %q, want temp_unsched", stopped.Status)
	}
	if stopped.CooldownReason != cooldownReasonAuthFail {
		t.Fatalf("CooldownReason = %q, want %q", stopped.CooldownReason, cooldownReasonAuthFail)
	}
	if stopped.CooldownUntil != nil {
		t.Fatalf("CooldownUntil != nil, want nil（401/403 全程不写冷却）")
	}
}

// TestResetCredentialAuthFailCount 锁 #6-2 成功重置语义（AC-3）：
// FailCount>0 的凭据计数清零（判停判据归位）；FailCount=0 时不产生写库——
// 成功是热路径，常态请求不应有任何凭据写（#6-4 异步落库前的必要约束）。
func TestResetCredentialAuthFailCount(t *testing.T) {
	initChatRecordTestDB(t)

	t.Run("计数大于 0 清零", func(t *testing.T) {
		cred := models.Credential{Key: "enc", KeyHash: "hash", FailCount: 2}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential: %v", err)
		}
		resetCredentialAuthFailCount(context.Background(), cred)
		var got models.Credential
		if err := models.DB.First(&got, cred.ID).Error; err != nil {
			t.Fatalf("reload credential: %v", err)
		}
		if got.FailCount != 0 {
			t.Fatalf("FailCount = %d, want 0（成功后重置）", got.FailCount)
		}
		if got.Status != models.CredentialStatusActive {
			t.Fatalf("Status = %q, want active（重置只清计数不动状态）", got.Status)
		}
	})

	t.Run("计数为 0 不写库", func(t *testing.T) {
		cred := models.Credential{Key: "enc2", KeyHash: "hash2", FailCount: 0}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential: %v", err)
		}
		resetCredentialAuthFailCount(context.Background(), cred)
		var got models.Credential
		if err := models.DB.First(&got, cred.ID).Error; err != nil {
			t.Fatalf("reload credential: %v", err)
		}
		if got.FailCount != 0 {
			t.Fatalf("FailCount = %d, want 0", got.FailCount)
		}
	})
}

// 端到端：凭据带历史鉴权失败计数（FailCount=2）→ 上游 200 成功 →
// executeSingleProviderAttempt 成功出口重置计数为 0（锁挂点，AC-3）。
func TestExecuteSingleProviderAttempt_SuccessResetsAuthFailCount(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
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
	plain := "sk-resets"
	cred := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain), Note: "reset-key", FailCount: 2}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model-provider: %v", err)
	}

	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-resets"}`, srv.URL), "")
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

	if !result.Success {
		t.Fatalf("Success = false, want true（200 直通成功）")
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.FailCount != 0 {
		t.Fatalf("FailCount = %d, want 0（成功出口重置鉴权失败计数）", got.FailCount)
	}
	if got.Status != models.CredentialStatusActive {
		t.Fatalf("Status = %q, want active", got.Status)
	}
}
