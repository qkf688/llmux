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
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ProviderRequestCaps 聚合 buildRequestBodyForProvider 的全部请求体改写参数。
// 重构自 6 个位置参数（style/providerType/raw/maxTokensLimit/supportsThinking + thinkingClamp），
// 避免继续横向膨胀。新增改写型能力字段时追加到此结构体，不再加位置参数。
type ProviderRequestCaps struct {
	Style            string                         // 客户端协议格式
	ProviderType     string                         // 上游供应商格式
	Raw              []byte                         // 原始请求体
	MaxTokensLimit   *int                           // max_tokens 上限，nil=不限
	SupportsThinking bool                           // 关联最终是否支持 thinking（SupportsThinkingResolved 结果）
	ThinkingClamp    *transform.ThinkingClampConfig // nil=不钳制（supportsThinking=false 时 thinking 已剥离）
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

// buildRequestBodyForProvider 聚合两条路径（真实/虚拟模型）的全部请求体改写。
// 执行顺序：stripThinkingFields（先剥离不支持 thinking 的）→ passthrough/transform 钳制 → clampMaxTokens。
// 改动时须同步两条路径的测试。
func buildRequestBodyForProvider(ctx context.Context, caps ProviderRequestCaps) ([]byte, bool, error) {
	style := caps.Style
	providerType := caps.ProviderType
	raw := caps.Raw

	// 裁剪 thinking 字段：model/关联不支持 thinking 时去掉请求中的思考配置，
	// 避免上游对不支持 thinking 的模型报 400/静默忽略导致行为不一致。
	// 先于钳制执行：supportsThinking=false 时整体剥离，钳制无意义。
	raw = stripThinkingFields(raw, caps.SupportsThinking)

	if style == providerType {
		slog.Debug("passthrough mode", "client_type", style, "provider_type", providerType)
		// passthrough 路径思考档位钳制（同格式 1×1，对 raw body 按 style 钳制）
		if caps.ThinkingClamp != nil {
			raw = clampPassthroughReasoning(raw, style, caps.ThinkingClamp)
		}
		validated, err := validateAndPatchOutgoingOpenAIRequest(providerType, raw)
		if err != nil {
			return nil, false, err
		}
		clamped, clampErr := clampMaxTokens(validated, caps.MaxTokensLimit)
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
	convertedBody, err := tm.ProcessRequest(ctx, raw, caps.ThinkingClamp)
	if err != nil {
		return nil, false, err
	}
	validated, err := validateAndPatchOutgoingOpenAIRequest(providerType, convertedBody)
	if err != nil {
		return nil, false, err
	}
	clamped, clampErr := clampMaxTokens(validated, caps.MaxTokensLimit)
	if clampErr != nil {
		slog.Warn("max_tokens clamp failed, sending unclamped body", "error", clampErr)
	}
	return clamped, false, nil
}

// stripThinkingFields 在 supportsThinking 为 false 时删除请求体中的 thinking 配置字段
// （OpenAI: reasoning_effort / reasoning；Anthropic: thinking / output_config.effort）。
// 为 true 或非 JSON 时原样返回。失败时记录日志并返回原 body（防御性裁剪，不阻断主流程）。
// 幂等改写函数：用 sjson.DeleteBytes 做字节级删键，保留其余字节原样（键序/数字精度/空白不重排）。
// output_config.effort 删除后若 output_config 变空对象，再删整个 output_config（避免残留空对象）。
// 注意：与 clampMaxTokens 叠加时可能各做一次 JSON 解析，大请求体场景开销可感知，可接受。
func stripThinkingFields(body []byte, supportsThinking bool) []byte {
	if supportsThinking || len(body) == 0 {
		return body
	}
	if !json.Valid(body) {
		return body
	}

	changed := false
	original := body
	for _, field := range []string{"thinking", "reasoning_effort", "reasoning", "output_config.effort"} {
		next, err := sjson.DeleteBytes(body, field)
		if err != nil {
			slog.Warn("strip thinking fields failed, sending original body", "field", field, "error", err)
			return original
		}
		if string(next) != string(body) {
			body = next
			changed = true
		}
	}

	// output_config.effort 删除后，若 output_config 只剩空对象 {}，删掉整个键避免残留。
	if changed {
		if oc := gjson.GetBytes(body, "output_config"); oc.IsObject() && len(oc.Map()) == 0 {
			if next, err := sjson.DeleteBytes(body, "output_config"); err == nil {
				body = next
			}
		}
		slog.Debug("stripped thinking fields from request body (model does not support thinking)")
	}
	return body
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
