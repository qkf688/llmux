package models

import "github.com/qkf688/llmux/common/maputil"

// 本文件是**上游 usage → models.Usage 的唯一归一入口**。
//
// 为什么落在 models：usage 的形状只取决于上游协议（openai / openai-res / anthropic）
// 三种，与下游 client 协议、与谁在调用无关；产出又恰好是 models.Usage。
// 四个调用方（transform 流式侧信道、anthropic→responses 出站流式、chat 的 processer
// 落库、anthropic 非流入站）分属不同 service 子包且互相不可依赖（service/chat 已
// import service/transform，反向复用会成环），共享点只能落在两边都能依赖的最底层。
//
// 收敛前四处各写一套，退化成同一概念的四个残缺子集并已经在漏字段：
// openai-res 的 processer 不读 reasoning、anthropic 非流入站不认 openai 兼容字段。
// 新增一种上游写法只该加一行候选路径，不该在第 N 处再抄一遍解析（DRY）。

// usageFieldCandidates 描述一个 token 字段在上游 JSON 里的候选取值路径，按优先级排列。
//
// 为什么用有序候选表而不是 per-provider if-else：兼容各家非标准写法（如把
// reasoning_tokens 放在 usage 顶层、用 prompt_cache_hit_tokens 表示缓存命中）时，
// 新增一种写法只加一行数据，不改控制流（OCP）。
//
// 排序规则（并存时谁胜出）：**嵌套标准位置 > 协议原生专有字段 > 顶层泛化兼容字段**。
// 依据：payload 里出现 input_tokens / cache_read_input_tokens 说明上游在用
// anthropic / responses 的原生口径，同一份 usage 里的 prompt_tokens、cached_tokens
// 只是 kimi 一类混合返回附带的兼容位，并存时原生字段才是权威值。
// 反向不成立——openai 原生响应不会额外冒出 input_tokens，故原生优先无副作用。
type usageFieldCandidates [][]string

var (
	promptTokenPaths = usageFieldCandidates{
		{"input_tokens"},
		{"prompt_tokens"},
	}
	completionTokenPaths = usageFieldCandidates{
		{"output_tokens"},
		{"completion_tokens"},
	}
	totalTokenPaths = usageFieldCandidates{
		{"total_tokens"},
	}
	cachedTokenPaths = usageFieldCandidates{
		{"prompt_tokens_details", "cached_tokens"},
		{"input_tokens_details", "cached_tokens"},
		// Anthropic Messages：缓存读取命中即统一模型的 cached_tokens。
		{"cache_read_input_tokens"},
		// DeepSeek 系列的写法：命中数放在 usage 顶层且键名不同。
		{"prompt_cache_hit_tokens"},
		{"cached_tokens"},
	}
	reasoningTokenPaths = usageFieldCandidates{
		{"completion_tokens_details", "reasoning_tokens"},
		{"output_tokens_details", "reasoning_tokens"},
		// 部分 OpenAI 兼容供应商把 reasoning 记在 usage 顶层。
		{"reasoning_tokens"},
	}
	// audio 明细无「顶层兼容写法」可收：顶层裸 audio_tokens 分不清是 input 侧还是
	// output 侧，猜错比不填更糟，故只认两侧各自的 details 位。
	promptAudioTokenPaths = usageFieldCandidates{
		{"prompt_tokens_details", "audio_tokens"},
		{"input_tokens_details", "audio_tokens"},
	}
	completionAudioTokenPaths = usageFieldCandidates{
		{"completion_tokens_details", "audio_tokens"},
		{"output_tokens_details", "audio_tokens"},
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

// UsageFromMap 把任意上游协议的 usage map 归一为 Usage。
//
// 契约：只认「> 0 的第一个候选」，所以「上游没给」与「上游明确报 0」都落到 0，
// 不虚构数值、也不产出零值明细。total 缺失时按 ResolveTotalTokens 回退。
//
// **必须覆盖 Usage 的全部明细字段**：这里漏一个字段，对应上游的该项统计就恒为 0
// 且无人察觉（audio 明细就曾因此漏过一轮——旧 openai processer 靠 json tag 全量
// 解码能读到，改走候选表后若不补齐即静默丢失）。给 Usage 加新明细字段时，
// 本函数与候选表必须同步。
func UsageFromMap(usage map[string]interface{}) Usage {
	u := Usage{
		PromptTokens:     pickUsageField(usage, promptTokenPaths),
		CompletionTokens: pickUsageField(usage, completionTokenPaths),
		TotalTokens:      pickUsageField(usage, totalTokenPaths),
	}
	u.TotalTokens = ResolveTotalTokens(u.PromptTokens, u.CompletionTokens, u.TotalTokens)
	u.PromptTokensDetails.CachedTokens = pickUsageField(usage, cachedTokenPaths)
	u.PromptTokensDetails.AudioTokens = pickUsageField(usage, promptAudioTokenPaths)
	u.CompletionTokensDetails.ReasoningTokens = pickUsageField(usage, reasoningTokenPaths)
	u.CompletionTokensDetails.AudioTokens = pickUsageField(usage, completionAudioTokenPaths)
	return u
}

// ResolveTotalTokens 统一 total 口径：上游省略 total（==0）时回退为 prompt+completion。
//
// 纯函数。收口这条回退，避免「发给客户端的 total」与「交给落库侧的 total」分叉——
// 客户端写出点此前直写上游原值，缺失时写出 0，而归一这侧有回退，结果客户端看 0、
// DB 记回退值。
//
// 口径分两种：**回退口径**（上游没给 total）为 prompt+completion，不含 Anthropic 的
// cache token，与 Usage.HasTokens 一致；**上游优先**——上游显式给了 total（>0）时
// 一律原样采用，不重算，即使该值含 cache token（贴近上游真值，与 logs-metrics.md 一致）。
func ResolveTotalTokens(prompt, completion, total int64) int64 {
	if total > 0 {
		return total
	}
	return prompt + completion
}
