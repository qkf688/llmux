package chat

import (
	"context"
	"testing"

	"github.com/qkf688/llmux/models"
)

// 本文件是 Stage 33「凭据结算腿设置读取失败 → 显式降级」的回归组（Ruling 3）：
// 判停阈值 / 冷却窗口读不到时不得拿编造值驱动停用类动作或冷却写；计数类维护照记。

// TestPersistCredentialAuthFailure_ThresholdReadFail_CountsWithoutStop 判停阈值读失败
// → 只计数不判停（不拿默认阈值 3/覆写阈值 1 执行 temp_unsched）。
func TestPersistCredentialAuthFailure_ThresholdReadFail_CountsWithoutStop(t *testing.T) {
	initChatRecordTestDB(t)
	SetSettingsReader(credCooldownSettingsReader{
		AuthFailThreshold: 1, // 裸变体的危险值：旧代码会拿它首败即判停
		failKeys: map[string]bool{
			models.SettingKeyCredHealthAuthFailThreshold: true,
		},
	})
	t.Cleanup(func() { SetSettingsReader(nil) })

	cred := models.Credential{Key: "enc", KeyHash: "hash"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	for i := 1; i <= 3; i++ {
		if err := persistCredentialAuthFailure(context.Background(), cred.ID); err != nil {
			t.Fatalf("persist auth fail (step %d): %v", i, err)
		}
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.FailCount != 3 {
		t.Fatalf("FailCount = %d, want 3（计数不因阈值读失败而丢）", got.FailCount)
	}
	if got.Status != models.CredentialStatusActive {
		t.Fatalf("Status = %q, want active（阈值读不到 → 判停跳过，不得拿编造阈值停用）", got.Status)
	}
}

// TestPersistCredentialCooldown_WindowReadFail_SkipsCooldown 冷却窗口读失败
// → 冷却写整体跳过（Ruling 4：不用默认窗口兜底），不写 CooldownUntil。
func TestPersistCredentialCooldown_WindowReadFail_SkipsCooldown(t *testing.T) {
	initChatRecordTestDB(t)
	SetSettingsReader(credCooldownSettingsReader{
		Cooldown429Sec: 60, // 裸变体的覆写值：旧代码会拿它写冷却
		failKeys: map[string]bool{
			models.SettingKeyCredHealthCooldown429Sec: true,
		},
	})
	t.Cleanup(func() { SetSettingsReader(nil) })

	cred := models.Credential{Key: "enc", KeyHash: "hash"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	if err := persistCredentialCooldown(context.Background(), cred.ID, cooldownReason429); err != nil {
		t.Fatalf("persist cooldown: %v", err)
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.CooldownUntil != nil {
		t.Fatal("CooldownUntil != nil, want 窗口读不到 → 冷却写跳过（不拿编造窗口写冷却）")
	}
}
