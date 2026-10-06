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
	"github.com/qkf688/llmux/service/credwrite"
)

// newStatsFixture 建齐 attempt 全链（provider/endpoint/group/mwp/密文凭据），
// 返回上游 server 与「建凭据行 → 组装 singleProviderAttemptInput」工厂。
// 模板同 chat_credential_auth_fail_test.go 的 runAttempt；AuthFailThreshold=99
// 使连败判停不干扰统计断言。
func newStatsFixture(t *testing.T, handler http.Handler) (*httptest.Server, func(note string) singleProviderAttemptInput) {
	t.Helper()
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	SetSettingsReader(credCooldownSettingsReader{Cooldown429Sec: 60, CooldownServerSec: 60, AuthFailThreshold: 99})
	t.Cleanup(func() { SetSettingsReader(nil) })

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	providerRow := models.Provider{Name: "p1", Type: providers.TypeOpenAI, Config: `{"base_url":"http://127.0.0.1:1/never"}`}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: providerRow.ID, Protocol: string(consts.ProtocolOpenAI), URL: srv.URL, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp, poolID := seedPooledGroup(t, providerRow.ID, "g1", 1, "")
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model-provider: %v", err)
	}

	plain := "sk-stats"
	makeInput := func(note string) singleProviderAttemptInput {
		cred := models.Credential{PoolID: poolID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain), Note: note}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential: %v", err)
		}
		chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":%q}`, srv.URL, plain), "")
		if err != nil {
			t.Fatalf("new provider: %v", err)
		}
		return singleProviderAttemptInput{
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
		}
	}
	return srv, makeInput
}

func reloadCredentialStats(t *testing.T, id uint) models.Credential {
	t.Helper()
	var got models.Credential
	if err := models.DB.First(&got, id).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	return got
}

// 成功尝试（上游 200）→ TotalRequests+1 且 LastUsedAt 刷新（AC-1）。
func TestExecuteSingleProviderAttempt_Success_CountsRequestAndTouchesLastUsed(t *testing.T) {
	_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","object":"chat.completion","model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`))
	}))

	input := makeInput("ok-key")
	result := executeSingleProviderAttempt(input, make(chan models.ChatLog))
	if !result.Success {
		t.Fatalf("Success = false, want true（200 直通尝试应成功）")
	}
	got := reloadCredentialStats(t, input.Selection.Credential.ID)
	if got.TotalRequests != 1 {
		t.Fatalf("TotalRequests = %d, want 1", got.TotalRequests)
	}
	if got.LastUsedAt == nil {
		t.Fatalf("LastUsedAt = nil, want 非空（成功 touch 最近使用）")
	}
	if got.TotalErrors != 0 {
		t.Fatalf("TotalErrors = %d, want 0（成功不碰错误列）", got.TotalErrors)
	}

	// 第二次成功：累计 +1，LastUsedAt 不回退
	input2 := input
	input2.Selection.Credential = got
	executeSingleProviderAttempt(input2, make(chan models.ChatLog))
	got2 := reloadCredentialStats(t, got.ID)
	if got2.TotalRequests != 2 {
		t.Fatalf("TotalRequests after 2nd = %d, want 2", got2.TotalRequests)
	}
	if got2.LastUsedAt.Before(*got.LastUsedAt) {
		t.Fatalf("LastUsedAt 回退：%v < %v", got2.LastUsedAt, got.LastUsedAt)
	}
}

// 凭据级失败 401 → TotalErrors+1，LastUsedAt/TotalRequests 不动（AC-2）；
// 与判停状态机正交：统计走独立列，既有连败记账不受影响。
func TestExecuteSingleProviderAttempt_Auth401_CountsError(t *testing.T) {
	_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid api key"}`))
	}))

	input := makeInput("e401")
	executeSingleProviderAttempt(input, make(chan models.ChatLog))
	got := reloadCredentialStats(t, input.Selection.Credential.ID)
	if got.TotalErrors != 1 {
		t.Fatalf("TotalErrors = %d, want 1（401 记一次凭据级失败）", got.TotalErrors)
	}
	if got.TotalRequests != 0 || got.LastUsedAt != nil {
		t.Fatalf("失败尝试污染成功列：TotalRequests=%d LastUsedAt=%v", got.TotalRequests, got.LastUsedAt)
	}
	if got.FailCount != 1 {
		t.Fatalf("FailCount = %d, want 1（既有连败记账不受影响）", got.FailCount)
	}
}

// 429 / 5xx 也记 TotalErrors（classifyCredentialFailure 判定域全覆盖，AC-2）。
func TestExecuteSingleProviderAttempt_CredentialFailureStatuses_CountError(t *testing.T) {
	for _, code := range []int{http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				_, _ = w.Write([]byte(`{"error":"busy"}`))
			}))
			input := makeInput(fmt.Sprintf("e%d", code))
			executeSingleProviderAttempt(input, make(chan models.ChatLog))
			got := reloadCredentialStats(t, input.Selection.Credential.ID)
			if got.TotalErrors != 1 {
				t.Fatalf("TotalErrors = %d, want 1（%d 属凭据级失败）", got.TotalErrors, code)
			}
		})
	}
}

// 网络/超时失败（Client.Do 报错）→ TotalErrors+1（AC-2 网络域）。
func TestExecuteSingleProviderAttempt_NetworkError_CountsError(t *testing.T) {
	srv, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // 制造连接失败

	input := makeInput("netfail")
	result := executeSingleProviderAttempt(input, make(chan models.ChatLog))
	if !result.CredentialFailure {
		t.Fatalf("CredentialFailure = false, want true（网络失败按 #13 属凭据级）")
	}
	got := reloadCredentialStats(t, input.Selection.Credential.ID)
	if got.TotalErrors != 1 {
		t.Fatalf("TotalErrors = %d, want 1", got.TotalErrors)
	}
	if got.CooldownUntil == nil {
		t.Fatalf("CooldownUntil = nil, want 非空（既有网络冷却不受影响）")
	}
}

// 组织级出口不记账（AC-3）：其余 4xx（400 请求体被拒）不该让共享 key 背锅。
func TestExecuteSingleProviderAttempt_OrgLevelFailure_NoStats(t *testing.T) {
	_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_request"}`))
	}))

	input := makeInput("org400")
	executeSingleProviderAttempt(input, make(chan models.ChatLog))
	got := reloadCredentialStats(t, input.Selection.Credential.ID)
	if got.TotalErrors != 0 || got.TotalRequests != 0 {
		t.Fatalf("组织级 400 记账了：TotalRequests=%d TotalErrors=%d, want 0/0", got.TotalRequests, got.TotalErrors)
	}
	if got.LastUsedAt != nil {
		t.Fatalf("LastUsedAt = %v, want nil", got.LastUsedAt)
	}
}

// 零值 Credential（存量测试直调 attempt 的构造形态）不得 panic、不得投递写
// ——防御壳与 applyCredentialCooldown 同款。
func TestRecordCredentialStats_ZeroCredentialNoop(t *testing.T) {
	initChatRecordTestDB(t)
	recordCredentialStatsSuccess(context.Background(), models.Credential{})
	recordCredentialStatsError(context.Background(), models.Credential{})
	if n := credwrite.DroppedCount(); n != 0 {
		t.Fatalf("dropped = %d, want 0（零值凭据不应触碰队列）", n)
	}
}
