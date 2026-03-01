package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"github.com/atopos31/llmio/service/chatcore"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func balanceChatVirtual(ctx context.Context, start time.Time, style string, before Before, providersWithMeta ProvidersWithMeta, reqMeta models.ReqMeta) (*http.Response, uint, error) {
	slog.Info("virtual model request", "virtual_model", providersWithMeta.VirtualModelName, "strategy", providersWithMeta.VirtualStrategy, "real_models_count", len(providersWithMeta.OrderedRealModels))

	// 全局超时控制
	globalTimer := time.NewTimer(time.Second * time.Duration(providersWithMeta.TimeOut))
	defer globalTimer.Stop()

	// 外层循环：遍历真实模型
	for modelIndex, orderedModel := range providersWithMeta.OrderedRealModels {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-globalTimer.C:
			return nil, 0, errors.New("virtual model global timeout")
		default:
			// 继续处理
		}

		realModel := orderedModel.Model
		slog.Info("trying real model", "virtual_model", providersWithMeta.VirtualModelName, "real_model", realModel.Name, "model_index", modelIndex+1, "total", len(providersWithMeta.OrderedRealModels))

		// 获取当前真实模型的提供商
		modelWithProviderChain := gorm.G[models.ModelWithProvider](models.DB).
			Where("model_id = ? AND status = ?", realModel.ID, true)

		// 检查是否启用严格能力匹配
		strictCapabilityMatch := getStrictCapabilityMatch(ctx)

		if strictCapabilityMatch {
			if before.toolCall {
				modelWithProviderChain = modelWithProviderChain.Where("tool_call = ?", true)
			}

			if before.structuredOutput {
				modelWithProviderChain = modelWithProviderChain.Where("structured_output = ?", true)
			}

			if before.image {
				modelWithProviderChain = modelWithProviderChain.Where("image = ?", true)
			}
		}

		modelWithProviders, err := modelWithProviderChain.Find(ctx)
		if err != nil {
			slog.Error("failed to get providers for real model", "real_model", realModel.Name, "error", err)
			continue // 尝试下一个真实模型
		}

		if len(modelWithProviders) == 0 {
			slog.Warn("no providers for real model", "real_model", realModel.Name)
			continue // 尝试下一个真实模型
		}

		modelWithProviderMap := lo.KeyBy(modelWithProviders, func(mp models.ModelWithProvider) uint { return mp.ID })

		providersList, err := gorm.G[models.Provider](models.DB).
			Where("id IN ?", lo.Map(modelWithProviders, func(mp models.ModelWithProvider, _ int) uint { return mp.ProviderID })).
			Find(ctx)
		if err != nil {
			slog.Error("failed to get providers", "error", err)
			continue
		}

		providerMap := lo.KeyBy(providersList, func(p models.Provider) uint { return p.ID })

		weightItems := make(map[uint]int)
		priorityItems := make(map[uint]int)
		for _, mp := range modelWithProviders {
			if _, ok := providerMap[mp.ProviderID]; !ok {
				continue
			}
			weightItems[mp.ID] = mp.Weight
			priorityItems[mp.ID] = mp.Priority
		}

		// 收集重试过程中的err日志
		retryLog := make(chan models.ChatLog, realModel.MaxRetry)
		go RecordRetryLog(context.Background(), retryLog, modelWithProviderMap)

		// 内层循环：遍历当前真实模型的提供商
		for retry := range realModel.MaxRetry {
			select {
			case <-ctx.Done():
				close(retryLog)
				return nil, 0, ctx.Err()
			case <-globalTimer.C:
				close(retryLog)
				return nil, 0, errors.New("virtual model global timeout")
			default:
				// 继续处理
			}

			// 根据优先级和权重选择供应商
			id, err := chatcore.SelectByPriorityAndWeight(weightItems, priorityItems)
			if err != nil {
				// 当前真实模型的所有提供商都失败了，尝试下一个真实模型
				slog.Warn("all providers failed for real model", "real_model", realModel.Name, "error", err)
				break
			}

			modelWithProvider, ok := modelWithProviderMap[*id]
			if !ok {
				// 数据不一致，移除该模型避免下次重复命中
				delete(weightItems, *id)
				continue
			}

			provider := providerMap[modelWithProvider.ProviderID]

			// 使用供应商的实际类型创建 provider 实例
			chatModel, err := providers.New(provider.Type, provider.Config, provider.Proxy)
			if err != nil {
				slog.Error("failed to create provider", "error", err)
				delete(weightItems, *id)
				continue
			}

			// 为当前provider创建带代理的client
			client := providers.GetClientWithProxy(time.Second*time.Duration(realModel.TimeOut), chatModel.GetProxy())

			slog.Info("using provider", "virtual_model", providersWithMeta.VirtualModelName, "real_model", realModel.Name, "provider", provider.Name, "model", modelWithProvider.ProviderModel, "retry", retry+1)

			log := models.ChatLog{
				Name:          before.Model, // 虚拟模型名称
				ProviderModel: modelWithProvider.ProviderModel,
				ProviderName:  provider.Name,
				Status:        "success",
				Style:         style,
				UserAgent:     reqMeta.UserAgent,
				RemoteIP:      reqMeta.RemoteIP,
				ChatIO:        providersWithMeta.IOLog,
				Retry:         retry,
				ProxyTime:     time.Since(start),
			}

			// 根据请求原始请求头 是否透传请求头 自定义请求头 构建新的请求头
			withHeader := false
			if modelWithProvider.WithHeader != nil {
				withHeader = *modelWithProvider.WithHeader
			}
			header := chatcore.BuildHeaders(reqMeta.Header, withHeader, modelWithProvider.CustomerHeaders, before.Stream)

			reqStart := time.Now()
			// 根据设置决定是否启用请求追踪
			var trace *httptrace.ClientTrace
			var reqCtx context.Context = ctx
			if getEnableRequestTrace(ctx) {
				trace = &httptrace.ClientTrace{
					GotFirstResponseByte: func() {
						slog.Debug("first response byte received", "response_time", time.Since(reqStart))
					},
				}
				reqCtx = httptrace.WithClientTrace(ctx, trace)
			}

			// 判断是否需要格式转换
			var requestBody []byte
			enableFormatConversion := getEnableFormatConversion(ctx)

			if style == provider.Type {
				// 直接透传，不进行格式转换
				slog.Debug("passthrough mode", "client_type", style, "provider_type", provider.Type)
				requestBody = before.raw
			} else if !enableFormatConversion {
				// 格式转换已关闭，跳过此供应商
				slog.Debug("format conversion disabled, skipping provider", "client_type", style, "provider_type", provider.Type)
				delete(weightItems, *id)
				delete(priorityItems, *id)
				continue
			} else {
				// 需要格式转换
				slog.Debug("transform mode", "client_type", style, "provider_type", provider.Type)
				tm := NewTransformerManager(style, provider.Type)
				convertedBody, err := tm.ProcessRequest(ctx, before.raw)
				if err != nil {
					retryLog <- log.WithError(fmt.Errorf("transform request error: %v", err))
					delete(weightItems, *id)
					continue
				}
				requestBody = convertedBody
			}

			// 检查是否启用原始请求响应记录
			logRawOptions := getLogRawRequestResponse(ctx)

			var requestHeadersJSON []byte
			var requestBodyStr string

			if logRawOptions.RequestHeaders || logRawOptions.RequestBody {
				if logRawOptions.RequestHeaders {
					requestHeadersJSON, err = json.Marshal(reqMeta.Header)
					if err != nil {
						slog.Error("failed to marshal request headers", "error", err)
						requestHeadersJSON = []byte("{}")
					}
				}

				if logRawOptions.RequestBody {
					requestBodyStr = string(requestBody)
				}
			}

			req, err := chatModel.BuildReq(reqCtx, header, modelWithProvider.ProviderModel, requestBody)
			if err != nil {
				retryLog <- log.WithError(err)
				delete(weightItems, *id)
				continue
			}

			// 提前创建日志记录
			logId, err := SaveChatLog(ctx, log)
			if err != nil {
				slog.Error("failed to create log before request", "error", err)
				close(retryLog)
				return nil, 0, err
			}

			res, err := client.Do(req)
			if err != nil {
				// 更新日志状态为错误
				if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, models.ChatLog{
					Status: "error",
					Error:  err.Error(),
				}); updateErr != nil {
					slog.Error("failed to update log status", "error", updateErr)
				}
				incrementConsecutiveFailures(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
				applyWeightDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
				applyPriorityDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
				delete(weightItems, *id)
				delete(priorityItems, *id)
				continue
			}

			if res.StatusCode != http.StatusOK {
				byteBody, err := io.ReadAll(res.Body)
				if err != nil {
					slog.Error("read body error", "error", err)
				}

				errorUpdate := models.ChatLog{
					Status: "error",
					Error:  fmt.Sprintf("status: %d, body: %s", res.StatusCode, string(byteBody)),
				}

				if logRawOptions.RequestHeaders || logRawOptions.RequestBody || logRawOptions.ResponseHeaders || logRawOptions.RawResponseBody {
					if logRawOptions.ResponseHeaders {
						responseHeadersJSON, err := json.Marshal(res.Header)
						if err != nil {
							slog.Error("failed to marshal response headers", "error", err)
							responseHeadersJSON = []byte("{}")
						}
						errorUpdate.ResponseHeaders = string(responseHeadersJSON)
					}

					if logRawOptions.RequestHeaders {
						errorUpdate.RequestHeaders = string(requestHeadersJSON)
					}
					if logRawOptions.RequestBody {
						errorUpdate.RequestBody = requestBodyStr
					}
					if logRawOptions.RawResponseBody {
						errorUpdate.RawResponseBody = string(byteBody)
					}
				}

				if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, errorUpdate); updateErr != nil {
					slog.Error("failed to update log status", "error", updateErr)
				}

				incrementConsecutiveFailures(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
				applyWeightDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
				applyPriorityDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)

				if res.StatusCode == http.StatusTooManyRequests {
					weightItems[*id] -= weightItems[*id] / 3
				} else {
					delete(weightItems, *id)
					delete(priorityItems, *id)
				}
				res.Body.Close()
				continue
			}

			// 记录原始响应体（转换前）
			var rawResponseBodyStr string
			if logRawOptions.RawResponseBody {
				bodyBytes, err := io.ReadAll(res.Body)
				if err != nil {
					slog.Error("failed to read raw response body", "error", err)
				} else {
					res.Body.Close()
					rawResponseBodyStr = string(bodyBytes)
					res.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}

			// 判断是否需要响应格式转换
			if style != provider.Type {
				tm := NewTransformerManager(style, provider.Type)
				convertedRes, err := tm.ProcessResponse(res)
				if err != nil {
					if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, models.ChatLog{
						Status: "error",
						Error:  fmt.Sprintf("transform response error: %v", err),
					}); updateErr != nil {
						slog.Error("failed to update log status", "error", updateErr)
					}
					incrementConsecutiveFailures(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
					applyWeightDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
					applyPriorityDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
					res.Body.Close()
					delete(weightItems, *id)
					continue
				}
				res = convertedRes
			} else {
				slog.Debug("passthrough response", "client_type", style, "provider_type", provider.Type)
			}

			// 记录响应头和原始响应体信息
			if logRawOptions.RequestHeaders || logRawOptions.RequestBody || logRawOptions.ResponseHeaders || logRawOptions.RawResponseBody {
				updateData := models.ChatLog{}

				if logRawOptions.ResponseHeaders {
					responseHeadersJSON, err := json.Marshal(res.Header)
					if err != nil {
						slog.Error("failed to marshal response headers", "error", err)
						responseHeadersJSON = []byte("{}")
					}
					updateData.ResponseHeaders = string(responseHeadersJSON)
				}

				if logRawOptions.RequestHeaders {
					updateData.RequestHeaders = string(requestHeadersJSON)
				}
				if logRawOptions.RequestBody {
					updateData.RequestBody = requestBodyStr
				}

				if logRawOptions.RawResponseBody && rawResponseBodyStr != "" {
					updateData.RawResponseBody = rawResponseBodyStr
				}

				if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, updateData); updateErr != nil {
					slog.Error("failed to update log with request/response headers", "error", updateErr)
				}
			}

			resetConsecutiveFailures(ctx, *id)
			applySuccessAdjustments(ctx, *id)

			// 成功：更新 round_robin 索引（如果是 round_robin 策略）
			if providersWithMeta.VirtualStrategy == "round_robin" {
				virtualModelService := NewVirtualModelService(models.DB)
				virtualModelService.UpdateRoundRobinIndex(providersWithMeta.VirtualModelID)
			}

			close(retryLog)
			slog.Info("virtual model request succeeded", "virtual_model", providersWithMeta.VirtualModelName, "real_model", realModel.Name, "provider", provider.Name)
			return res, logId, nil
		}

		// 当前真实模型的所有提供商都失败了，关闭 retryLog 并尝试下一个真实模型
		close(retryLog)
		slog.Warn("all providers exhausted for real model, trying next", "real_model", realModel.Name)
	}

	// 所有真实模型都失败了
	return nil, 0, errors.New("all real models exhausted for virtual model")
}
