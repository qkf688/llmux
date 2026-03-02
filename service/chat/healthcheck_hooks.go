package chat

import "github.com/atopos31/llmio/service/healthcheck"

func init() {
	healthcheck.SetAdjustmentHooks(healthcheck.AdjustmentHooks{
		ShouldCountHealthCheckSuccess: shouldCountHealthCheckSuccess,
		ShouldCountHealthCheckFailure: shouldCountHealthCheckFailure,
		ApplySuccessAdjustments:       applySuccessAdjustments,
		ApplyWeightDecay:              applyWeightDecayByModelProviderID,
		ApplyPriorityDecay:            applyPriorityDecayByModelProviderID,
	})
}
