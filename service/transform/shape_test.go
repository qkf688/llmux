package transform

import (
	"bytes"
	"strings"
	"testing"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
)

// TestValidateResponseShape 锁「检测类内容门」口径（Stage 19）：上游回 200/201 但内容
// 并非合法响应时不得判健康——否则健康检查绿、真实聊天每条报错（Stage 5 盲区的镜像）。
// 口径三条：① 空回复判过（门只挡形状不挡语义，对齐 RunStreamProbe 空流 Passed）；
// ② 错误形状判失败（common.JSONContentError，与 chat 提交门同源判据）；
// ③ 必须能被所测 wire format 的解析入口解析（HTML/垃圾页在此挡下）。
func TestValidateResponseShape(t *testing.T) {
	cases := []struct {
		name    string
		format  consts.WireFormat
		body    string
		wantErr string // 空 = 判过；非空 = 判失败且错误文本含此子串
	}{
		{name: "openai 空回复判过", format: consts.FormatOpenAIChat, body: ""},
		{name: "openai 空白回复判过", format: consts.FormatOpenAIChat, body: " \n\t "},
		{name: "openai 合法 0-content 判过", format: consts.FormatOpenAIChat, body: `{"id":"cmpl-1","object":"chat.completion","model":"gpt-4.1","choices":[]}`},
		{name: "anthropic 合法 0-content 判过", format: consts.FormatAnthropic, body: `{"id":"msg_1","type":"message","role":"assistant","model":"claude-x","content":[],"stop_reason":"end_turn"}`},
		{name: "responses 合法 0-content 判过", format: consts.FormatOpenAIResponses, body: `{"id":"resp_1","object":"response","status":"completed","output":[]}`},
		{name: "html 登录页判失败", format: consts.FormatOpenAIChat, body: `<html>login required</html>`, wantErr: "not valid JSON"},
		{name: "非 JSON 垃圾判失败", format: consts.FormatOpenAIChat, body: `hello world`, wantErr: "not valid JSON"},
		{name: "非对象 JSON 判失败", format: consts.FormatOpenAIChat, body: `[1,2,3]`, wantErr: "not an object"},
		{name: "openai 错误形状判失败", format: consts.FormatOpenAIChat, body: `{"error":{"message":"quota exceeded"}}`, wantErr: "error payload"},
		{name: "anthropic 错误形状判失败", format: consts.FormatAnthropic, body: `{"type":"error","error":{"message":"overloaded"}}`, wantErr: "error payload"},
		{name: "未注册 wire format 不回退判失败", format: consts.WireFormat("unknown-format"), body: `{"id":"x"}`, wantErr: "not registered"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateResponseShape(tc.format, []byte(tc.body))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateResponseShape(%s, %q) = %v, want nil", tc.format, tc.body, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateResponseShape(%s, %q) = nil, want error containing %q", tc.format, tc.body, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateResponseShape(%s, %q) err = %v, want containing %q", tc.format, tc.body, err, tc.wantErr)
			}
		})
	}
}

// TestValidateResponseShape_OversizeBodyUnverifiable 锁超限口径随函数走（AC-4）：响应体达到
// 诊断读体上限即「形状不可验」——截断后的 JSON 不可信，判过会把无法验证当健康；判失败文案
// 刻意同时覆盖「被截断」与「恰好等长」两种可能（ReadCappedBody 的 LimitReader 语义区分不了）。
func TestValidateResponseShape_OversizeBodyUnverifiable(t *testing.T) {
	atCap := bytes.Repeat([]byte("a"), common.UpstreamDiagnosticReadLimitBytes)
	err := ValidateResponseShape(consts.FormatOpenAIChat, atCap)
	if err == nil || !strings.Contains(err.Error(), "read cap") {
		t.Fatalf("ValidateResponseShape(at cap) err = %v, want 含 read cap 的形状不可验错误", err)
	}
}
