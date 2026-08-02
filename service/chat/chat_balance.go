package chat

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/chatcore"
)

func BalanceChat(ctx context.Context, start time.Time, style string, before Before, providersWithMeta ProvidersWithMeta, reqMeta models.ReqMeta) (*http.Response, uint, string, *strings.Builder, error) {
	slog.Info("request", "model", before.Model, "stream", before.Stream, "tool_call", before.toolCall, "structured_output", before.structuredOutput, "image", before.image)

	// 检查是否是虚拟模型
	if providersWithMeta.IsVirtualModel {
		return balanceChatVirtual(ctx, start, style, before, providersWithMeta, reqMeta)
	}

	providerMap := providersWithMeta.ProviderMap
	weightItems := providersWithMeta.WeightItems
	priorityItems := providersWithMeta.PriorityItems

	retryLog := make(chan models.ChatLog, providersWithMeta.MaxRetry)
	defer close(retryLog)
	go RecordRetryLog(context.Background(), retryLog, providersWithMeta.ModelWithProviderMap)

	timer := time.NewTimer(time.Second * time.Duration(providersWithMeta.TimeOut))
	defer timer.Stop()

	for retry := range providersWithMeta.MaxRetry {
		select {
		case <-ctx.Done():
			return nil, 0, "", nil, ctx.Err()
		case <-timer.C:
			return nil, 0, "", nil, errors.New("retry time out")
		default:
		}

		id, err := chatcore.SelectByPriorityAndWeight(weightItems, priorityItems)
		if err != nil {
			return nil, 0, "", nil, err
		}

		modelWithProvider, ok := providersWithMeta.ModelWithProviderMap[*id]
		if !ok {
			delete(weightItems, *id)
			continue
		}

		provider := providerMap[modelWithProvider.ProviderID]
		chatModel, err := providers.New(provider.Type, provider.Config, provider.Proxy)
		if err != nil {
			return nil, 0, "", nil, err
		}

		client := providers.GetClientWithProxy(time.Second*time.Duration(providersWithMeta.TimeOut), chatModel.GetProxy())
		slog.Info("using provider", "provider", provider.Name, "model", modelWithProvider.ProviderModel, "proxy", chatModel.GetProxy())

		result := executeSingleProviderAttempt(singleProviderAttemptInput{
			Ctx:               ctx,
			Start:             start,
			Style:             style,
			Before:            before,
			RealModelName:     before.Model,
			ReqMeta:           reqMeta,
			IOLog:             providersWithMeta.IOLog,
			Retry:             retry,
			Provider:          provider,
			ModelWithProvider: modelWithProvider,
			ChatModel:         chatModel,
			Client:            client,
		}, retryLog)

		if result.FatalErr != nil {
			return nil, 0, "", nil, result.FatalErr
		}
		if result.Success {
			return result.Response, result.LogID, provider.Name, result.RawAccumulator, nil
		}

		applyProviderSelectionResult(weightItems, priorityItems, *id, result)
	}

	return nil, 0, "", nil, errors.New("maximum retry attempts reached")
}
