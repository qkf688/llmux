package apikeys

import "testing"

// 净化器直测（表驱动）：HTTP 契约测试走不到的字符边界在此钉死。
func TestSanitizeOutboundHeaders(t *testing.T) {
	cases := []struct {
		name    string
		input   map[string]string
		want    map[string]string
		wantErr bool
	}{
		{name: "nil 保持 nil（未配置≠空配置）", input: nil, want: nil},
		{name: "规范化 Title-Case", input: map[string]string{"x-app": "cli"}, want: map[string]string{"X-App": "cli"}},
		{name: "session_id → Session_id（与出站 Set 同口径）", input: map[string]string{"session_id": "s"}, want: map[string]string{"Session_id": "s"}},
		{name: "token 特殊字符名合法", input: map[string]string{"X-Codex-Turn-State": "a"}, want: map[string]string{"X-Codex-Turn-State": "a"}},
		{name: "空名拒绝", input: map[string]string{"   ": "v"}, wantErr: true},
		{name: "名含空格拒绝", input: map[string]string{"Bad Name": "v"}, wantErr: true},
		{name: "名含中文拒绝", input: map[string]string{"中文": "v"}, wantErr: true},
		{name: "名含冒号拒绝", input: map[string]string{"a:b": "v"}, wantErr: true},
		{name: "值含 CRLF 拒绝（头注入）", input: map[string]string{"X-A": "a\r\nX-B: injected"}, wantErr: true},
		{name: "值含 NUL 拒绝", input: map[string]string{"X-A": "a\x00b"}, wantErr: true},
		{name: "值含 DEL 拒绝", input: map[string]string{"X-A": "a\x7fb"}, wantErr: true},
		{name: "值含 TAB 允许（httpguts 同判据）", input: map[string]string{"X-A": "a\tb"}, want: map[string]string{"X-A": "a\tb"}},
		{name: "值空允许（预设占位行）", input: map[string]string{"X-A": ""}, want: map[string]string{"X-A": ""}},
		{name: "值高字节允许（Go transport 自放行）", input: map[string]string{"X-A": "版本"}, want: map[string]string{"X-A": "版本"}},
		{name: "规范化后撞名拒绝（迭代序随机的唯一确定语义）", input: map[string]string{"x-app": "1", "X-App": "2"}, wantErr: true},
		{name: "Authorization/Cookie 不设黑名单照放（方案 §2）", input: map[string]string{"Authorization": "Bearer x", "Cookie": "a=b"}, want: map[string]string{"Authorization": "Bearer x", "Cookie": "a=b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sanitizeOutboundHeaders(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Fatalf("got[%q]=%q, want %q (full=%v)", k, got[k], v, got)
				}
			}
		})
	}
}
