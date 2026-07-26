package chat

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
	"strings"
	"time"

	"github.com/atopos31/llmio/consts"
	"github.com/atopos31/llmio/models"
	"github.com/atopos31/llmio/providers"
	"github.com/atopos31/llmio/service/adjustment"
	preprocessopenai "github.com/atopos31/llmio/service/chat/preprocess/openai"
	"github.com/atopos31/llmio/service/chatcore"
	"github.com/atopos31/llmio/service/chatstats"
	"github.com/atopos31/llmio/service/transform"
)

type singleProviderAttemptInput struct {
	Ctx               context.Context
	Start             time.Time
	Style             string
	Before            Before
	RealModelName     string
	ReqMeta           models.ReqMeta
	IOLog             bool
	Retry             int
	Provider          models.Provider
	ModelWithProvider models.ModelWithProvider
	ChatModel         providers.Provider
	Client            *http.Client
}

type singleProviderAttemptResult struct {
	Response       *http.Response
	LogID          uint
	Success        bool
	FatalErr       error
	RemoveWeight   bool
	RemovePriority bool
	ReduceWeight   bool
}

type requestLogSnapshot struct {
	RequestHeadersJSON []byte
	RequestBodyStr     string
}

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

func withOptionalRequestTrace(ctx context.Context) context.Context {
	if !getEnableRequestTrace(ctx) {
		return ctx
	}

	reqStart := time.Now()
	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			slog.Debug("first response byte received", "response_time", time.Since(reqStart))
		},
	}
	return httptrace.WithClientTrace(ctx, trace)
}

func buildRequestBodyForProvider(ctx context.Context, style, providerType string, raw []byte) ([]byte, bool, error) {
	if style == providerType {
		slog.Debug("passthrough mode", "client_type", style, "provider_type", providerType)
		validated, err := validateAndPatchOutgoingOpenAIRequest(providerType, raw)
		return validated, false, err
	}

	if !getEnableFormatConversion(ctx) {
		slog.Debug("format conversion disabled, skipping provider", "client_type", style, "provider_type", providerType)
		return nil, true, nil
	}

	slog.Debug("transform mode", "client_type", style, "provider_type", providerType)
	tm := transform.NewTransformerManager(style, providerType)
	convertedBody, err := tm.ProcessRequest(ctx, raw)
	if err != nil {
		return nil, false, err
	}
	validated, err := validateAndPatchOutgoingOpenAIRequest(providerType, convertedBody)
	return validated, false, err
}

func validateAndPatchOutgoingOpenAIRequest(providerType string, body []byte) ([]byte, error) {
	if providerType != consts.StyleOpenAI {
		return body, nil
	}

	if err := preprocessopenai.ValidateToolCallFunctionNames(body); err != nil {
		return nil, newClientRequestError(http.StatusBadRequest, err.Error())
	}

	if patched, changed, err := preprocessopenai.FillMissingToolCallIDs(body); err == nil && changed {
		body = patched
	}

	if patched, changed, err := preprocessopenai.FillMissingMessageContent(body); err == nil && changed {
		body = patched
	}

	return body, nil
}

func captureRequestLogSnapshot(options models.RawLogOptions, requestHeader http.Header, requestBody []byte) requestLogSnapshot {
	snapshot := requestLogSnapshot{}
	if !options.RequestHeaders && !options.RequestBody {
		return snapshot
	}

	if options.RequestHeaders {
		requestHeadersJSON, err := json.Marshal(requestHeader)
		if err != nil {
			slog.Error("failed to marshal request headers", "error", err)
			requestHeadersJSON = []byte("{}")
		}
		snapshot.RequestHeadersJSON = requestHeadersJSON
	}

	if options.RequestBody {
		snapshot.RequestBodyStr = string(requestBody)
	}

	return snapshot
}

func handleNonOKProviderResponse(
	ctx context.Context,
	res *http.Response,
	logID uint,
	logRawOptions models.RawLogOptions,
	logSnapshot requestLogSnapshot,
	modelWithProvider models.ModelWithProvider,
	provider models.Provider,
) singleProviderAttemptResult {
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
		if logRawOptions.RawResponseBody {
			errorUpdate.RawResponseBody = string(byteBody)
		}
	}

	updateChatLogByID(ctx, logID, errorUpdate, "failed to update log status")
	applyProviderFailureAdjustments(ctx, modelWithProvider.ID, provider.Name, modelWithProvider.ProviderModel)
	if err := chatstats.RecordProviderStats(context.Background(), provider.Name, false, 0, 0); err != nil {
		slog.Warn("failed to record provider stats", "provider", provider.Name, "error", err)
	}
	res.Body.Close()

	if res.StatusCode == http.StatusTooManyRequests {
		return singleProviderAttemptResult{ReduceWeight: true}
	}
	return singleProviderAttemptResult{RemoveWeight: true, RemovePriority: true}
}

func captureRawResponseBody(options models.RawLogOptions, res *http.Response) string {
	if !options.RawResponseBody {
		return ""
	}

	// For SSE streams, avoid reading the entire body here (it would block streaming and buffer unbounded data).
	if strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "text/event-stream") {
		return ""
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("failed to read raw response body", "error", err)
		return ""
	}

	res.Body.Close()
	rawResponseBodyStr := string(bodyBytes)
	res.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return rawResponseBodyStr
}

func updateRequestAndResponseLog(
	ctx context.Context,
	logID uint,
	logRawOptions models.RawLogOptions,
	logSnapshot requestLogSnapshot,
	responseHeader http.Header,
	rawResponseBodyStr string,
) {
	if !logRawOptions.RequestHeaders && !logRawOptions.RequestBody && !logRawOptions.ResponseHeaders && !logRawOptions.RawResponseBody {
		return
	}

	updateData := models.ChatLog{}

	if logRawOptions.ResponseHeaders {
		responseHeadersJSON, err := json.Marshal(responseHeader)
		if err != nil {
			slog.Error("failed to marshal response headers", "error", err)
			responseHeadersJSON = []byte("{}")
		}
		updateData.ResponseHeaders = string(responseHeadersJSON)
	}

	if logRawOptions.RequestHeaders {
		updateData.RequestHeaders = string(logSnapshot.RequestHeadersJSON)
	}
	if logRawOptions.RequestBody {
		updateData.RequestBody = logSnapshot.RequestBodyStr
	}
	if logRawOptions.RawResponseBody && rawResponseBodyStr != "" {
		updateData.RawResponseBody = rawResponseBodyStr
	}

	updateChatLogByID(ctx, logID, updateData, "failed to update log with request/response headers")
}

func applyProviderFailureAdjustments(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	adjustment.IncrementConsecutiveFailures(ctx, modelProviderID, providerName, providerModel)
	adjustment.ApplyWeightDecayByModelProviderID(ctx, modelProviderID, providerName, providerModel)
	adjustment.ApplyPriorityDecayByModelProviderID(ctx, modelProviderID, providerName, providerModel)
}

func updateChatLogByID(ctx context.Context, logID uint, update models.ChatLog, errorMsg string) {
	if _, err := repos().ChatLog.UpdateByID(ctx, logID, update); err != nil {
		slog.Error(errorMsg, "error", err)
	}
}

func applyProviderSelectionResult(weightItems, priorityItems map[uint]int, id uint, result singleProviderAttemptResult) {
	if result.ReduceWeight {
		weightItems[id] -= weightItems[id] / 3
		return
	}
	if result.RemoveWeight {
		delete(weightItems, id)
	}
	if result.RemovePriority {
		delete(priorityItems, id)
	}
}
