package consts

import "testing"

// TestTypeOfProtocol 锁定 protocol → provider type 反向映射：
// 端点协议 responses 对应注册类型 openai-res（而非 Style 字面值），
// 未知协议**不兜底**（兜底会静默把漏配协议按 OpenAI 适配，错误推迟到上游才暴露）。
func TestTypeOfProtocol(t *testing.T) {
	tests := []struct {
		name     string
		protocol Protocol
		want     string
		wantOK   bool
	}{
		{name: "openai", protocol: ProtocolOpenAI, want: "openai", wantOK: true},
		{name: "responses → openai-res", protocol: ProtocolResponses, want: "openai-res", wantOK: true},
		{name: "anthropic", protocol: ProtocolAnthropic, want: "anthropic", wantOK: true},
		{name: "未知协议不兜底", protocol: Protocol("unknown"), want: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := TypeOfProtocol(tt.protocol)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("TypeOfProtocol(%q) = (%q, %v), want (%q, %v)", tt.protocol, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestWireFormatOfProtocol 锁定 protocol → wire format 反向映射
// （S3 透传判定取数点将改用端点协议，此前取数点只按 provider type）。
func TestWireFormatOfProtocol(t *testing.T) {
	tests := []struct {
		name     string
		protocol Protocol
		want     WireFormat
		wantOK   bool
	}{
		{name: "openai", protocol: ProtocolOpenAI, want: FormatOpenAIChat, wantOK: true},
		{name: "responses", protocol: ProtocolResponses, want: FormatOpenAIResponses, wantOK: true},
		{name: "anthropic", protocol: ProtocolAnthropic, want: FormatAnthropic, wantOK: true},
		{name: "未知协议不兜底", protocol: Protocol("unknown"), want: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := WireFormatOfProtocol(tt.protocol)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("WireFormatOfProtocol(%q) = (%q, %v), want (%q, %v)", tt.protocol, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestProtocolOfType 锁定 provider type → 默认出站协议映射（与前端
// webui/src/routes/providers/utils/schedule.ts 的 protocolOfType 同构）。
func TestProtocolOfType(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		want Protocol
	}{
		{name: "openai", typ: "openai", want: ProtocolOpenAI},
		{name: "openai-res → responses", typ: "openai-res", want: ProtocolResponses},
		{name: "anthropic", typ: "anthropic", want: ProtocolAnthropic},
		{name: "未知名默认 openai", typ: "unknown-type", want: ProtocolOpenAI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProtocolOfType(tt.typ); got != tt.want {
				t.Fatalf("ProtocolOfType(%q) = %q, want %q", tt.typ, got, tt.want)
			}
		})
	}
}
