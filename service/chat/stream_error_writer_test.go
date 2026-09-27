package chat

import (
	"bytes"
	"testing"

	"github.com/qkf688/llmux/consts"
)

// 断流错误帧字节形状是客户端 SDK 契约：三种 style 逐字锁定（与 handler 层
// stream_error_test.go 的端到端形状断言同源），防注册面重构悄悄改形状。
func TestStreamErrorWriter_FrameShapes(t *testing.T) {
	const msg = "upstream stream interrupted before completion"
	cases := []struct {
		name     string
		style    string
		wantBody string
	}{
		{
			name:  "anthropic 出站 event:error + api_error",
			style: "anthropic",
			wantBody: "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"api_error\"," +
				"\"message\":\"" + msg + "\"}}\n\n",
		},
		{
			name:  "openai 出站 data 行 + server_error",
			style: "openai",
			wantBody: "data: {\"error\":{\"message\":\"" + msg +
				"\",\"type\":\"server_error\",\"param\":null,\"code\":500}}\n\n",
		},
		{
			name:  "openai-res 出站 event:error + server_error",
			style: "openai-res",
			wantBody: "event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"server_error\"," +
				"\"message\":\"" + msg + "\"}}\n\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, err := GetStreamErrorWriter(tc.style)
			if err != nil {
				t.Fatalf("GetStreamErrorWriter(%q): %v", tc.style, err)
			}
			got, err := w(msg)
			if err != nil {
				t.Fatalf("writer err = %v", err)
			}
			if !bytes.Equal(got, []byte(tc.wantBody)) {
				t.Fatalf("frame =\n%s\nwant:\n%s", got, tc.wantBody)
			}
		})
	}
}

// message 含引号/换行时帧内 JSON 必须转义（typed marshal 兜底；字符串拼接造 JSON
// 时代的注入面回归锁——转义失效会把帧撕成多行，客户端解析器收到垃圾）。
func TestStreamErrorWriter_MessageEscaped(t *testing.T) {
	w, err := GetStreamErrorWriter(string(consts.StyleOpenAI))
	if err != nil {
		t.Fatalf("GetStreamErrorWriter: %v", err)
	}
	got, err := w("he said \"hi\"\nbye")
	if err != nil {
		t.Fatalf("writer err = %v", err)
	}
	if !bytes.Contains(got, []byte(`he said \"hi\"\nbye`)) {
		t.Fatalf("message 未被 JSON 转义: %q", got)
	}
	if bytes.Contains(got, []byte("he said \"hi\"\nbye")) {
		t.Fatalf("message 原文（含裸换行）漏进帧: %q", got)
	}
	if n := bytes.Count(got, []byte("\n")); n != 2 {
		t.Fatalf("帧内换行数 = %d, want 2（data 行尾 + 帧分隔空行）: %q", n, got)
	}
}

// 注册完备性机械锁：凡注册了 Beforer 的 style 必须注册 StreamErrorWriter——
// 新 style 漏登记即红，不再靠人记得补 handler 侧分支（Stage 30 W5 的收口判据）。
func TestRegistry_StreamErrorWriterCoversAllStyles(t *testing.T) {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()
	for style := range defaultRegistry.beforeers {
		if _, ok := defaultRegistry.streamErrorWriters[style]; !ok {
			t.Errorf("style %q 注册了 Beforer 但缺 StreamErrorWriter（断流错误帧将退化为只断流）", style)
		}
	}
}

// 注册表重复注册必须 panic（AGENTS §5 纪律）。
func TestRegisterStreamErrorWriter_DuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.RegisterStreamErrorWriter("x", streamErrorWriterOpenAI)
	defer func() {
		if recover() == nil {
			t.Fatal("重复注册 StreamErrorWriter 未 panic")
		}
	}()
	r.RegisterStreamErrorWriter("x", streamErrorWriterOpenAI)
}

// 未知 style：查表失败（handler 侧语义 = 只断流不写字节）。
func TestGetStreamErrorWriter_UnknownStyle(t *testing.T) {
	if _, err := GetStreamErrorWriter("gemini"); err == nil {
		t.Fatal("GetStreamErrorWriter(未知 style) err = nil, want 报错")
	}
}
