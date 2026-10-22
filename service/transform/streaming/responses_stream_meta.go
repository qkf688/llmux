package streaming

import (
	"github.com/qkf688/llmux/common/maputil"
)

// responseStreamMeta 缓存上游 chunk 的响应级元信息。
//
// 为什么需要缓存：Responses 流的终态事件 response.completed 必须回带
// id / model / created_at 三个字段。正常路径下它们取自「带 finish_reason 的那一帧」，
// 但兜底收尾（上游全程不给 finish_reason）手里没有那一帧——只能取整条流里
// 缓存下来的值。故每帧都尝试抓取，后到的不覆盖已有真值。
type responseStreamMeta struct {
	id      string
	model   string
	created int
}

// captureOpenAIStreamMeta 从 chunk 抓取响应级元信息并缓存。
//
// 只在字段非空时写入：OpenAI 的 usage 尾包（choices:[]）不带 id/model，
// 无条件覆盖会把已缓存的真值抹成空串。
func captureOpenAIStreamMeta(state *realtimeStreamState, chunk map[string]interface{}) {
	if id := maputil.String(chunk, "id"); id != "" {
		state.openAIStreamMeta.id = id
	}
	if model := maputil.String(chunk, "model"); model != "" {
		state.openAIStreamMeta.model = model
	}
	if created := int(maputil.Float64(chunk, "created")); created != 0 {
		state.openAIStreamMeta.created = created
	}
}
