package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/adjustment"
	"github.com/qkf688/llmux/service/chatcore"
	"github.com/qkf688/llmux/service/chatstats"
	"github.com/qkf688/llmux/service/transform"
)

func executeSingleProviderAttempt(input singleProviderAttemptInput, retryLog chan<- models.ChatLog) singleProviderAttemptResult {
	logEntry := models.ChatLog{
		Name:          input.Before.Model,
		RealModelName: input.RealModelName,
		ProviderModel: input.ModelWithProvider.ProviderModel,
		ProviderName:  input.Provider.Name,
		Status:        "success",
		Style:         input.Style,
		UserAgent:     input.ReqMeta.UserAgent,
		RemoteIP:      input.ReqMeta.RemoteIP,
		ChatIO:        input.IOLog,
		Retry:         input.Retry,
		ProxyTime:     time.Since(input.Start),
	}

	withHeader := false
	if input.ModelWithProvider.WithHeader != nil {
		withHeader = *input.ModelWithProvider.WithHeader
	}
	header := chatcore.BuildHeaders(input.ReqMeta.Header, withHeader, input.ModelWithProvider.CustomerHeaders, input.Before.Stream)

	reqCtx := withOptionalRequestTrace(input.Ctx)

	// 构建思考档位钳制配置：supportsThinking=false 时 ThinkingClamp 为 nil
	// （stripThinkingFields 已整体剥离 thinking，钳制无意义）。
	var thinkingClamp *transform.ThinkingClampConfig
	supportsThinking := input.ModelWithProvider.SupportsThinkingResolved(input.Model)
	if supportsThinking {
		thinkingClamp = buildThinkingClampConfig(input.Ctx, input.Model, &input.ModelWithProvider)
	}

	requestBody, skipProvider, bodyErr := buildRequestBodyForProvider(input.Ctx, ProviderRequestCaps{
		Style:                      input.Style,
		ProviderType:               input.Provider.Type,
		Raw:                        input.Before.raw,
		MaxTokensLimit:             input.ModelWithProvider.MaxTokens,
		SupportsThinking:           supportsThinking,
		ThinkingClamp:              thinkingClamp,
		AllowBudgetExceedMaxTokens: providerAllowsBudgetExceedMaxTokens(input.ChatModel),
	})
	if skipProvider {
		return singleProviderAttemptResult{RemoveWeight: true, RemovePriority: true}
	}
	if bodyErr != nil {
		var statusCoder interface{ StatusCode() int }
		if errors.As(bodyErr, &statusCoder) {
			return singleProviderAttemptResult{FatalErr: bodyErr}
		}
		retryLog <- logEntry.WithError(fmt.Errorf("transform request error: %v", bodyErr))
		if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
			slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
		}
		return singleProviderAttemptResult{RemoveWeight: true}
	}

	logRawOptions := getLogRawRequestResponse(input.Ctx)
	logSnapshot := captureRequestLogSnapshot(logRawOptions, input.ReqMeta.Header, requestBody, input.Before.raw)

	req, err := input.ChatModel.BuildReq(reqCtx, header, input.ModelWithProvider.ProviderModel, requestBody)
	if err != nil {
		retryLog <- logEntry.WithError(err)
		if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
			slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
		}
		return singleProviderAttemptResult{RemoveWeight: true}
	}

	logID, err := SaveChatLog(input.Ctx, logEntry)
	if err != nil {
		slog.Error("failed to create log before request", "error", err)
		return singleProviderAttemptResult{FatalErr: err}
	}

	res, err := input.Client.Do(req)
	if err != nil {
		errorUpdate := models.ChatLog{
			Status: "error",
			Error:  err.Error(),
		}
		if logRawOptions.RequestHeaders {
			errorUpdate.RequestHeaders = string(logSnapshot.RequestHeadersJSON)
		}
		if logRawOptions.RequestBody {
			errorUpdate.RequestBody = logSnapshot.RequestBodyStr
		}
		if logRawOptions.RawRequestBody {
			errorUpdate.RawRequestBody = logSnapshot.RawRequestBodyStr
		}
		updateChatLogByID(input.Ctx, logID, errorUpdate, "failed to update log status")
		applyProviderFailureAdjustments(input.Ctx, input.ModelWithProvider.ID, input.Provider.Name, input.ModelWithProvider.ProviderModel)
		if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
			slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
		}
		return singleProviderAttemptResult{RemoveWeight: true, RemovePriority: true}
	}

	if res.StatusCode != http.StatusOK {
		return handleNonOKProviderResponse(input.Ctx, res, logID, logRawOptions, logSnapshot, input.ModelWithProvider, input.Provider)
	}

	rawResponseBodyStr := captureRawResponseBody(logRawOptions, res)

	// 流式响应的原始 body 不能在入口读取（会阻塞流），改用侧信道在转换 goroutine 内旁路记录。
	// 侧信道同时承载**上游原始 usage**：转换后的下游流受目标协议表达能力限制，
	// 从它反解 usage 必然有损，故由转换层在解析上游时旁路交出真值（见 models.TransformSideChannel）。
	// 仅在需要转换时创建——直通路径没有转换层可供旁路，落库回退 processer 解析（该路径本就完整）。
	captureRawBody := logRawOptions.RawResponseBody && rawResponseBodyStr == ""

	// 响应侧的直通判定必须与请求侧（buildRequestBodyForProvider）用同一依据：**协议形状**。
	// 按 provider type 字符串比较会让「openai 客户端打一家 OpenAI 兼容的新上游」请求直通、
	// 响应却白跑一趟转换。任一侧形状解析不出来时按「形状不同」走转换路径，由转换层报错
	// （请求侧已先解析过并在失败时返回错误，走到这里两侧本应都解析成功）。
	clientFormat, clientFormatOK := consts.WireFormatOfStyle(consts.Style(input.Style))
	upstreamFormat, upstreamFormatOK := providers.WireFormatOf(input.Provider.Type)

	var sideChannel *models.TransformSideChannel
	if !clientFormatOK || !upstreamFormatOK || clientFormat != upstreamFormat {
		sideChannel = models.NewTransformSideChannel(captureRawBody)
		tm := transform.NewTransformerManager(clientFormat, upstreamFormat)
		convertedRes, err := tm.ProcessResponse(res, sideChannel)
		if err != nil {
			errorUpdate := models.ChatLog{
				Status: "error",
				Error:  fmt.Sprintf("transform response error: %v", err),
			}
			if logRawOptions.ResponseHeaders {
				responseHeadersJSON, marshalErr := json.Marshal(res.Header)
				if marshalErr != nil {
					slog.Error("failed to marshal response headers", "error", marshalErr)
					responseHeadersJSON = []byte("{}")
				}
				errorUpdate.ResponseHeaders = string(responseHeadersJSON)
			}
			if logRawOptions.RequestHeaders {
				errorUpdate.RequestHeaders = string(logSnapshot.RequestHeadersJSON)
			}
			if logRawOptions.RequestBody {
				errorUpdate.RequestBody = logSnapshot.RequestBodyStr
			}
			if logRawOptions.RawRequestBody {
				errorUpdate.RawRequestBody = logSnapshot.RawRequestBodyStr
			}
			if logRawOptions.RawResponseBody && rawResponseBodyStr != "" {
				errorUpdate.RawResponseBody = rawResponseBodyStr
			}
			updateChatLogByID(input.Ctx, logID, errorUpdate, "failed to update log status")
			applyProviderFailureAdjustments(input.Ctx, input.ModelWithProvider.ID, input.Provider.Name, input.ModelWithProvider.ProviderModel)
			if err := chatstats.RecordProviderStats(context.Background(), input.Provider.Name, false, 0, 0); err != nil {
				slog.Warn("failed to record provider stats", "provider", input.Provider.Name, "error", err)
			}
			res.Body.Close()
			return singleProviderAttemptResult{RemoveWeight: true}
		}
		res = convertedRes
	} else {
		slog.Debug("passthrough response", "client_format", clientFormat, "upstream_format", upstreamFormat, "provider_type", input.Provider.Type)
	}

	updateRequestAndResponseLog(input.Ctx, logID, logRawOptions, logSnapshot, res.Header, rawResponseBodyStr)

	adjustment.ResetConsecutiveFailures(input.Ctx, input.ModelWithProvider.ID)
	adjustment.ApplySuccessAdjustments(input.Ctx, input.ModelWithProvider.ID)
	return singleProviderAttemptResult{
		Response:    res,
		LogID:       logID,
		Success:     true,
		SideChannel: sideChannel,
	}
}
