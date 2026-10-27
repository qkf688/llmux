package chat

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/settings"
)

// 本文件是 Stage 33「凭据结算腿设置读取失败 → 显式降级」的回归组（Ruling 3）：
// 判停阈值 / 冷却窗口读不到时不得拿编造值驱动停用类动作或冷却写；计数类维护照记。

// TestPersistCredentialAuthFailure_ThresholdReadFail_CountsWithoutStop 判停阈值读失败
// → 只计数不判停（不拿默认阈值 3/覆写阈值 1 执行 temp_unsched）。
func TestPersistCredentialAuthFailure_ThresholdReadFail_CountsWithoutStop(t *testing.T) {
	initChatRecordTestDB(t)
	installChatTestSettings(t, credCooldownSettingsReader{
		AuthFailThreshold: 1, // 裸变体的危险值：旧代码会拿它首败即判停
		failKeys: map[string]bool{
			models.SettingKeyCredHealthAuthFailThreshold: true,
		},
	})

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
	installChatTestSettings(t, credCooldownSettingsReader{
		Cooldown429Sec: 60, // 危险覆写值：读不到时若退它，就会写出一段编造的冷却
		failKeys: map[string]bool{
			models.SettingKeyCredHealthCooldown429Sec: true,
		},
	})

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

// TestPersistCredentialCooldown_ServerWindowReadFail_SkipsCooldown 服务端窗冷却读失败腿
// （Stage 36 补缺口）：5xx/网络与 429 共用一个分窗判定函数，[非 429 走服务端窗] 这条
// 分支此前没有读失败用例——同构代码不等于已覆盖，须同样跳过冷却写、不得退默认窗口。
func TestPersistCredentialCooldown_ServerWindowReadFail_SkipsCooldown(t *testing.T) {
	initChatRecordTestDB(t)
	installChatTestSettings(t, credCooldownSettingsReader{
		CooldownServerSec: 120, // 危险覆写值：读不到时若退它，就会写出一段编造的冷却
		failKeys: map[string]bool{
			models.SettingKeyCredHealthCooldownServerSec: true,
		},
	})

	cred := models.Credential{Key: "enc", KeyHash: "hash"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	// 非 429 的凭据级失败（5xx）走服务端窗
	if err := persistCredentialCooldown(context.Background(), cred.ID, cooldownReasonForStatus(500)); err != nil {
		t.Fatalf("persist cooldown: %v", err)
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.CooldownUntil != nil {
		t.Fatalf("CooldownUntil != nil (reason=%q), want 服务端窗读不到 → 冷却写跳过", got.CooldownReason)
	}
}

// captureSlog 捕获 slog 输出（断言「读失败已留痕」用）。返回捕获文本与恢复函数。
//
// 为什么要捕获：开关读失败与「运营主动关停」在行为上同形（都按关处理），
// 唯一可观测的差别就是留痕——不留痕则两者永远无法区分。
func captureSlog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	restore := func() { slog.SetDefault(old) }
	t.Cleanup(restore)
	// 留痕限频是进程级的（60 秒窗口）：不重置的话上一轮用例的留痕会把本轮压掉，
	// 断言留痕的用例会假红（-count>1 尤其明显）。
	settings.ResetReadSkipLimiterForTest()
	return buf, restore
}

// TestCredHealthSwitchReadFail_LogsAndTreatsAsOff 锁「开关读失败 → 可辨 + 留痕」
// （Stage 33）：四个开关用 BoolE 读取，读失败时按「关」处理（保守方向），
// 并经 settings.LogReadSkip 留痕——否则「设置读不到」与「运营主动关停」同形，
// DB 抖动导致机制静默失效时事后无从判断。
func TestCredHealthSwitchReadFail_LogsAndTreatsAsOff(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		readErr error
	}{
		{"429 冷却开关", models.SettingKeyCredHealthCooldown429Enabled, nil},
		{"服务端冷却开关", models.SettingKeyCredHealthCooldownServerEnabled, nil},
		{"判停开关", models.SettingKeyCredHealthAuthFailStopEnabled, nil},
		{"探活开关", models.SettingKeyCredHealthProbeEnabled, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initChatRecordTestDB(t)
			installChatTestSettings(t, credCooldownSettingsReader{
				failKeys: map[string]bool{tc.key: true},
			})
			buf, _ := captureSlog(t)

			if got := getCredHealthSwitch(context.Background(), tc.key); got {
				t.Fatalf("getCredHealthSwitch(%s) = true, want false（读失败按关处理）", tc.key)
			}
			if !strings.Contains(buf.String(), tc.key) {
				t.Fatalf("留痕缺设置键 %s，日志=%s", tc.key, buf.String())
			}
			if !strings.Contains(buf.String(), "settings read failed") {
				t.Fatalf("留痕缺「settings read failed」消息，日志=%s", buf.String())
			}
		})
	}
}

// TestPersistCredentialCooldown_SwitchReadFail_SkipsCooldown 冷却开关读失败腿：
// 与「运营主动关停」同效——不写冷却；且因开关读失败发生在窗口读之前，
// 窗口值读不到也不写（拿编造窗口写冷却才是 Stage 33 禁止的）。
func TestPersistCredentialCooldown_SwitchReadFail_SkipsCooldown(t *testing.T) {
	initChatRecordTestDB(t)
	installChatTestSettings(t, credCooldownSettingsReader{
		Cooldown429Sec:    60,
		CooldownServerSec: 60,
		failKeys: map[string]bool{
			models.SettingKeyCredHealthCooldown429Enabled: true,
		},
	})
	captureSlog(t)

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
		t.Fatal("CooldownUntil != nil, want 开关读失败 → 冷却写跳过")
	}
}

// TestPersistCredentialAuthFailure_SwitchReadFail_CountsWithoutStop 判停开关读失败腿：
// 读不到开关时按「关」处理——只计数、不判停（绝不拿编造阈值停用凭据）。
func TestPersistCredentialAuthFailure_SwitchReadFail_CountsWithoutStop(t *testing.T) {
	initChatRecordTestDB(t)
	installChatTestSettings(t, credCooldownSettingsReader{
		AuthFailThreshold: 1, // 危险覆写值：开关读不到时若退它/若判停，首败即停
		failKeys: map[string]bool{
			models.SettingKeyCredHealthAuthFailStopEnabled: true,
		},
	})
	captureSlog(t)

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
		t.Fatalf("FailCount = %d, want 3（计数不因开关读失败而丢）", got.FailCount)
	}
	if got.Status != models.CredentialStatusActive {
		t.Fatalf("Status = %q, want active（开关读不到 → 按关处理，不得停用凭据）", got.Status)
	}
}
