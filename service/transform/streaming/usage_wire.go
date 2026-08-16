package streaming

import (
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/responses"
)

// 本文件收敛 models.Usage → **出站 usage 线格式**的装配，并提供
// responses.ResponsesUsage → models.Usage 的类型转换。
//
// 为什么收敛：三个写出点（anthropic→responses 的 response.completed、
// openai→responses 的 response.completed、responses→openai 的 usage 尾包）
// 各自手写「三个总量 + 两个条件明细」，键名分属两条协议线，但装配规则完全一致。
// 规则同构却分散，已两次发散：写出点曾直写上游 total 导致与落库分叉；details
// 判据曾在一处附加了父级 >0 的前置条件。规则只该有一份实现。
//
// 边界一：归一（上游任意形状 → models.Usage）不在本文件，走 models.UsageFromMap；
// 本文件只管「已归一的 Usage 按某条协议线的键名怎么写出」。
// 「什么时候、从哪一跳把上游 usage 交给落库侧」在 upstream_usage.go。
//
// 边界二：**anthropic 客户端线不收进键名表**——`responses_to_anthropic.go` 的
// message_delta usage 差异不止键名：无 total_tokens、cached 走顶层
// cache_read_input_tokens 而非嵌套 details、无 reasoning 槽位。硬塞进
// usageWireKeys 需要加一堆「是否写 total」「明细是否顶层」布尔开关，参数化会
// 退化成分支堆，比重复更糟，故保留其手写装配。**但零值明细判据（>0 才写）
// 必须与本文件保持一致**——那处若独立漂移，就是本文件要防的第三次发散。

// usageWireKeys 描述一条协议线的 usage 键名。
//
// 两条线的差异**只有键名**：语义量、零值判据、total 回退口径完全相同。
// 故用键名表参数化，而非每条线一套装配分支——新增协议线只加一份键名数据（OCP）。
type usageWireKeys struct {
	prompt            string
	completion        string
	total             string
	promptDetails     string
	completionDetails string
}

// 键名表是**只读数据**：改这里等于改对外协议契约，不要为调试临时改（包级 var
// 只因 Go 无 const struct，语义上应视作常量）。
var (
	// Responses 线：/v1/responses 的 response.usage。
	responsesUsageKeys = usageWireKeys{
		prompt:            "input_tokens",
		completion:        "output_tokens",
		total:             "total_tokens",
		promptDetails:     "input_tokens_details",
		completionDetails: "output_tokens_details",
	}
	// OpenAI Chat 线：chat.completion / chat.completion.chunk 的 usage。
	openAIUsageKeys = usageWireKeys{
		prompt:            "prompt_tokens",
		completion:        "completion_tokens",
		total:             "total_tokens",
		promptDetails:     "prompt_tokens_details",
		completionDetails: "completion_tokens_details",
	}
)

// usageWireFromModel 按给定协议线的键名装配 usage 对象。纯函数。
//
// total 一律过 models.ResolveTotalTokens：与侧信道交给落库侧的口径同源，且对
// 已归一的入参幂等，故调用点无需自行判断是否回退过（少一个可漏的前置条件）。
//
// **零值明细不写出**：cached / reasoning 只在 > 0 时才产出 details 子对象。
// 「字段缺失＝未知」与「上游明确报告 0」是两种语义，写出 0 会被下游读成后者。
// 判据只看 detail 本身，不附加父级 prompt / completion > 0 的前置条件——出站
// 与入站归一 models.UsageFromMap 必须同口径，否则 cached>0 && prompt==0
// 这类异常上游下两侧发散。
//
// audio 明细**有意不写出**：三条线现有的客户端契约都未包含它，落库侧仍记录
// （models.Usage 保留该字段）。要补须同时定下各线键名并更新契约文档，不在此
// 悄悄扩，否则「装配器吃 Usage 全字段」会被误读成已覆盖 audio。
func usageWireFromModel(u models.Usage, k usageWireKeys) map[string]interface{} {
	wire := map[string]interface{}{
		k.prompt:     int(u.PromptTokens),
		k.completion: int(u.CompletionTokens),
		k.total:      int(models.ResolveTotalTokens(u.PromptTokens, u.CompletionTokens, u.TotalTokens)),
	}
	if cached := u.PromptTokensDetails.CachedTokens; cached > 0 {
		wire[k.promptDetails] = map[string]interface{}{"cached_tokens": int(cached)}
	}
	if reasoning := u.CompletionTokensDetails.ReasoningTokens; reasoning > 0 {
		wire[k.completionDetails] = map[string]interface{}{"reasoning_tokens": int(reasoning)}
	}
	return wire
}

// responsesUsageFromModel 装配 Responses 线的 usage 对象。
//
// 空值语义（无 usage 时是否写出该字段）留给调用点：各写出点的「没有 usage」
// 判据不同（openai→responses 看 pending map 是否为空、anthropic→responses 看
// 两处 usage 帧是否出现过），塞进装配器会让其中一方的判据失真。
func responsesUsageFromModel(u models.Usage) map[string]interface{} {
	return usageWireFromModel(u, responsesUsageKeys)
}

// openAIUsageFromModel 装配 OpenAI Chat 线的 usage 对象。
func openAIUsageFromModel(u models.Usage) map[string]interface{} {
	return usageWireFromModel(u, openAIUsageKeys)
}

// usageFromResponses 把结构化的 ResponsesUsage 转为 models.Usage。调用方保证 u != nil。
//
// 上游是 openai-res 时 usage 已是强类型，字段位置在类型里就确定了，不必绕一趟
// map 再交候选表按键名识别。ResponsesUsage 无 audio 槽位，故 audio 明细恒为零值。
func usageFromResponses(u *responses.ResponsesUsage) models.Usage {
	out := models.Usage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      models.ResolveTotalTokens(u.InputTokens, u.OutputTokens, u.TotalTokens),
	}
	if d := u.InputTokenDetails; d != nil {
		out.PromptTokensDetails.CachedTokens = d.CachedTokens
	}
	if d := u.OutputTokenDetails; d != nil {
		out.CompletionTokensDetails.ReasoningTokens = d.ReasoningTokens
	}
	return out
}
