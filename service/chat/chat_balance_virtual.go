package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

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

	globalTimer := time.NewTimer(time.Second * time.Duration(pwm.TimeOut))
	defer globalTimer.Stop()

	// 全局超时错误在外层循环与内层重试循环共用同一实例，保证两处返回的语义一致。
	deadlineErr := errors.New("virtual model global timeout")

	for modelIndex, orderedModel := range pwm.OrderedRealModels {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-globalTimer.C:
			return nil, deadlineErr
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
			saveVirtualSkipLog(ctx, pwm.VirtualModelName, realModel.Name, in.Style,
				fmt.Sprintf("virtual model skip: failed to query providers for real model %q: %v", realModel.Name, err))
			continue
		}
		if len(modelWithProviders) == 0 {
			slog.Warn("no providers for real model", "real_model", realModel.Name)
			saveVirtualSkipLog(ctx, pwm.VirtualModelName, realModel.Name, in.Style,
				fmt.Sprintf("virtual model skip: no enabled providers for real model %q", realModel.Name))
			continue
		}

		pool, err := buildCandidatePool(ctx, modelWithProviders)
		if err != nil {
			slog.Error("failed to get providers", "error", err)
			continue
		}

		outcome := runProviderRetryLoop(retryLoopInput{
			Ctx:           ctx,
			Start:         in.Start,
			Style:         in.Style,
			Before:        in.Before,
			ReqMeta:       in.ReqMeta,
			IOLog:         pwm.IOLog,
			Pool:          pool,
			RealModelName: realModel.Name,
			Model:         &realModel,
			MaxRetry:      realModel.MaxRetry,
			ClientTimeout: time.Second * time.Duration(realModel.TimeOut),
			Deadline:      globalTimer.C,
			DeadlineErr:   deadlineErr,
			LogAttrs:      []any{"virtual_model", pwm.VirtualModelName, "real_model", realModel.Name},
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

	return nil, errors.New("all real models exhausted for virtual model")
}

// saveVirtualSkipLog 记录「某个真实模型被整体跳过」的日志：此时尚未选出供应商，
// 故只有模型层面的信息可填。
func saveVirtualSkipLog(ctx context.Context, virtualModelName, realModelName, style, errMsg string) {
	if _, err := SaveChatLog(ctx, models.ChatLog{
		Name:          virtualModelName,
		RealModelName: realModelName,
		ProviderModel: realModelName,
		Status:        "error",
		Style:         style,
		Error:         errMsg,
	}); err != nil {
		slog.Error("failed to save virtual model skip log", "real_model", realModelName, "error", err)
	}
}
