package chat

import (
	"context"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/credwrite"
	"github.com/qkf688/llmux/service/settings"
)

func init() {
	credwrite.SetHandlers(credwrite.Handlers{
		Cooldown:      persistCredentialCooldown,
		AuthFail:      persistCredentialAuthFailure,
		RecordSuccess: persistCredentialRecordSuccess,
		RecordError:   persistCredentialRecordError,
	})
}

func persistCredentialCooldown(ctx context.Context, credID uint, reason string) error {
	window, ok := cooldownWindowForReason(ctx, reason)
	if !ok {
		// 不写冷却：开关关闭（运营关停）或窗口读失败（已留痕）——不拿编造窗口写冷却（Stage 33，Ruling 4）。
		return nil
	}
	_, err := repos().Credential.UpdateFields(ctx, credID, map[string]any{
		"cooldown_until":  time.Now().Add(window),
		"cooldown_reason": reason,
	})
	return err
}

func persistCredentialAuthFailure(ctx context.Context, credID uint) error {
	if !getCredHealthAuthFailStopEnabled(ctx) {
		// 判停开关关闭：只计数，绝不判停——判停 = 自动停用凭据，是运营要能一键关停的动作。
		_, err := repos().Credential.IncrementFailCountNoStop(ctx, credID)
		return err
	}
	threshold, err := getCredHealthAuthFailThreshold(ctx)
	if err != nil {
		// 判停阈值读失败：只计数不判停（已留痕）——temp_unsched 是停用类动作，
		// 绝不拿编造阈值执行（Stage 33 显式降级）。
		settings.LogReadSkip("credential auth-fail stop", models.SettingKeyCredHealthAuthFailThreshold, err)
		_, err := repos().Credential.IncrementFailCountNoStop(ctx, credID)
		return err
	}
	_, err = repos().Credential.IncrementFailCountAndStopIfThreshold(ctx, credID, threshold, cooldownReasonAuthFail)
	return err
}

func persistCredentialRecordSuccess(ctx context.Context, credID uint, at time.Time) error {
	_, err := repos().Credential.RecordCredentialSuccess(ctx, credID, at)
	return err
}

func persistCredentialRecordError(ctx context.Context, credID uint) error {
	_, err := repos().Credential.RecordCredentialError(ctx, credID)
	return err
}
