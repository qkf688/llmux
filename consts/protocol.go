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
