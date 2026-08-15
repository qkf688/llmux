package streaming

import (
	"github.com/qkf688/llmux/common/maputil"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/responses"
)

// 本文件负责把**上游原始 usage** 旁路交给落库侧（models.TransformSideChannel）。
//
// 为什么不复用转换后的 usage：转换输出受目标协议表达能力限制（Anthropic 无
// reasoning token 槽位、message_start 必须早于真实 prompt_tokens 发出），
// 从下游流反解必然有损。故在**读上游那一跳**就地捕获。
//
// 组织原则：usage 的形状只取决于上游协议（openai / openai-res / anthropic）三种，
// 与下游 client 协议无关。因此这里按上游协议各写一个映射函数，由对应的第一跳
// handler 调用；不按「方向」铺开，避免 N×M 份重复解析（DRY）。
//
// 只有持有 sideChannel 的那一跳会真正写入：多跳转换里第二跳的 sideChannel 为 nil，
// 这些函数退化为 no-op，不会用中间格式污染上游真值。

// usageFieldCandidates 描述一个 token 字段在上游 JSON 里的候选取值路径，按优先级排列。
//
// 为什么需要候选列表：兼容供应商各自的非标准写法（如把 reasoning_tokens 放在
// usage 顶层、用 prompt_cache_hit_tokens 表示缓存命中）。用有序候选表而不是
// per-provider if-else，新增一种写法只加一行数据，不改控制流（OCP）。
type usageFieldCandidates [][]string

var (
	promptTokenPaths = usageFieldCandidates{
		{"prompt_tokens"},
		{"input_tokens"},
	}
	completionTokenPaths = usageFieldCandidates{
		{"completion_tokens"},
		{"output_tokens"},
	}
	totalTokenPaths = usageFieldCandidates{
		{"total_tokens"},
	}
	cachedTokenPaths = usageFieldCandidates{
		{"prompt_tokens_details", "cached_tokens"},
		{"input_tokens_details", "cached_tokens"},
		{"cached_tokens"},
		// DeepSeek 系列的写法：命中数放在 usage 顶层且键名不同。
		{"prompt_cache_hit_tokens"},
		// Anthropic Messages：缓存读取命中即统一模型的 cached_tokens。
		{"cache_read_input_tokens"},
	}
	reasoningTokenPaths = usageFieldCandidates{
		{"completion_tokens_details", "reasoning_tokens"},
		{"output_tokens_details", "reasoning_tokens"},
		// 部分 OpenAI 兼容供应商把 reasoning 记在 usage 顶层。
		{"reasoning_tokens"},
	}
)

// pickUsageField 按候选顺序取第一个 > 0 的值；全部缺失或为 0 时返回 0。
//
// 纯函数：不读全局状态、不改入参，便于表驱动测试。
func pickUsageField(usage map[string]interface{}, candidates usageFieldCandidates) int64 {
	for _, path := range candidates {
		node := usage
		for i, key := range path {
			if i == len(path)-1 {
				if v := maputil.Float64(node, key); v > 0 {
					return int64(v)
				}
				break
			}
			next, ok := node[key].(map[string]interface{})
			if !ok {
				break
			}
			node = next
		}
	}
	return 0
}

// usageFromUpstreamMap 把任意协议的 usage map 归一为 models.Usage。
//
// total_tokens 缺失时回退为 prompt+completion（与 service/chat/process.go 的
// 落库口径一致：不把 Anthropic 的 cache token 额外计入总数）。
func usageFromUpstreamMap(usage map[string]interface{}) models.Usage {
	u := models.Usage{
		PromptTokens:     pickUsageField(usage, promptTokenPaths),
		CompletionTokens: pickUsageField(usage, completionTokenPaths),
		TotalTokens:      pickUsageField(usage, totalTokenPaths),
	}
	if u.TotalTokens == 0 {
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}
	u.PromptTokensDetails.CachedTokens = pickUsageField(usage, cachedTokenPaths)
	u.CompletionTokensDetails.ReasoningTokens = pickUsageField(usage, reasoningTokenPaths)
	return u
}

// captureUpstreamUsageMap 在本跳持有侧信道时，记录上游原始 usage。
//
// 允许对同一条流多次调用（Anthropic 的 usage 拆在 message_start 与 message_delta
// 两处，两处都要交），但**后一次传入的快照必须是前一次的超集**——侧信道语义是
// 「最后一次有效观测胜出」，传一个只带 output 侧的快照会把先前的 input 侧抹成 0。
//
// 全零 / 无法识别的快照由 SetUpstreamUsage 内部丢弃，此处不再重复判断。
func captureUpstreamUsageMap(state *realtimeStreamState, usage map[string]interface{}) {
	if state.sideChannel == nil || len(usage) == 0 {
		return
	}
	state.sideChannel.SetUpstreamUsage(usageFromUpstreamMap(usage))
}

// captureUpstreamUsageResponses 记录 openai-res 上游的原始 usage。
//
// 当上游本身就是 openai-res（provider=openai-res 的单跳转换）时，usage 已是
// 结构化的 ResponsesUsage；转成 map 走同一条归一 / 写入路径，避免另开一份逻辑。
// 多跳转换里第二跳（openai-res → client）的 sideChannel 为 nil，此处退化为 no-op。
func captureUpstreamUsageResponses(state *realtimeStreamState, usage *responses.ResponsesUsage) {
	if state.sideChannel == nil || usage == nil {
		return
	}
	m := map[string]interface{}{
		"input_tokens":  float64(usage.InputTokens),
		"output_tokens": float64(usage.OutputTokens),
		"total_tokens":  float64(usage.TotalTokens),
	}
	if d := usage.InputTokenDetails; d != nil {
		m["input_tokens_details"] = map[string]interface{}{"cached_tokens": float64(d.CachedTokens)}
	}
	if d := usage.OutputTokenDetails; d != nil {
		m["output_tokens_details"] = map[string]interface{}{"reasoning_tokens": float64(d.ReasoningTokens)}
	}
	captureUpstreamUsageMap(state, m)
}
