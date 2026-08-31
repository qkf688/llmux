package credprobe

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/credwrite"
)

func init() {
	// merge 注册，不清掉 chat 侧 Cooldown/AuthFail（#6-4-2）。
	credwrite.SetHandlers(credwrite.Handlers{
		TouchProbe: persistTouchProbe,
		Recover:    persistRecover,
	})
}

func persistTouchProbe(ctx context.Context, credID uint, at time.Time) error {
	_, err := repos().Credential.UpdateFields(ctx, credID, map[string]any{
		"last_probe_at": at,
	})
	if err != nil {
		return fmt.Errorf("touch last_probe_at: %w", err)
	}
	return nil
}

// persistRecover 条件恢复 active；未落地时同步 touch LastProbeAt（已在 worker 内，不再入队）。
func persistRecover(ctx context.Context, credID uint, at time.Time) (bool, error) {
	fields := map[string]any{
		"status":        models.CredentialStatusActive,
		"last_probe_at": at,
	}
	for k, v := range models.CredentialRecoveryFields(models.CredentialStatusActive) {
		fields[k] = v
	}
	n, err := repos().Credential.UpdateFieldsIfStatus(ctx, credID, models.CredentialStatusTempUnsched, fields)
	if err != nil {
		if touchErr := persistTouchProbe(ctx, credID, at); touchErr != nil {
			slog.Warn("failed to touch last_probe_at after recover error",
				"credential_id", credID, "error", touchErr)
		}
		return false, fmt.Errorf("recover credential: %w", err)
	}
	if n == 0 {
		if touchErr := persistTouchProbe(ctx, credID, at); touchErr != nil {
			return false, fmt.Errorf("touch last_probe_at after recover skip: %w", touchErr)
		}
		return false, nil
	}
	return true, nil
}
