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
// 顶层的多态字段停在 json.RawMessage，由各自的 parse 函数带**形状守卫**二次解析
// （shared.DecodeJSONObject / shared.RawString），而不是直接声明成子 struct：
// system 是 string｜块数组、tool_choice 是 string｜对象，直接 Unmarshal 到 struct
// 会让形状不符的合法请求整条 400，与 shared.Optional* 那套宽容语义相悖。
//
// messages 是**唯一**仍走 map 解析的字段：content 块有 6 种 type 混排，struct 化需要
// 自定义分派，收益待评估（见 .plan/stages.md Stage 2）。
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

// anthropicThinking 是 thinking 字段的嵌套 DTO。
//
// budget_tokens 用 OptionalNumber 而非裸 int64：客户端把它写成字符串或对象时必须
// 当作未传（与 map 时代 maputil.Int64 的断言失败等价），不能让整条请求 400。
type anthropicThinking struct {
	Type         shared.Optional[string]      `json:"type"`
	BudgetTokens shared.OptionalNumber[int64] `json:"budget_tokens"`
}

// anthropicOutputConfig 是 output_config 字段的嵌套 DTO（Claude 4.6 adaptive thinking）。
type anthropicOutputConfig struct {
	Effort shared.Optional[string] `json:"effort"`
}

// anthropicToolChoice 是 tool_choice **对象形态**的嵌套 DTO；另一形态是裸字符串，
// 由 parseToolChoice 先行分派，不在本 DTO 表达。
type anthropicToolChoice struct {
	Type shared.Optional[string] `json:"type"`
	Name shared.Optional[string] `json:"name"`
}

// anthropicCacheControl 是 cache_control 的嵌套 DTO。
type anthropicCacheControl struct {
	Type shared.Optional[string] `json:"type"`
}

// anthropicTextBlock 是 system 数组元素（纯 text 块）的 DTO。
//
// text 与 content 两个键并存是历史兼容：部分客户端把文本写进 content。取值优先
// text、回落 content，与 map 时代一致。
type anthropicTextBlock struct {
	Type         shared.Optional[string] `json:"type"`
	Text         shared.Optional[string] `json:"text"`
	Content      shared.Optional[string] `json:"content"`
	CacheControl json.RawMessage         `json:"cache_control"`
}

// anthropicTool 是 tools 数组元素的 DTO。
//
// input_schema 停在 RawMessage 原样透传：它是客户端自定的 JSON Schema，网关不解释
// 其结构，解成 map 再塞回去只是无谓的往返损耗。
type anthropicTool struct {
	Name         shared.Optional[string] `json:"name"`
	Description  shared.Optional[string] `json:"description"`
	InputSchema  json.RawMessage         `json:"input_schema"`
	CacheControl json.RawMessage         `json:"cache_control"`
}

// ClaimedRequestKeys 返回 Anthropic 入站解析实际认领的顶层键集合。
//
// 定义在本包而非 service/transform/anthropic：认领键必须与真正做解析的 DTO 同源，
// 放到 adapter 侧就得导出 DTO 或手抄一份清单，两者都会漂移。transform 侧只做转发。
func ClaimedRequestKeys() map[string]struct{} {
	return shared.ClaimedJSONKeys(anthropicRequest{})
}
