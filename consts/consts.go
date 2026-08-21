package consts

// Style 标识**客户端入站协议**：客户端从哪个端点进来。
// 闭集，与 handler/register_v1.go 的三条路由一一对应，也是 models.ChatLog.Style 的落库取值。
//
// 这里刻意用 defined type 而非 `= string` 别名：Style 与 WireFormat 的字符串取值完全相同
// （见 wire_format.go 的取值一致性约束），只有让编译器把两者当成不同类型，才能拦住
// 「把上游 provider type 当客户端 style 传」这类混用——靠命名纪律拦是拦不住的。
type Style string

const (
	StyleOpenAI    Style = "openai"
	StyleOpenAIRes Style = "openai-res"
	StyleAnthropic Style = "anthropic"
)
