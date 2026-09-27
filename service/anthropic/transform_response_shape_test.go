package anthropic

import (
	"strings"
	"testing"
)

// TestParseResponse_MalformedShapesRejected 锁 Stage 27 形状校验（此前解析过度宽容：
// `{"usage":"boom"}` 等形状非法的响应会拆出「全零/空」的零值成功，把坏供应商伪装成
// 可用——真实链路上 200 + 垃圾形状会被判成功、加分、继续接流量）。形状非法 = 报错，
// 失败回重试循环换下家。残余边界不变：`{"code":401,"message":…}` 这类「合法 JSON 对象
// 但语义是错误页」不在此挡（framework-conventions §6 既有口径），由提交门/客户端发现。
func TestParseResponse_MalformedShapesRejected(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "usage 类型错误", raw: `{"id":"m","model":"c","usage":"boom"}`},
		{name: "usage 是数组", raw: `{"id":"m","model":"c","usage":[1,2]}`},
		{name: "content 是数字", raw: `{"id":"m","model":"c","content":42}`},
		{name: "content 是字符串化对象", raw: `{"id":"m","model":"c","stop_reason":123}`},
		{name: "空对象", raw: `{}`},
		{name: "非对象（数组）", raw: `[1,2,3]`},
		{name: "非对象（字符串）", raw: `"ok"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := ParseResponse([]byte(tt.raw))
			if err == nil {
				t.Fatalf("ParseResponse(%s) succeeded: %+v, want error", tt.raw, unified)
			}
			if !strings.Contains(err.Error(), "anthropic") {
				t.Fatalf("err = %v, want 带 anthropic 前缀可归因", err)
			}
		})
	}
}
