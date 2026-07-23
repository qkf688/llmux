package adjustment

import "github.com/atopos31/llmio/service/healthcheck"

func init() {
	healthcheck.SetAdjustmentHooks(healthcheck.AdjustmentHooks{
		ShouldCountHealthCheckSuccess: ShouldCountHealthCheckSuccess,
		ShouldCountHealthCheckFailure: ShouldCountHealthCheckFailure,
		ApplySuccessAdjustments:       ApplySuccessAdjustments,
		ApplyWeightDecay:              ApplyWeightDecayByModelProviderID,
		ApplyPriorityDecay:            ApplyPriorityDecayByModelProviderID,
	})
}
