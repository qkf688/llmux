package chat

import (
	"context"
	"errors"
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
	installChatTestSettings(t, credCooldownSettingsReader{Cooldown429Sec: 60, CooldownServerSec: 60})

	_, poolID := seedPooledGroup(t, 1, "g1", 1, "")
	cred := models.Credential{PoolID: poolID, Key: "enc", KeyHash: "hash"}
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
	if diff := time.Until(*got.CooldownUntil); diff > 61*time.Second {
		t.Fatalf("CooldownUntil = %v, want within 60s window", diff)
	}
	if got.CooldownReason != "http_429" {
		t.Fatalf("CooldownReason = %q, want http_429", got.CooldownReason)
	}
}

// credCooldownSettingsReader 测试用 settings.Reader stub：只覆写凭据健康键，
// 其余回退 defaultValue。窗口最小粒度秒。
//
// failKeys 命中 = 读失败，**裸变体与 E 变体一起失败**（Stage 36 按生产三态建模：
// 生产里两者读同一张表，不存在「只有 E 变体读不到」的世界；旧 mock 让裸变体照旧
// 返回覆写值，会把「读失败时裸读退覆写值驱动写」这条真实风险从测试里抹掉）。
type credCooldownSettingsReader struct {
	Cooldown429Sec    int
	CooldownServerSec int
	AuthFailThreshold int
	ProbeIntervalSec  int
	failKeys          map[string]bool
}

func (r credCooldownSettingsReader) readErr(key string) error {
	if r.failKeys[key] {
		return errors.New("simulated settings read failure")
	}
	return nil
}

func (r credCooldownSettingsReader) Bool(_ context.Context, key string, def bool) bool {
	if r.readErr(key) != nil {
		return def
	}
	return def
}

func (r credCooldownSettingsReader) Int(_ context.Context, key string, def, _ int) int {
	if r.readErr(key) != nil {
		return def
	}
	switch key {
	case models.SettingKeyCredHealthCooldown429Sec:
		return r.Cooldown429Sec
	case models.SettingKeyCredHealthCooldownServerSec:
		return r.CooldownServerSec
	case models.SettingKeyCredHealthAuthFailThreshold:
		return r.AuthFailThreshold
	case models.SettingKeyCredHealthProbeIntervalSec:
		if r.ProbeIntervalSec > 0 {
			return r.ProbeIntervalSec
		}
		return def
	}
	return def
}

func (r credCooldownSettingsReader) String(_ context.Context, _ string, def string) string {
	return def
}

func (r credCooldownSettingsReader) BoolE(ctx context.Context, key string, def bool) (bool, error) {
	if err := r.readErr(key); err != nil {
		return def, err
	}
	return r.Bool(ctx, key, def), nil
}

func (r credCooldownSettingsReader) IntE(ctx context.Context, key string, def, min int) (int, error) {
	if err := r.readErr(key); err != nil {
		return def, err
	}
	return r.Int(ctx, key, def, min), nil
}

// TestApplyCredentialCooldown_ConfigurableWindows 锁「冷却窗口可配 + 按失败类型分窗」（#6-1）：
// 429 走 429 窗口独立读设置项；5xx/网络共用服务端窗口。reason 机器码不变。
// 401/403 不在本表：#6-2 起走鉴权判停路径（不写冷却），见 chat_credential_auth_fail_test.go。
func TestApplyCredentialCooldown_ConfigurableWindows(t *testing.T) {
	initChatRecordTestDB(t)
	installChatTestSettings(t, credCooldownSettingsReader{Cooldown429Sec: 10, CooldownServerSec: 120})

	cases := []struct {
		name    string
		reason  string
		wantSec int
	}{
		{"429 限流走 429 窗口", "http_429", 10},
		{"5xx 服务端走 server 窗口", "http_500", 120},
		{"网络失败走 server 窗口", "network", 120},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, poolID := seedPooledGroup(t, 1, "g1", 1, "")
			cred := models.Credential{PoolID: poolID, Key: "enc", KeyHash: "hash"}
			if err := models.DB.Create(&cred).Error; err != nil {
				t.Fatalf("create credential: %v", err)
			}

			before := time.Now()
			applyCredentialCooldown(context.Background(), cred, tc.reason)

			var got models.Credential
			if err := models.DB.First(&got, cred.ID).Error; err != nil {
				t.Fatalf("reload credential: %v", err)
			}
			if got.CooldownUntil == nil {
				t.Fatalf("CooldownUntil = nil, want 冷却已写入")
			}
			want := time.Duration(tc.wantSec) * time.Second
			if got.CooldownUntil.Before(before.Add(want-time.Second)) || got.CooldownUntil.After(before.Add(want+time.Second)) {
				t.Fatalf("CooldownUntil = %v (now+%v), want now+%v", got.CooldownUntil, got.CooldownUntil.Sub(before), want)
			}
			if got.CooldownReason != tc.reason {
				t.Fatalf("CooldownReason = %q, want %q（机器码不变）", got.CooldownReason, tc.reason)
			}
		})
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
	grp, poolID := seedPooledGroup(t, providerRow.ID, "g1", 1, "")
	plain := "sk-cooldown"
	cred := models.Credential{PoolID: poolID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain), Note: "cooldown-key"}
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
	// 429 一票淘汰（2026-09-22 有意废弃旧「ReduceWeight 留池降权」语义）：
	// 限流候选本次请求内不再回头，防重试循环随机撞回刚限流的供应商。
	if !result.RemoveWeight || !result.RemovePriority {
		t.Fatalf("RemoveWeight=%v RemovePriority=%v, want 双 true（429 一票淘汰）", result.RemoveWeight, result.RemovePriority)
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
