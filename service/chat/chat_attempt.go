package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/service/adjustment"
	"github.com/atopos31/llmio/service/chatcore"
	"github.com/atopos31/llmio/service/chatstats"
	"github.com/atopos31/llmio/service/transform"
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

	requestBody, skipProvider, bodyErr := buildRequestBodyForProvider(input.Ctx, input.Style, input.Provider.Type, input.Before.raw)
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
	logSnapshot := captureRequestLogSnapshot(logRawOptions, input.ReqMeta.Header, requestBody)

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

	if input.Style != input.Provider.Type {
		tm := transform.NewTransformerManager(input.Style, input.Provider.Type)
		convertedRes, err := tm.ProcessResponse(res)
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
		slog.Debug("passthrough response", "client_type", input.Style, "provider_type", input.Provider.Type)
	}

	updateRequestAndResponseLog(input.Ctx, logID, logRawOptions, logSnapshot, res.Header, rawResponseBodyStr)

	adjustment.ResetConsecutiveFailures(input.Ctx, input.ModelWithProvider.ID)
	adjustment.ApplySuccessAdjustments(input.Ctx, input.ModelWithProvider.ID)
	return singleProviderAttemptResult{
		Response: res,
		LogID:    logID,
		Success:  true,
	}
}
