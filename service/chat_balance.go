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
	"gorm.io/gorm"
)

func BalanceChat(ctx context.Context, start time.Time, style string, before Before, providersWithMeta ProvidersWithMeta, reqMeta models.ReqMeta) (*http.Response, uint, error) {
	slog.Info("request", "model", before.Model, "stream", before.Stream, "tool_call", before.toolCall, "structured_output", before.structuredOutput, "image", before.image)

	// 检查是否是虚拟模型
	if providersWithMeta.IsVirtualModel {
		return balanceChatVirtual(ctx, start, style, before, providersWithMeta, reqMeta)
	}

	// 非虚拟模型：使用原有逻辑
	providerMap := providersWithMeta.ProviderMap
	weightItems := providersWithMeta.WeightItems
	priorityItems := providersWithMeta.PriorityItems

	// 收集重试过程中的err日志
	retryLog := make(chan models.ChatLog, providersWithMeta.MaxRetry)
	defer close(retryLog)

	go RecordRetryLog(context.Background(), retryLog, providersWithMeta.ModelWithProviderMap)

	// 注意：这里我们需要在循环中为每个provider创建带代理的client
	// 所以先移除这行，在循环内部创建

	timer := time.NewTimer(time.Second * time.Duration(providersWithMeta.TimeOut))
	defer timer.Stop()
	for retry := range providersWithMeta.MaxRetry {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-timer.C:
			return nil, 0, errors.New("retry time out")
		default:
			// 根据优先级和权重选择供应商
			id, err := chatcore.SelectByPriorityAndWeight(weightItems, priorityItems)
			if err != nil {
				return nil, 0, err
			}

			modelWithProvider, ok := providersWithMeta.ModelWithProviderMap[*id]
			if !ok {
				// 数据不一致，移除该模型避免下次重复命中
				delete(weightItems, *id)
				continue
			}

			provider := providerMap[modelWithProvider.ProviderID]

			// 使用供应商的实际类型创建 provider 实例，而不是客户端格式
			chatModel, err := providers.New(provider.Type, provider.Config, provider.Proxy)
			if err != nil {
				return nil, 0, err
			}

			// 为当前provider创建带代理的client
			// 使用完整的超时时间,特别是对于工具调用场景需要更长的等待时间
			client := providers.GetClientWithProxy(time.Second*time.Duration(providersWithMeta.TimeOut), chatModel.GetProxy())

			slog.Info("using provider", "provider", provider.Name, "model", modelWithProvider.ProviderModel, "proxy", chatModel.GetProxy())

			log := models.ChatLog{
				Name:          before.Model,
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
			// 当客户端格式与供应商类型一致时，直接透传原始请求体
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
				// 记录原始请求头信息（客户端发送的完整头部）
				if logRawOptions.RequestHeaders {
					requestHeadersJSON, err = json.Marshal(reqMeta.Header)
					if err != nil {
						slog.Error("failed to marshal request headers", "error", err)
						requestHeadersJSON = []byte("{}")
					}
				}

				// 记录完整的请求体，不做大小限制
				if logRawOptions.RequestBody {
					requestBodyStr = string(requestBody)
				}
			}

			req, err := chatModel.BuildReq(reqCtx, header, modelWithProvider.ProviderModel, requestBody)
			if err != nil {
				retryLog <- log.WithError(err)
				// 构建请求失败 移除待选
				delete(weightItems, *id)
				continue
			}

			// 提前创建日志记录,确保所有请求都被记录
			logId, err := SaveChatLog(ctx, log)
			if err != nil {
				slog.Error("failed to create log before request", "error", err)
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
				// 应用权重和优先级衰减
				applyWeightDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
				applyPriorityDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
				// 请求失败 移除待选
				delete(weightItems, *id)
				delete(priorityItems, *id)
				continue
			}

			if res.StatusCode != http.StatusOK {
				byteBody, err := io.ReadAll(res.Body)
				if err != nil {
					slog.Error("read body error", "error", err)
				}

				// 准备错误日志更新数据
				errorUpdate := models.ChatLog{
					Status: "error",
					Error:  fmt.Sprintf("status: %d, body: %s", res.StatusCode, string(byteBody)),
				}

				// 如果启用了日志记录，也记录请求和响应信息
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

				// 更新日志状态为错误
				if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, errorUpdate); updateErr != nil {
					slog.Error("failed to update log status", "error", updateErr)
				}

				incrementConsecutiveFailures(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
				// 应用权重和优先级衰减
				applyWeightDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
				applyPriorityDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)

				if res.StatusCode == http.StatusTooManyRequests {
					// 达到RPM限制 降低权重
					weightItems[*id] -= weightItems[*id] / 3
				} else {
					// 非RPM限制 移除待选
					delete(weightItems, *id)
					delete(priorityItems, *id)
				}
				res.Body.Close()
				continue
			}

			// 记录原始响应体（转换前）
			var rawResponseBodyStr string
			if logRawOptions.RawResponseBody {
				// 读取原始响应体
				bodyBytes, err := io.ReadAll(res.Body)
				if err != nil {
					slog.Error("failed to read raw response body", "error", err)
				} else {
					res.Body.Close()
					// 保存完整的原始响应体，不做大小限制
					rawResponseBodyStr = string(bodyBytes)
					// 重新创建响应体供后续使用
					res.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}

			// 判断是否需要响应格式转换
			// 当客户端格式与供应商类型一致时，直接透传响应
			if style != provider.Type {
				// 需要格式转换
				tm := NewTransformerManager(style, provider.Type)
				convertedRes, err := tm.ProcessResponse(res)
				if err != nil {
					// 更新日志状态为错误
					if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, models.ChatLog{
						Status: "error",
						Error:  fmt.Sprintf("transform response error: %v", err),
					}); updateErr != nil {
						slog.Error("failed to update log status", "error", updateErr)
					}
					incrementConsecutiveFailures(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
					// 应用权重和优先级衰减
					applyWeightDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
					applyPriorityDecayByModelProviderID(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
					res.Body.Close()
					delete(weightItems, *id)
					continue
				}
				res = convertedRes
			} else {
				// 直接透传响应，不进行格式转换
				slog.Debug("passthrough response", "client_type", style, "provider_type", provider.Type)
			}

			// 记录响应头和原始响应体信息（仅在启用时）
			if logRawOptions.RequestHeaders || logRawOptions.RequestBody || logRawOptions.ResponseHeaders || logRawOptions.RawResponseBody {
				// 更新日志记录原始请求响应内容
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

				// 如果需要格式转换，RawResponseBody 存储转换前的内容
				// 如果不需要格式转换，RawResponseBody 存储原始内容（与 ResponseBody 相同）
				if logRawOptions.RawResponseBody && rawResponseBodyStr != "" {
					updateData.RawResponseBody = rawResponseBodyStr
				}

				if _, updateErr := gorm.G[models.ChatLog](models.DB).Where("id = ?", logId).Updates(ctx, updateData); updateErr != nil {
					slog.Error("failed to update log with request/response headers", "error", updateErr)
				}
			}

			resetConsecutiveFailures(ctx, *id)
			applySuccessAdjustments(ctx, *id)
			return res, logId, nil
		}
	}

	return nil, 0, errors.New("maximum retry attempts reached")
}
