package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"github.com/atopos31/llmio/service/chatcore"
	"github.com/atopos31/llmio/service/virtualmodel"
	"github.com/samber/lo"
)

func balanceChatVirtual(ctx context.Context, start time.Time, style string, before Before, providersWithMeta ProvidersWithMeta, reqMeta models.ReqMeta) (*http.Response, uint, string, error) {
	slog.Info("virtual model request", "virtual_model", providersWithMeta.VirtualModelName, "strategy", providersWithMeta.VirtualStrategy, "real_models_count", len(providersWithMeta.OrderedRealModels))

	globalTimer := time.NewTimer(time.Second * time.Duration(providersWithMeta.TimeOut))
	defer globalTimer.Stop()

	for modelIndex, orderedModel := range providersWithMeta.OrderedRealModels {
		select {
		case <-ctx.Done():
			return nil, 0, "", ctx.Err()
		case <-globalTimer.C:
			return nil, 0, "", errors.New("virtual model global timeout")
		default:
		}

		realModel := orderedModel.Model
		slog.Info("trying real model", "virtual_model", providersWithMeta.VirtualModelName, "real_model", realModel.Name, "model_index", modelIndex+1, "total", len(providersWithMeta.OrderedRealModels))

		modelWithProviders, err := queryEnabledModelProviders(ctx, realModel.ID, before)
		if err != nil {
			slog.Error("failed to get providers for real model", "real_model", realModel.Name, "error", err)
			_, _ = SaveChatLog(ctx, models.ChatLog{
				Name:          providersWithMeta.VirtualModelName,
				RealModelName: realModel.Name,
				ProviderModel: realModel.Name,
				Status:        "error",
				Style:         style,
				Error:         fmt.Sprintf("virtual model skip: failed to query providers for real model %q: %v", realModel.Name, err),
			})
			continue
		}
		if len(modelWithProviders) == 0 {
			slog.Warn("no providers for real model", "real_model", realModel.Name)
			_, _ = SaveChatLog(ctx, models.ChatLog{
				Name:          providersWithMeta.VirtualModelName,
				RealModelName: realModel.Name,
				ProviderModel: realModel.Name,
				Status:        "error",
				Style:         style,
				Error:         fmt.Sprintf("virtual model skip: no enabled providers for real model %q", realModel.Name),
			})
			continue
		}

		modelWithProviderMap := lo.KeyBy(modelWithProviders, func(mp models.ModelWithProvider) uint { return mp.ID })

		providerMap, err := buildProviderMapByModelProviders(ctx, modelWithProviders)
		if err != nil {
			slog.Error("failed to get providers", "error", err)
			continue
		}

		weightItems, priorityItems := buildSelectionItemsByModelProviders(modelWithProviders, providerMap)

		retryLog := make(chan models.ChatLog, realModel.MaxRetry)
		go RecordRetryLog(context.Background(), retryLog, modelWithProviderMap)

		for retry := range realModel.MaxRetry {
			select {
			case <-ctx.Done():
				close(retryLog)
				return nil, 0, "", ctx.Err()
			case <-globalTimer.C:
				close(retryLog)
				return nil, 0, "", errors.New("virtual model global timeout")
			default:
			}

			id, err := chatcore.SelectByPriorityAndWeight(weightItems, priorityItems)
			if err != nil {
				slog.Warn("all providers failed for real model", "real_model", realModel.Name, "error", err)
				break
			}

			modelWithProvider, ok := modelWithProviderMap[*id]
			if !ok {
				delete(weightItems, *id)
				continue
			}

			provider := providerMap[modelWithProvider.ProviderID]
			chatModel, err := providers.New(provider.Type, provider.Config, provider.Proxy)
			if err != nil {
				slog.Error("failed to create provider", "error", err)
				delete(weightItems, *id)
				continue
			}

			client := providers.GetClientWithProxy(time.Second*time.Duration(realModel.TimeOut), chatModel.GetProxy())
			slog.Info("using provider", "virtual_model", providersWithMeta.VirtualModelName, "real_model", realModel.Name, "provider", provider.Name, "model", modelWithProvider.ProviderModel, "retry", retry+1)

			result := executeSingleProviderAttempt(singleProviderAttemptInput{
				Ctx:               ctx,
				Start:             start,
				Style:             style,
				Before:            before,
				RealModelName:     realModel.Name,
				ReqMeta:           reqMeta,
				IOLog:             providersWithMeta.IOLog,
				Retry:             retry,
				Provider:          provider,
				ModelWithProvider: modelWithProvider,
				ChatModel:         chatModel,
				Client:            client,
			}, retryLog)

			if result.FatalErr != nil {
				close(retryLog)
				return nil, 0, "", result.FatalErr
			}
			if result.Success {
				sel, _ := virtualmodel.GetSelector(providersWithMeta.VirtualStrategy)
				if sel.RequiresAdvanceOnSuccess() {
					virtualModelService := virtualmodel.NewService(models.DB)
					virtualModelService.UpdateRoundRobinIndex(providersWithMeta.VirtualModelID)
				}
				close(retryLog)
				slog.Info("virtual model request succeeded", "virtual_model", providersWithMeta.VirtualModelName, "real_model", realModel.Name, "provider", provider.Name)
				return result.Response, result.LogID, provider.Name, nil
			}

			applyProviderSelectionResult(weightItems, priorityItems, *id, result)
		}

		close(retryLog)
		slog.Warn("all providers exhausted for real model, trying next", "real_model", realModel.Name)
	}

	return nil, 0, "", errors.New("all real models exhausted for virtual model")
}
