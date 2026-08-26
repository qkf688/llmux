package v1

import (
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestFirstByteWatchdog_FirstByteWithinWindow 窗口内上游出首帧 → 正常透传、
// 看门狗不触发（严格断言经 errors.Is 无哨兵错误）。
func TestFirstByteWatchdog_FirstByteWithinWindow(t *testing.T) {
	pr, pw := io.Pipe()
	wd := newFirstByteWatchdogReader(pr, time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		pw.Write([]byte("data: hello\n\n"))
		pw.Close()
	}()

	var n int
	var readErr error
	for {
		buf := make([]byte, 1024)
		n, readErr = wd.Read(buf)
		if n > 0 {
			t.Logf("read %d bytes", n)
		}
		if readErr != nil {
			break
		}
	}
	<-done

	if readErr != io.EOF {
		t.Fatalf("read err = %v, want io.EOF（看门狗不应触发）", readErr)
	}
	if wd.TimedOut() {
		t.Fatal("TimedOut() = true, want false（首帧在窗口内到达）")
	}
}

// TestFirstByteWatchdog_TimingStartsAtFirstRead 计时起点 = 首次 Read 而非构造：
// 构造后 sleep 超过窗口再 Read，不应超时（等头耗时不计入窗口，AC-2）。
func TestFirstByteWatchdog_TimingStartsAtFirstRead(t *testing.T) {
	const window = 50 * time.Millisecond
	wd := newFirstByteWatchdogReader(io.NopCloser(strings.NewReader("abc")), window)

	time.Sleep(window * 2) // 模拟"头到达后、io.Copy 启动前"的间隙

	buf := make([]byte, 3)
	n, err := wd.Read(buf)
	if err != nil {
		t.Fatalf("Read err = %v, want nil（计时应从首次 Read 起算，constructed->Read 的间隔不计入）", err)
	}
	if n != 3 {
		t.Fatalf("Read n = %d, want 3", n)
	}
	if wd.TimedOut() {
		t.Fatal("TimedOut() = true, want false")
	}
}

// TestFirstByteWatchdog_TimeoutAbortsRead 上游头后不写任何数据 → Read 在窗口
// 超时后返回 errFirstByteWatchdog（而非无限阻塞），底层 src 被关闭。
func TestFirstByteWatchdog_TimeoutAbortsRead(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close() })
	wd := newFirstByteWatchdogReader(pr, 50*time.Millisecond)

	type result struct {
		n   int
		err error
	}
	readResult := make(chan result, 1)
	go func() {
		buf := make([]byte, 1024)
		n, err := wd.Read(buf)
		readResult <- result{n, err}
	}()

	select {
	case res := <-readResult:
		if !errors.Is(res.err, errFirstByteWatchdog) {
			t.Fatalf("Read err = %v, want errFirstByteWatchdog", res.err)
		}
		if res.n != 0 {
			t.Fatalf("Read n = %d, want 0", res.n)
		}
		if !wd.TimedOut() {
			t.Fatal("TimedOut() = false, want true")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Read 未在窗口超时后返回，看门狗未中止阻塞读")
	}
}

// TestFirstByteWatchdog_ErrorEventShapes 三种入站格式的 SSE error 事件形状断言
// （shapes 是客户端能否识别错误的关键契约，写死在测试里）。
func TestFirstByteWatchdog_ErrorEventShapes(t *testing.T) {
	cases := []struct {
		name     string
		style    string
		wantBody string
	}{
		{
			name:  "anthropic 出站 event:error + overloaded_error",
			style: "anthropic",
			wantBody: "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\"," +
				"\"message\":\"upstream returned no data after the response headers (first-byte watchdog)\"}}\n\n",
		},
		{
			name:  "openai 出站 data 行 + server_error",
			style: "openai",
			wantBody: "data: {\"error\":{\"message\":\"upstream returned no data after the response headers " +
				"(first-byte watchdog)\",\"type\":\"server_error\",\"param\":null,\"code\":500}}\n\n",
		},
		{
			name:  "openai-res 出站 event:error + server_error",
			style: "openai-res",
			wantBody: "event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"server_error\"," +
				"\"message\":\"upstream returned no data after the response headers (first-byte watchdog)\"}}\n\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			writeStreamErrorEvent(c, tc.style)

			if got := w.Body.String(); got != tc.wantBody {
				t.Fatalf("body =\n%s\nwant:\n%s", got, tc.wantBody)
			}
		})
	}
}

// TestFirstByteWatchdog_UnknownStyleBreaksStream 未知 style 兜底：仅断流不写事件。
func TestFirstByteWatchdog_UnknownStyleBreaksStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	writeStreamErrorEvent(c, "gemini")

	if got := w.Body.String(); got != "" {
		t.Fatalf("body = %q, want empty（未知 style 无协议可依）", got)
	}
}
