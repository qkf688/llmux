package consts

import "testing"

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
