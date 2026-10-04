package v1

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/service"
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

			if err := writeStreamErrorEvent(c, tc.style); err != nil {
				t.Fatalf("writeStreamErrorEvent err = %v", err)
			}

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

	if err := writeStreamErrorEvent(c, "gemini"); err != nil {
		t.Fatalf("writeStreamErrorEvent err = %v, want 未知 style 不是错误（只断流）", err)
	}

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
	// 无 deadline 的 ctx = 不设写上限，保持本组用例的原语义（写上限另有 write_deadline_test.go）。
	// src 用 service.MarkUpstreamReader 包一层与生产同形：生产里交给 writeResponseBody 的
	// body 是 io.TeeReader(MarkUpstreamReader(resBody), pw)，src 的读错误带「来自上游」标记。
	// 不包的话，这些「读错误」在测试里没有标记，会被 writeResponseBody 当成写错误盖上
	// 客户端侧标记——测的是一个生产里不存在的形态（Stage 39）。
	err := writeResponseBody(context.Background(), c, string(consts.StyleOpenAI), true, service.MarkUpstreamReader(src))
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
	err := writeResponseBody(context.Background(), c, string(consts.StyleOpenAI), false, service.MarkUpstreamReader(src))
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
	err := writeResponseBody(context.Background(), c, string(consts.StyleOpenAI), false, service.MarkUpstreamReader(src))
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

// 正常收尾（读取端以干净 EOF 结束）不是中断：交给 writeResponseBody 的 body 与生产同形
// （service.MarkUpstreamReader 包 reader，见上一条注释），此时**不得**返回错误、更不得给
// 客户端追加协议 error 事件。旧实现的后果按形态分两类：缺终止符的流与非流式整条被误判失败
// （非流式是 scanner 要再读一次才出 token，从而吃到管道错误；2026-10-05 实测），日志落
// `processer error: write response body failed: EOF`（真机库内该文案 14 条全部无 `[DONE]`）；
// 带 `[DONE]` 的流因 processer 提前收工仍记 success，
// 只是客户端在 `[DONE]` 之后多收一条 error 事件（停在 `[DONE]` 的客户端无感，读到 EOF 收尾
// 的客户端会误判整轮失败）。
// 流式与非流式**两半都要锁**：非流式走另一条判据链（日志侧 bufio.Scanner 的身份比较，
// process.go 的非流式分支），只测流式会漏掉那半条独立路径。
// 断言用 body 全等（而非「不含 error」）：多写任何字节都是污染。
func TestWriteResponseBody_CleanEOF_MarkedReaderNoErrorEvent(t *testing.T) {
	cases := []struct {
		name   string
		stream bool
		src    string
	}{
		{"流式·含 [DONE] 的完整流", true, "data: {\"x\":1}\n\ndata: [DONE]\n\n"},
		// 缺终止符的完整流现在算成功：这是用户批准的方案 A 取舍（内容完整性判据另立，见
		// .local/find-issues/2026-10-05-缺终止符与干净截断无内容判据.md）；该判据落地时
		// 本格要按新的内容口径翻转，不是「删掉即可」。
		{"流式·缺终止符的完整流", true, "data: {\"x\":1}\n\n"},
		{"非流式·完整 JSON 体", false, "{\"id\":\"x\",\"choices\":[]}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			err := writeResponseBody(context.Background(), c, string(consts.StyleOpenAI), tc.stream,
				service.MarkUpstreamReader(strings.NewReader(tc.src)))
			if err != nil {
				t.Fatalf("writeResponseBody err = %v, want nil（正常收尾不是失败）", err)
			}
			if got := w.Body.String(); got != tc.src {
				t.Fatalf("body = %q, want 原样 %q（不得追加 error 事件）", got, tc.src)
			}
		})
	}
}

// 读错误（非写错误）同样走收口分支。
func TestWriteResponseBody_ReadError_EmitsProtocolErrorEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	err := writeResponseBody(context.Background(), c, string(consts.StyleAnthropic), true,
		service.MarkUpstreamReader(errReader{err: errors.New("read boom")}))
	if err == nil {
		t.Fatal("writeResponseBody err = nil, want 读错误上抛")
	}
	if !strings.Contains(w.Body.String(), "event: error") {
		t.Fatalf("body 缺 anthropic error 事件: %q", w.Body.String())
	}
}
