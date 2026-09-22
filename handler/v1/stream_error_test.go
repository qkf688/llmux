package v1

import (
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
)

// TestWriteStreamErrorEvent_ErrorEventShapes 三种入站格式的 SSE error 事件形状断言
// （shapes 是客户端能否识别错误的关键契约，写死在测试里）。
func TestWriteStreamErrorEvent_ErrorEventShapes(t *testing.T) {
	cases := []struct {
		name     string
		style    string
		wantBody string
	}{
		{
			name:  "anthropic 出站 event:error + api_error",
			style: "anthropic",
			// 断流不是过载：overloaded_error 会让 Anthropic SDK 判为可重试并整请求重放，
			// 语义误导（审查回批 2026-09-23 修正）。
			wantBody: "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"api_error\"," +
				"\"message\":\"" + streamInterruptMessage + "\"}}\n\n",
		},
		{
			name:  "openai 出站 data 行 + server_error",
			style: "openai",
			wantBody: "data: {\"error\":{\"message\":\"" + streamInterruptMessage +
				"\",\"type\":\"server_error\",\"param\":null,\"code\":500}}\n\n",
		},
		{
			name:  "openai-res 出站 event:error + server_error",
			style: "openai-res",
			wantBody: "event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"server_error\"," +
				"\"message\":\"" + streamInterruptMessage + "\"}}\n\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			writeStreamErrorEvent(c, tc.style, streamInterruptMessage)

			if got := w.Body.String(); got != tc.wantBody {
				t.Fatalf("body =\n%s\nwant:\n%s", got, tc.wantBody)
			}
		})
	}
}

// TestWriteStreamErrorEvent_UnknownStyleBreaksStream 未知 style 兜底：仅断流不写事件。
func TestWriteStreamErrorEvent_UnknownStyleBreaksStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	writeStreamErrorEvent(c, "gemini", streamInterruptMessage)

	if got := w.Body.String(); got != "" {
		t.Fatalf("body = %q, want empty（未知 style 无协议可依）", got)
	}
}

// errReader 返回少量字节后失败，模拟上游中途断流。
type errReader struct{ err error }

func (r errReader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	return 0, io.EOF
}

// chunkThenErrReader 先给一段字节再失败。
type chunkThenErrReader struct {
	chunk []byte
	err   error
	done  bool
}

func (r *chunkThenErrReader) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		n := copy(p, r.chunk)
		return n, nil
	}
	return 0, r.err
}

// AC-5：提交后流式中断 → 协议内 error 事件收尾，绝不拼 JSON 500 信封污染 SSE 流。
func TestWriteResponseBody_MidStreamInterrupt_EmitsProtocolErrorEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	src := &chunkThenErrReader{chunk: []byte("data: chunk\n\n"), err: errors.New("boom")}
	err := writeResponseBody(c, string(consts.StyleOpenAI), true, src)
	if err == nil {
		t.Fatal("writeResponseBody err = nil, want 中断错误上抛")
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: chunk\n\n") {
		t.Fatalf("body 丢失已写字节: %q", body)
	}
	if !strings.Contains(body, `"error"`) {
		t.Fatalf("body 缺协议 error 事件: %q", body)
	}
	if strings.Contains(body, `{"code":500`) || strings.Contains(body, "internal server error") {
		t.Fatalf("JSON 500 信封污染了 SSE 流: %q", body)
	}
}

// AC-5 非流式半身：中断后静默截断，不再追加任何字节（尤其不追加 JSON 信封）。
func TestWriteResponseBody_NonStreamInterrupt_DoesNotAppendEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	src := &chunkThenErrReader{chunk: []byte(`{"id":"x"`), err: errors.New("boom")}
	err := writeResponseBody(c, string(consts.StyleOpenAI), false, src)
	if err == nil {
		t.Fatal("writeResponseBody err = nil, want 中断错误上抛")
	}

	body := w.Body.String()
	if body != `{"id":"x"` {
		t.Fatalf("body = %q, want 只有已写字节（静默截断）", body)
	}
	if strings.Contains(body, "internal server error") || strings.Contains(body, `{"code"`) {
		t.Fatalf("非流式中断被追加了信封: %q", body)
	}
}

// 非流式中断不在 handler 自留痕：唯一一条留痕归口日志管道（RecordLog 收口），
// handler 再自记即双份（Stage 18 口径），按行计数的告警会把一次故障算成两次。
func TestWriteResponseBody_NonStreamInterrupt_NoSelfLog(t *testing.T) {
	var logs strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	src := &chunkThenErrReader{chunk: []byte(`{"id":"x"`), err: errors.New("boom")}
	err := writeResponseBody(c, string(consts.StyleOpenAI), false, src)
	if err == nil {
		t.Fatal("writeResponseBody err = nil, want 中断错误上抛")
	}
	if !strings.Contains(err.Error(), "write response body failed") {
		t.Fatalf("err = %v, want 包含 write response body failed 上下文（供日志管道唯一留痕）", err)
	}
	if got := strings.TrimSpace(logs.String()); got != "" {
		t.Fatalf("handler 自留了日志（want 0 条，唯一留痕归口日志管道）:\n%s", got)
	}
}

// 读错误（非写错误）同样走收口分支。
func TestWriteResponseBody_ReadError_EmitsProtocolErrorEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	err := writeResponseBody(c, string(consts.StyleAnthropic), true, errReader{err: errors.New("read boom")})
	if err == nil {
		t.Fatal("writeResponseBody err = nil, want 读错误上抛")
	}
	if !strings.Contains(w.Body.String(), "event: error") {
		t.Fatalf("body 缺 anthropic error 事件: %q", w.Body.String())
	}
}
