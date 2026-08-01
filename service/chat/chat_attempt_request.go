package chat

import (
	"context"
	"encoding/json"
	"fmt"
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

func buildRequestBodyForProvider(ctx context.Context, style, providerType string, raw []byte, maxTokensLimit *int) ([]byte, bool, error) {
	if style == providerType {
		slog.Debug("passthrough mode", "client_type", style, "provider_type", providerType)
		validated, err := validateAndPatchOutgoingOpenAIRequest(providerType, raw)
		if err != nil {
			return nil, false, err
		}
		clamped, clampErr := clampMaxTokens(validated, maxTokensLimit)
		if clampErr != nil {
			slog.Warn("max_tokens clamp failed, sending unclamped body", "error", clampErr)
		}
		return clamped, false, nil
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
	if err != nil {
		return nil, false, err
	}
	clamped, clampErr := clampMaxTokens(validated, maxTokensLimit)
	if clampErr != nil {
		slog.Warn("max_tokens clamp failed, sending unclamped body", "error", clampErr)
	}
	return clamped, false, nil
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

// clampMaxTokens 将请求体中的 max_tokens（及 OpenAI 新名 max_completion_tokens）
// 裁剪到 limit。limit 为 nil 或 <=0 表示不限，直接返回原 body。
// 避免客户端发超大 max_tokens 触发上游 400（如 Cursor 默认填 1M）。
// 返回 error 供调用方感知失败；调用方可选择打日志后继续用原 body（兜底不阻断主流程）。
func clampMaxTokens(body []byte, limit *int) ([]byte, error) {
	if limit == nil || *limit <= 0 {
		return body, nil
	}
	maxLimit := *limit

	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return body, fmt.Errorf("unmarshal body for max_tokens clamp: %w", err)
	}

	changed := false
	// OpenAI Chat API 旧名 max_tokens / 新名 max_completion_tokens，Anthropic 用 max_tokens
	for _, field := range []string{"max_tokens", "max_completion_tokens"} {
		v, ok := obj[field]
		if !ok {
			continue
		}
		current, ok := toInt(v)
		if !ok {
			continue
		}
		if current > maxLimit {
			obj[field] = maxLimit
			changed = true
		}
	}

	if !changed {
		return body, nil
	}
	patched, err := json.Marshal(obj)
	if err != nil {
		return body, fmt.Errorf("marshal body after max_tokens clamp: %w", err)
	}
	return patched, nil
}

// toInt 把 JSON 解析出的 number 转成 int。JSON 数字默认解析为 float64。
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	}
	return 0, false
}
