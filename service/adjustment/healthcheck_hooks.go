package adjustment

import "github.com/qkf688/llmux/service/healthcheck"

func init() {
	healthcheck.SetAdjustmentHooks(healthcheck.AdjustmentHooks{
		ShouldCountHealthCheckSuccess: ShouldCountHealthCheckSuccess,
		ShouldCountHealthCheckFailure: ShouldCountHealthCheckFailure,
		ApplySuccessAdjustments:       ApplySuccessAdjustments,
		ApplyWeightDecay:              ApplyWeightDecayByModelProviderID,
		ApplyPriorityDecay:            ApplyPriorityDecayByModelProviderID,
	})
}
