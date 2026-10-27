package chat

import (
	"context"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/channel"
)

// 本文件锁「凭据健康四机制默认全关」这一交付承诺（2026-10-27）：
// 四个开关 schema Default=false，models.Init 的播种按默认值建行，settings 表里
// 因此**没有任何行**表示「开关打开」——不配置即等于关停。
//
// 为什么必须单独用例会锁：其余机制用例都显式打开了开关（enableCredHealthSwitches
// 或 stub 恒真），「默认关」若哪天被改成默认开，全仓测试仍然全绿——本文件是
// 唯一能对此变红的那条。

// TestCredHealthDefaults_AllSwitchesOff 锁生产默认：四个开关读回全是关。
func TestCredHealthDefaults_AllSwitchesOff(t *testing.T) {
	initChatRecordTestDB(t)
	// 不装 stub、不写表：走默认读库 + 播种值（models.Init → initDefaultSettings）。

	ctx := context.Background()
	for _, tc := range []struct {
		name string
		got  bool
	}{
		{"429 冷却开关", getCredHealthCooldown429Enabled(ctx)},
		{"服务端错误冷却开关", getCredHealthCooldownServerEnabled(ctx)},
		{"鉴权失败判停开关", getCredHealthAuthFailStopEnabled(ctx)},
		{"惰性探活开关", getCredHealthProbeEnabled(ctx)},
	} {
		if tc.got {
			t.Errorf("%s = true, want false（默认关：自动冷却/自动停用不默认生效）", tc.name)
		}
	}
}

// TestCredHealthDefaults_NoCooldownNoStop 锁默认配置下的实际行为：
// 429 失败不写冷却（凭据保持 active、继续参与选路）、401/403 只计数不判停。
// 走真实 settings 表 + 真实 repository 写路径（不经 stub）。
func TestCredHealthDefaults_NoCooldownNoStop(t *testing.T) {
	initChatRecordTestDB(t)

	cred := models.Credential{Key: "enc", KeyHash: "hash"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	// 429 → 凭据级冷却写
	if err := persistCredentialCooldown(context.Background(), cred.ID, cooldownReason429); err != nil {
		t.Fatalf("persist cooldown: %v", err)
	}
	// 401 → 鉴权失败结算写
	for range 5 {
		if err := persistCredentialAuthFailure(context.Background(), cred.ID); err != nil {
			t.Fatalf("persist auth fail: %v", err)
		}
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.CooldownUntil != nil {
		t.Fatalf("CooldownUntil = %v, want nil（默认关：429 不写冷却）", got.CooldownUntil)
	}
	if got.CooldownReason != "" {
		t.Fatalf("CooldownReason = %q, want 空（默认关：不留冷却痕迹）", got.CooldownReason)
	}
	if got.Status != models.CredentialStatusActive {
		t.Fatalf("Status = %q, want active（默认关：连续 5 次 401 也不判停）", got.Status)
	}
	if got.FailCount != 5 {
		t.Fatalf("FailCount = %d, want 5（默认关时失败次数照记，只是不判停）", got.FailCount)
	}
}

// TestCredHealthDefaults_ProbeDisabled_NoAutoRecover 锁默认配置下探活不生效：
// temp_unsched 凭据不会被自动探活恢复（该凭据只能人工改回 active）。
// 走真实 settings 表 + 真实选路装配（不经 stub）。
func TestCredHealthDefaults_ProbeDisabled_NoAutoRecover(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)

	plain := "sk-default-off"
	cred := models.Credential{
		Key:            mustEncrypt(t, cipher, plain),
		KeyHash:        cipher.Hash(plain),
		Status:         models.CredentialStatusTempUnsched,
		CooldownReason: cooldownReasonAuthFail,
		FailCount:      3,
	}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	// 探活开关默认关 → 直接返回 false，不打上游、不恢复凭据。
	if probeRecoverInGroup(context.Background(),
		models.Provider{Name: "p", Type: "openai"},
		models.Endpoint{Protocol: "openai", URL: "http://127.0.0.1:1/never", Enabled: true},
		[]models.Credential{cred},
	) {
		t.Fatal("probeRecoverInGroup = true, want false（默认关：探活不生效）")
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.Status != models.CredentialStatusTempUnsched {
		t.Fatalf("Status = %q, want temp_unsched（默认关：判停凭据不自动恢复）", got.Status)
	}
	if got.LastProbeAt != nil && time.Since(*got.LastProbeAt) < time.Minute {
		t.Fatal("LastProbeAt 被刷新，want 默认关时不探活")
	}
}
