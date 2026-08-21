package consts

// WireFormat 标识请求/响应 body 的**协议形状**（wire 格式）。
// service/transform 只认这个：它决定用哪个 FormatAdapter 解析与构建 body，
// 与「上游是哪一家供应商」无关。
//
// 与 provider type 的分工：
//   - provider type 是**开集**（每接一家上游就多一个，存在 DB 的 Provider.Type，取值域归 providers 包），
//     由 providers.Metadata 声明自己说哪种 WireFormat。
//   - WireFormat 是**闭集**：只有写了适配器的形状才存在。加一家 OpenAI 兼容的新上游不该新增 WireFormat。
type WireFormat string

const (
	FormatOpenAIChat      WireFormat = "openai"
	FormatOpenAIResponses WireFormat = "openai-res"
	FormatAnthropic       WireFormat = "anthropic"
)

// WireFormatOfStyle 给出某个客户端入站 style 对应的 body 形状。
//
// 三条入站路由各自只接受一种形状，故为 1:1。未知 style 返回 false 而**不兜底**：
// 兜底会让「路由注册了但没接协议适配」这类漏配静默退化成按 OpenAI 形状解析，
// 错误现象会飘到很远的地方才暴露。由调用方决定报错还是自己兜。
func WireFormatOfStyle(style Style) (WireFormat, bool) {
	switch style {
	case StyleOpenAI:
		return FormatOpenAIChat, true
	case StyleOpenAIRes:
		return FormatOpenAIResponses, true
	case StyleAnthropic:
		return FormatAnthropic, true
	}
	return "", false
}
