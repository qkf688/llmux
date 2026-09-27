package common

import "testing"

// 提交门内容判据表驱动：三家协议的错误形状 wire 无关（error 键非 null /
// type:"error" / event: error 行），合法成功载荷与 [DONE] 必须放行。
func TestSSEFrameContentError(t *testing.T) {
	cases := []struct {
		name    string
		frame   SSEFrame
		wantErr bool
	}{
		{"openai 错误载荷", SSEFrame{Data: `{"error":{"message":"boom","type":"server_error"}}`}, true},
		{"openai 正常 chunk", SSEFrame{Data: `{"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"hi"}}]}`}, false},
		{"openai error:null 免检", SSEFrame{Data: `{"id":"c1","choices":[],"error":null}`}, false},
		{"anthropic error 事件行", SSEFrame{Event: "error", Data: `{"type":"error","error":{"type":"overloaded_error","message":"busy"}}`}, true},
		{"anthropic 正常 message_start", SSEFrame{Event: "message_start", Data: `{"type":"message_start","message":{"id":"m1"}}`}, false},
		{"responses 错误载荷(type:error)", SSEFrame{Event: "error", Data: `{"type":"error","error":{"code":"server_error","message":"boom"}}`}, true},
		{"responses 正常 completed", SSEFrame{Event: "response.completed", Data: `{"type":"response","response":{"id":"r1"}}`}, false},
		{"DONE 终止符放行", SSEFrame{Data: "[DONE]"}, false},
		{"非法 JSON", SSEFrame{Data: `<html>login</html>`}, true},
		{"空载荷", SSEFrame{Data: "   "}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := SSEFrameContentError(tc.frame)
			if (err != nil) != tc.wantErr {
				t.Fatalf("SSEFrameContentError(%+v) err = %v, wantErr %v", tc.frame, err, tc.wantErr)
			}
		})
	}
}

func TestJSONContentError(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{"合法对象", `{"id":"x","object":"chat.completion"}`, false},
		{"空体", ``, true},
		{"纯空白", "  \n ", true},
		{"HTML", `<html>login</html>`, true},
		{"错误对象", `{"error":{"message":"quota"}}`, true},
		{"type:error 对象", `{"type":"error","error":{"message":"busy"}}`, true},
		{"数组非对象", `[1,2,3]`, true},
		{"error:null 对象", `{"id":"x","error":null}`, false},
		{"error 值前异常空白 null", `{"id":"x","error":  null}`, false},
		{"type 值前异常空白 error", `{"type":  "error","error":{"message":"busy"}}`, true},
		{"裸 null 非对象", `null`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := JSONContentError([]byte(tc.data))
			if (err != nil) != tc.wantErr {
				t.Fatalf("JSONContentError(%q) err = %v, wantErr %v", tc.data, err, tc.wantErr)
			}
		})
	}
}
