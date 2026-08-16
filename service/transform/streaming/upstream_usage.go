package streaming

import (
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/responses"
)

// 本文件负责把**上游原始 usage** 旁路交给落库侧（models.TransformSideChannel）。
//
// 为什么不复用转换后的 usage：转换输出受目标协议表达能力限制（Anthropic 无
// reasoning token 槽位、message_start 必须早于真实 prompt_tokens 发出），
// 从下游流反解必然有损。故在**读上游那一跳**就地捕获。
//
// 归一本身（各家非标准 usage 形状 → models.Usage）不在本包实现，统一走
// models.UsageFromMap——同一份归一还被 chat 的 processer 落库路径与 anthropic
// 非流入站复用，四处各写一套必然漂移成残缺子集。本文件只负责「什么时候、
// 从哪一跳交出去」。
//
// 只有持有 sideChannel 的那一跳会真正写入：多跳转换里第二跳的 sideChannel 为 nil，
// 这些函数退化为 no-op，不会用中间格式污染上游真值。

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
	state.sideChannel.SetUpstreamUsage(models.UsageFromMap(usage))
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
