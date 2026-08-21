package consts

import "testing"

// TestWireFormatOfStyle 锁定「客户端入站 style → body 协议形状」的 1:1 映射。
// 三条入站路由各自只接受一种 body 形状，映射表改错会让 transform 拿错适配器解析请求。
func TestWireFormatOfStyle(t *testing.T) {
	tests := []struct {
		name       string
		style      Style
		wantFormat WireFormat
		wantOK     bool
	}{
		{name: "openai chat 端点", style: StyleOpenAI, wantFormat: FormatOpenAIChat, wantOK: true},
		{name: "responses 端点", style: StyleOpenAIRes, wantFormat: FormatOpenAIResponses, wantOK: true},
		{name: "anthropic messages 端点", style: StyleAnthropic, wantFormat: FormatAnthropic, wantOK: true},
		{name: "未注册 style 不兜底", style: Style("gemini"), wantFormat: "", wantOK: false},
		{name: "空 style 不兜底", style: Style(""), wantFormat: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := WireFormatOfStyle(tt.style)
			if ok != tt.wantOK {
				t.Fatalf("WireFormatOfStyle(%q) ok = %v, want %v", tt.style, ok, tt.wantOK)
			}
			if got != tt.wantFormat {
				t.Fatalf("WireFormatOfStyle(%q) = %q, want %q", tt.style, got, tt.wantFormat)
			}
		})
	}
}

// TestWireFormatValuesMatchStyleValues 锁定「两个概念的字符串取值相同」这条前提。
// DB 里 Provider.Type 存的就是这些字面量（"openai" / "openai-res" / "anthropic"），
// 取值一旦漂移，存量 provider 会查不到 metadata 而全部不可用——这是纯概念分层不该付的代价。
func TestWireFormatValuesMatchStyleValues(t *testing.T) {
	tests := []struct {
		style  Style
		format WireFormat
	}{
		{style: StyleOpenAI, format: FormatOpenAIChat},
		{style: StyleOpenAIRes, format: FormatOpenAIResponses},
		{style: StyleAnthropic, format: FormatAnthropic},
	}

	for _, tt := range tests {
		if string(tt.style) != string(tt.format) {
			t.Errorf("style %q 与 format %q 取值不一致", tt.style, tt.format)
		}
	}
}
