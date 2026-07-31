package chat

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/qkf688/llmux/consts"
	preprocessopenai "github.com/qkf688/llmux/service/chat/preprocess/openai"
	"github.com/qkf688/llmux/service/transform"
)

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
