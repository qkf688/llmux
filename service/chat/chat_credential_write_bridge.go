package chat

import (
	"context"
	"time"

	"github.com/qkf688/llmux/service/credwrite"
)

func init() {
	credwrite.SetHandlers(credwrite.Handlers{
		Cooldown: persistCredentialCooldown,
		AuthFail: persistCredentialAuthFailure,
	})
}

func persistCredentialCooldown(ctx context.Context, credID uint, reason string) error {
	window := cooldownWindowForReason(ctx, reason)
	_, err := repos().Credential.UpdateFields(ctx, credID, map[string]any{
		"cooldown_until":  time.Now().Add(window),
		"cooldown_reason": reason,
	})
	return err
}

func persistCredentialAuthFailure(ctx context.Context, credID uint) error {
	threshold := getCredHealthAuthFailThreshold(ctx)
	_, err := repos().Credential.IncrementFailCountAndStopIfThreshold(ctx, credID, threshold, cooldownReasonAuthFail)
	return err
}
