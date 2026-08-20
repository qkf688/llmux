package anthropic

import (
	"encoding/json"

	"github.com/qkf688/llmux/service/transform/shared"
)

// anthropicRequest 是 Anthropic 入站请求的顶层 DTO。
//
// 它同时是两件事的单一数据源：顶层字段的解析入口，以及未认领字段检测的认领键集合
// （由 ClaimedRequestKeys 反射 json tag 得出）。加字段时两者自动同步，无需维护第二
// 份键清单。
//
// **纪律：往本 DTO 加字段与把该字段接进 UnifiedRequest 必须是同一次改动。**
// ClaimedRequestKeys 反射的是 tag，不是「这个字段有没有真的被读」——只加 tag 会让检测
// 对该键闭嘴，把真实的丢字段缺口伪装成已解决，比没有检测更糟。Anthropic 真实 API 的
// top_k / service_tier / container / mcp_servers 等当前确实未解析，检测把它们报出来是
// 正确行为，不要为了「让报告干净」把它们塞进这里。
//
// 嵌套结构刻意停在 json.RawMessage：system 可以是裸 string 或块数组、content 块有
// 6 种 type 混排，struct 化需要自定义分派，收益待评估（见 .plan/stages.md Stage 2），
// 且对**顶层**键检测零贡献。内部仍走既有 map 解析。
type anthropicRequest struct {
	Model         shared.Optional[string]        `json:"model"`
	Stream        shared.Optional[bool]          `json:"stream"`
	MaxTokens     shared.OptionalNumber[int]     `json:"max_tokens"`
	Temperature   shared.OptionalNumber[float64] `json:"temperature"`
	TopP          shared.OptionalNumber[float64] `json:"top_p"`
	StopSequences shared.OptionalStringSeq       `json:"stop_sequences"`
	Metadata      shared.OptionalStringMap       `json:"metadata"`
	System        json.RawMessage                `json:"system"`
	Messages      json.RawMessage                `json:"messages"`
	Tools         json.RawMessage                `json:"tools"`
	Thinking      json.RawMessage                `json:"thinking"`
	OutputConfig  json.RawMessage                `json:"output_config"`
	ToolChoice    json.RawMessage                `json:"tool_choice"`
}

// ClaimedRequestKeys 返回 Anthropic 入站解析实际认领的顶层键集合。
//
// 定义在本包而非 service/transform/anthropic：认领键必须与真正做解析的 DTO 同源，
// 放到 adapter 侧就得导出 DTO 或手抄一份清单，两者都会漂移。transform 侧只做转发。
func ClaimedRequestKeys() map[string]struct{} {
	return shared.ClaimedJSONKeys(anthropicRequest{})
}
