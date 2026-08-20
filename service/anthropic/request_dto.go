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
// messages 及其 content 块同样走 struct：anthropicMessage → 先用
// anthropicContentBlockEnvelope peek 出块的 type → 再把同一份 raw 解进对应的
// 块子 struct（见下方块 DTO 群），单次遍历一并产出各类结果（parseContentBlocks）。
// **请求入站已无 map 逐键取值**（响应入站的
// response_inbound.go 仍走 map，不在本 DTO 的职责内）。
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

// anthropicTextBlock 是 text 块的 DTO，system 数组元素与 messages[].content 的
// text 块共用——Anthropic 协议里两处是同一种块，没有理由声明两份。
//
// text 与 content 两个键并存是历史兼容：部分客户端把文本写进 content。取值优先
// text、回落 content，与 map 时代一致。
type anthropicTextBlock struct {
	Type         shared.Optional[string] `json:"type"`
	Text         shared.Optional[string] `json:"text"`
	Content      shared.Optional[string] `json:"content"`
	CacheControl json.RawMessage         `json:"cache_control"`
}

// anthropicContentBlockEnvelope 只读 content 块的判别字段 type，供分派层先行 peek。
//
// 这是「envelope-peek + 各 type 独立子 struct」的判别层：读出 type 后再把同一份
// RawMessage 解进对应的块 struct。**禁止**退回成把 6 种 type 的字段并进一个大 struct
// （openai 侧的 openAIChatContentPart 就是那种字段并集，是反面参照不是模板）——并集
// 会让「哪些键属于哪个 type」这个信息从类型系统里消失。
type anthropicContentBlockEnvelope struct {
	Type shared.Optional[string] `json:"type"`
}

// anthropicImageBlock 是 image 块的 DTO。
type anthropicImageBlock struct {
	Source       json.RawMessage `json:"source"`
	CacheControl json.RawMessage `json:"cache_control"`
}

// anthropicImageSource 是 image.source 的 DTO：base64 与 url 两种 source type
// 用的字段不同，这里平铺是因为它们同属一个 source 对象、由 type 决定读哪几个，
// 不构成跨块的字段并集。
type anthropicImageSource struct {
	Type      shared.Optional[string] `json:"type"`
	MediaType shared.Optional[string] `json:"media_type"`
	Data      shared.Optional[string] `json:"data"`
	URL       shared.Optional[string] `json:"url"`
}

// anthropicToolUseBlock 是 tool_use 块的 DTO。
//
// input 停在 RawMessage：协议允许它是对象，但真实流量里也出现数组与标量。统一模型的
// arguments 是字符串，只在 input 确为 JSON 对象时才 marshal 进去，其余形态一律塌成
// "{}"（表征测试 TestTransformToUnified_ToolUseNonMapInputKeepsEmptyArgs 锁死了这点）。
type anthropicToolUseBlock struct {
	ID           shared.Optional[string] `json:"id"`
	Name         shared.Optional[string] `json:"name"`
	Input        json.RawMessage         `json:"input"`
	CacheControl json.RawMessage         `json:"cache_control"`
}

// anthropicToolResultBlock 是 tool_result 块的 DTO。
//
// content 停在 RawMessage：它是 string｜块数组二义，由 parseToolResultContent 二次分派。
type anthropicToolResultBlock struct {
	ToolUseID    shared.Optional[string] `json:"tool_use_id"`
	Content      json.RawMessage         `json:"content"`
	IsError      shared.Optional[bool]   `json:"is_error"`
	CacheControl json.RawMessage         `json:"cache_control"`
}

// anthropicThinkingBlock 是 thinking 块的 DTO（与顶层 thinking 字段的
// anthropicThinking 是两回事：那个配置 budget，这个承载 assistant 轮的思考内容）。
type anthropicThinkingBlock struct {
	Thinking  shared.Optional[string] `json:"thinking"`
	Signature shared.Optional[string] `json:"signature"`
}

// anthropicRedactedThinkingBlock 是 redacted_thinking 块的 DTO。
// data 是不透明密文，只搬运不解析。
type anthropicRedactedThinkingBlock struct {
	Data shared.Optional[string] `json:"data"`
}

// anthropicMessage 是 messages 数组元素的 DTO；content 停在 RawMessage，
// 因为它是 string｜块数组二义，由 parseContentBlocks 分派。
type anthropicMessage struct {
	Role         shared.Optional[string] `json:"role"`
	Content      json.RawMessage         `json:"content"`
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

// MismatchedRequestKeys 返回被本协议认领、但值类型不匹配而被 shared.Optional* 容器
// 静默丢弃的顶层键。定义在本包的理由同 ClaimedRequestKeys：必须与真正解析的 DTO 同源。
//
// 传零值样板与 ClaimedRequestKeys 同形：DTO 只用来提供类型，解码用的实例由 shared
// 内部新建，故本函数无状态、可并发调用。
func MismatchedRequestKeys(rawBody []byte) ([]string, error) {
	return shared.MismatchedTopLevelKeys(rawBody, anthropicRequest{})
}
