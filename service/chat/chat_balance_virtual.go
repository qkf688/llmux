package chat

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/virtualmodel"
)

func balanceChatVirtual(ctx context.Context, in BalanceInput) (*BalanceResult, error) {
	pwm := in.ProvidersWithMeta
	slog.Info("virtual model request",
		"virtual_model", pwm.VirtualModelName,
		"strategy", pwm.VirtualStrategy,
		"real_models_count", len(pwm.OrderedRealModels),
	)

	// 全局化后总预算语义 = 跨真实模型共享的总时长上限（单次尝试窗口见下，二者解耦：
	// 窗口到点不再拖垮后续真实模型的窗口）。Stage 33 起该上限就是请求 ctx 的
	// deadline（handler 入口经 WithRequestTotalTimeout 挂载），不再是独立计时器。
	window := upstreamAttemptWindow(ctx, in.Before.Stream)

	// 钉定的真实模型排尝试首位（缓存亲和 AC-4）：本地副本重排，不动 virtualmodel
	// 策略状态；未钉时顺序不变。故障转移语义不变——钉定模型穷尽后照常试其余模型。
	ordered := pwm.OrderedRealModels
	if sticky, ok := stickyTargetFor(ctx, in.Before); ok {
		ordered = preferRealModelsFirst(ordered, sticky.ModelID)
	}

	for modelIndex, orderedModel := range ordered {
		select {
		case <-ctx.Done():
			// 归因映射在 BalanceChat 唯一出口（mapAbortErr），此处原样上抛。
			return nil, ctx.Err()
		default:
		}

		realModel := orderedModel.Model
		slog.Info("trying real model",
			"virtual_model", pwm.VirtualModelName,
			"real_model", realModel.Name,
			"model_index", modelIndex+1,
			"total", len(pwm.OrderedRealModels),
		)

		modelWithProviders, err := queryEnabledModelProviders(ctx, realModel.ID, in.Before)
		if err != nil {
			slog.Error("failed to get providers for real model", "real_model", realModel.Name, "error", err)
			saveVirtualSkipLog(ctx, in.ReqMeta, pwm.VirtualModelName, realModel.Name, in.Style,
				fmt.Sprintf("virtual model skip: failed to query providers for real model %q: %v", realModel.Name, err))
			continue
		}
		if len(modelWithProviders) == 0 {
			slog.Warn("no providers for real model", "real_model", realModel.Name)
			saveVirtualSkipLog(ctx, in.ReqMeta, pwm.VirtualModelName, realModel.Name, in.Style,
				"virtual model skip: "+noUsableProviderReason(ctx, realModel.Name))
			continue
		}

		pool, err := buildCandidatePool(ctx, modelWithProviders)
		if err != nil {
			slog.Error("failed to get providers", "error", err)
			continue
		}

		outcome := runProviderRetryLoop(retryLoopInput{
			Ctx:            ctx,
			Start:          in.Start,
			Style:          in.Style,
			Before:         in.Before,
			ReqMeta:        in.ReqMeta,
			IOLog:          pwm.IOLog,
			Pool:           pool,
			RealModelName:  realModel.Name,
			Model:          &realModel,
			MaxRetry:       getRequestMaxRetry(ctx),
			UpstreamWindow: window,
			LogAttrs:       []any{"virtual_model", pwm.VirtualModelName, "real_model", realModel.Name},
		})

		switch outcome.Status {
		case retryLoopSucceeded:
			// 轮询类策略需在成功后推进游标，其它策略（priority/random）不推进。
			if sel, _ := virtualmodel.GetSelector(pwm.VirtualStrategy); sel.RequiresAdvanceOnSuccess() {
				virtualmodel.Default().UpdateRoundRobinIndex(pwm.VirtualModelID, len(pwm.OrderedRealModels))
			}
			slog.Info("virtual model request succeeded",
				"virtual_model", pwm.VirtualModelName,
				"real_model", realModel.Name,
				"provider", outcome.Result.ProviderName,
			)
			return outcome.Result, nil
		case retryLoopAborted:
			return nil, outcome.Err
		}

		slog.Warn("all providers exhausted for real model, trying next", "real_model", realModel.Name)
	}

	return nil, retryExhaustedOutcome(ctx, "all real models exhausted for virtual model")
}

// saveVirtualSkipLog 记录「某个真实模型被整体跳过」的日志：此时尚未选出供应商，
// 故只有模型层面的信息可填。ReqMeta 整包传入（与 retryLoopInput 同风格）——
// 归因快照随建行落库，散参会让下一个旁路建行点再次漏掉。
func saveVirtualSkipLog(ctx context.Context, reqMeta models.ReqMeta, virtualModelName, realModelName, style, errMsg string) {
	savePreProviderFailureLog(ctx, reqMeta, style, preProviderFailureLog{
		RequestedModel: virtualModelName,
		RealModelName:  realModelName,
		ProviderModel:  realModelName,
		Error:          errMsg,
	})
}
