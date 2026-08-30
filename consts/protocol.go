package consts

// Protocol 标识协议端点（endpoints.Protocol）的出站协议。
// 取值与前端 S0 原型（webui/src/routes/providers/utils/schedule.ts）一致：
// openai / responses / anthropic。注意与 Style / WireFormat 的差异：
// StyleOpenAIRes 的字符串是 "openai-res"，而 responses 端点协议是 "responses"。
type Protocol string

const (
	ProtocolOpenAI    Protocol = "openai"
	ProtocolResponses Protocol = "responses"
	ProtocolAnthropic Protocol = "anthropic"
)

// ProtocolOfType 给出 provider type 的默认出站协议（存量迁移与新建缺省时用）。
// 与前端 protocolOfType 同构（webui/src/routes/providers/utils/schedule.ts），
// 两侧改动必须同步（直返实体两侧同源）。
func ProtocolOfType(providerType string) Protocol {
	switch providerType {
	case "anthropic":
		return ProtocolAnthropic
	case "openai-res":
		return ProtocolResponses
	default:
		return ProtocolOpenAI
	}
}

// protocolToMeta 是 Protocol → provider type / wire format 的唯一映射表。
// 与 ProtocolOfType 是反向关系，但**不共享**同一个 switch：ProtocolOfType 面向
// 存量兜底（未知 type 回落 openai），本表面向端点协议校验（未知协议必须显式失败，
// 静默回落会把漏配的端点按 OpenAI 适配，错误推迟到上游才暴露——与
// WireFormatOfStyle 不兜底同一原则）。
type protocolMeta struct {
	providerType string
	wireFormat   WireFormat
}

var protocolMetaTable = map[Protocol]protocolMeta{
	ProtocolOpenAI:    {providerType: string(StyleOpenAI), wireFormat: FormatOpenAIChat},
	ProtocolResponses: {providerType: string(StyleOpenAIRes), wireFormat: FormatOpenAIResponses},
	ProtocolAnthropic: {providerType: string(StyleAnthropic), wireFormat: FormatAnthropic},
}

// TypeOfProtocol 给出协议端点出站协议对应的 provider 注册类型（providers.New 的第一参）。
// 注意 responses 端点协议 ↔ openai-res 注册类型（endpointPath=/responses），
// 不是字面同值。未知协议返回 false。
func TypeOfProtocol(protocol Protocol) (string, bool) {
	meta, ok := protocolMetaTable[protocol]
	if !ok {
		return "", false
	}
	return meta.providerType, true
}

// WireFormatOfProtocol 给出协议端点出站协议对应的 body 形状（透传判定取数点）。
// S3 起透传判定按选中端点协议而非 Provider.Type，此映射是把端点协议
// 投影到闭集 WireFormat 的唯一入口。未知协议返回 false。
func WireFormatOfProtocol(protocol Protocol) (WireFormat, bool) {
	meta, ok := protocolMetaTable[protocol]
	if !ok {
		return "", false
	}
	return meta.wireFormat, true
}
