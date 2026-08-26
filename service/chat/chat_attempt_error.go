package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/adjustment"
	"github.com/qkf688/llmux/service/chatstats"
)

func handleNonOKProviderResponse(
	ctx context.Context,
	res *http.Response,
	logID uint,
	logRawOptions models.RawLogOptions,
	logSnapshot requestLogSnapshot,
	modelWithProvider models.ModelWithProvider,
	provider models.Provider,
	start time.Time, // 请求开始时刻（handler startReq）：用于回填 ProxyTime 终值
) singleProviderAttemptResult {
	byteBody, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("read body error", "error", err)
	}

	errorUpdate := models.ChatLog{
		Status: "error",
		Error:  fmt.Sprintf("status: %d, body: %s", res.StatusCode, string(byteBody)),
		// 回填端到端耗时：上游已返回完整错误响应，建行快照不含上游耗时。
		ProxyTime: time.Since(start),
	}

	if logRawOptions.RequestHeaders || logRawOptions.RequestBody || logRawOptions.RawRequestBody || logRawOptions.ResponseHeaders || logRawOptions.RawResponseBody {
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

func applyProviderFailureAdjustments(ctx context.Context, modelProviderID uint, providerName, providerModel string) {
	adjustment.IncrementConsecutiveFailures(ctx, modelProviderID, providerName, providerModel)
	adjustment.ApplyWeightDecayByModelProviderID(ctx, modelProviderID, providerName, providerModel)
	adjustment.ApplyPriorityDecayByModelProviderID(ctx, modelProviderID, providerName, providerModel)
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
