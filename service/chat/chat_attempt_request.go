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
// 重构自 6 个位置参数（style/endpointProtocol/raw/maxTokensLimit/supportsThinking + thinkingClamp），
// 避免继续横向膨胀。新增改写型能力字段时追加到此结构体，不再加位置参数。
type ProviderRequestCaps struct {
	Style            string                         // 客户端协议格式
	EndpointProtocol consts.Protocol                // 选中端点出站协议（透传判定取数点，S3-2 起替代 Provider.Type）
	Raw              []byte                         // 原始请求体
	MaxTokensLimit   *int                           // max_tokens 上限，nil=不限
	SupportsThinking bool                           // 关联最终是否支持 thinking（SupportsThinkingResolved 结果）
	ThinkingClamp    *transform.ThinkingClampConfig // nil=不钳制（supportsThinking=false 时 thinking 已剥离）
	// AllowBudgetExceedMaxTokens 为 true 时跳过 budget >= max_tokens 的协议级收敛。
	// 来源：上游 provider 是否启用了 Anthropic interleaved thinking beta——该 beta 下
	// 「budget 必须小于 max_tokens」的硬约束整体不成立（见 anthropic.BetaInterleavedThinking）。
	AllowBudgetExceedMaxTokens bool
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
// 执行顺序：stripThinkingFields（先剥离不支持 thinking 的）→ passthrough/transform 钳制 → clampMaxTokens
// → reconcileThinkingBudgetWithMaxTokens（收敛 clampMaxTokens 可能造出的 budget >= max_tokens 非法组合）。
// 改动时须同步两条路径的测试。
func buildRequestBodyForProvider(ctx context.Context, caps ProviderRequestCaps) ([]byte, bool, error) {
	style := caps.Style
	raw := caps.Raw

	// 选路依据是**协议形状**，端点协议直接回答「出站是哪一种形状」：openai 客户端打一家
	// OpenAI 兼容的新上游（端点协议同为 openai）时两端形状相同，应走 passthrough，
	// 而不是因 type 字符串不同白跑一趟转换。
	// S3-2 起取数点从 Provider.Type 换成选中端点协议——协议形状与 type 字符串解耦，
	// 多协议端点供应商按请求实际命中的端点判定（定点定协议，透传/转换各走其路）。
	//
	// TODO: caps.Style 仍是裸 string，此处显式转换成 consts.Style。把 Style 端到端改成
	// consts.Style（handler/v1 → chat input → caps）可以省掉这次转换，属独立重构。
	clientFormat, ok := consts.WireFormatOfStyle(consts.Style(style))
	if !ok {
		return nil, false, fmt.Errorf("client style %q has no registered wire format", style)
	}
	upstreamFormat, ok := consts.WireFormatOfProtocol(caps.EndpointProtocol)
	if !ok {
		return nil, false, fmt.Errorf("endpoint protocol %q has no wire format", caps.EndpointProtocol)
	}

	// 裁剪 thinking 字段：model/关联不支持 thinking 时去掉请求中的思考配置，
	// 避免上游对不支持 thinking 的模型报 400/静默忽略导致行为不一致。
	// 先于钳制执行：supportsThinking=false 时整体剥离，钳制无意义。
	raw = stripThinkingFields(raw, caps.SupportsThinking)

	if clientFormat == upstreamFormat {
		slog.Debug("passthrough mode", "client_format", clientFormat, "upstream_format", upstreamFormat, "endpoint_protocol", caps.EndpointProtocol)
		// passthrough 路径思考档位钳制（同格式 1×1，对 raw body 按协议形状钳制）
		if caps.ThinkingClamp != nil {
			raw = clampPassthroughReasoning(raw, clientFormat, caps.ThinkingClamp)
		}
		validated, err := validateAndPatchOutgoingOpenAIRequest(upstreamFormat, raw)
		if err != nil {
			return nil, false, err
		}
		clamped, clampErr := clampMaxTokens(validated, caps.MaxTokensLimit)
		if clampErr != nil {
			slog.Warn("max_tokens clamp failed, sending unclamped body", "error", clampErr)
		}
		return reconcileThinkingBudgetWithMaxTokens(clamped, upstreamFormat, caps.AllowBudgetExceedMaxTokens), false, nil
	}

	if !getEnableFormatConversion(ctx) {
		slog.Debug("format conversion disabled, skipping provider", "client_format", clientFormat, "upstream_format", upstreamFormat)
		return nil, true, nil
	}

	slog.Debug("transform mode", "client_format", clientFormat, "upstream_format", upstreamFormat)
	tm := transform.NewTransformerManager(clientFormat, upstreamFormat)
	convertedBody, err := tm.ProcessRequest(ctx, raw, caps.ThinkingClamp)
	if err != nil {
		return nil, false, err
	}
	validated, err := validateAndPatchOutgoingOpenAIRequest(upstreamFormat, convertedBody)
	if err != nil {
		return nil, false, err
	}
	clamped, clampErr := clampMaxTokens(validated, caps.MaxTokensLimit)
	if clampErr != nil {
		slog.Warn("max_tokens clamp failed, sending unclamped body", "error", clampErr)
	}
	return reconcileThinkingBudgetWithMaxTokens(clamped, upstreamFormat, caps.AllowBudgetExceedMaxTokens), false, nil
}

// stripThinkingFields 在 supportsThinking 为 false 时删除请求体中的 thinking 配置字段
// （OpenAI: reasoning_effort / reasoning；Anthropic: thinking / output_config.effort），
// 并裁剪 messages 数组内 assistant 轮 content 里内嵌的 type=thinking /
// type=redacted_thinking 块（Anthropic 扩展思考多轮会话回灌场景，由
// service/anthropic/request_outbound.go 的 buildThinkingBlock/buildRedactedThinkingBlock
// 生成/透传；路由到不支持 thinking 的上游时同样触发 400）。
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
	}

	// 裁剪 messages 内嵌的 thinking 块（Anthropic 扩展思考多轮回灌场景）。
	var msgChanged bool
	body, msgChanged = stripThinkingBlocksFromMessages(body)
	if msgChanged {
		changed = true
	}

	if changed {
		slog.Debug("stripped thinking fields from request body (model does not support thinking)")
	}
	return body
}

// stripThinkingBlocksFromMessages 裁剪 messages 数组内 assistant 轮 content 里
// type=thinking / type=redacted_thinking 的块。
// 用 gjson 取快照索引、sjson.DeleteBytes 从后往前删，保持其余字节原样（不重排/不重序列化）。
// 从后往前删保证快照索引与当前 body 索引一致（删后面的不影响前面的索引）。
// 注意：若某条消息的 content 删空后只剩空数组，不在此处理——实际多轮会话中
// assistant 轮通常还带 text/tool_use 块，thinking-only 轮极罕见，留待后续按需补。
func stripThinkingBlocksFromMessages(body []byte) ([]byte, bool) {
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return body, false
	}

	changed := false
	msgs := messages.Array()
	for i := len(msgs) - 1; i >= 0; i-- {
		contentPath := fmt.Sprintf("messages.%d.content", i)
		content := gjson.GetBytes(body, contentPath)
		if !content.IsArray() {
			continue
		}

		parts := content.Array()
		for j := len(parts) - 1; j >= 0; j-- {
			t := parts[j].Get("type").String()
			if t != "thinking" && t != "redacted_thinking" {
				continue
			}
			elemPath := fmt.Sprintf("%s.%d", contentPath, j)
			next, err := sjson.DeleteBytes(body, elemPath)
			if err != nil {
				slog.Warn("strip thinking block from message failed", "path", elemPath, "error", err)
				return body, changed
			}
			body = next
			changed = true
		}
	}

	return body, changed
}

// validateAndPatchOutgoingOpenAIRequest 只对**出站 body 是 OpenAI Chat 形状**时生效：
// 校验/补齐的都是 OpenAI Chat 特有字段（tool_call 函数名、tool_call_id、message.content），
// 判定依据必须是 wire format 而非「上游是哪一家」——OpenAI 兼容的新上游同样需要这层补齐。
func validateAndPatchOutgoingOpenAIRequest(upstreamFormat consts.WireFormat, body []byte) ([]byte, error) {
	if upstreamFormat != consts.FormatOpenAIChat {
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

// clampMaxTokens 将请求体中的输出上限字段裁剪到 limit。limit 为 nil 或 <=0 表示不限，
// 直接返回原 body。避免客户端发超大值触发上游 400（如 Cursor 默认填 1M）。
// 返回 error 供调用方感知失败；调用方可选择打日志后继续用原 body（兜底不阻断主流程）。
//
// 字段表按**协议实际键名**穷举而非只认 max_tokens：同一个「输出上限」语义在三个协议
// 下有三个键名，漏一个该协议的运维阀值就是空转——`max_output_tokens` 曾漏在表外，
// openai-res 上游的 MaxTokensLimit 完全不生效且无任何报错。
func clampMaxTokens(body []byte, limit *int) ([]byte, error) {
	if limit == nil || *limit <= 0 {
		return body, nil
	}
	maxLimit := *limit

	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return body, fmt.Errorf("unmarshal body for output limit clamp: %w", err)
	}

	changed := false
	// OpenAI Chat 旧名 max_tokens / 新名 max_completion_tokens；Anthropic 用 max_tokens；
	// OpenAI Responses 用 max_output_tokens。逐个键独立钳制——同一 body 里出现多个
	// （客户端同时传旧新名）时都得压住，只认第一个会留下没钳到的那个。
	for _, field := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens"} {
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
		return body, fmt.Errorf("marshal body after output limit clamp: %w", err)
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
