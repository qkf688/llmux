package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/atopos31/llmio/models"
)

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

func updateChatLogByID(ctx context.Context, logID uint, update models.ChatLog, errorMsg string) {
	if _, err := repos().ChatLog.UpdateByID(ctx, logID, update); err != nil {
		slog.Error(errorMsg, "error", err)
	}
}
