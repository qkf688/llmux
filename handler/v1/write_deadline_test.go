package v1

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/service"
)

// endlessReader 永不 EOF：把 io.Copy 逼进「一直写」状态。客户端不读时 socket 发送
// 缓冲总有上限，写必然卡在缓冲满的那一刻——这就是「客户端不收结果」的现场。
type endlessReader struct{}

func (endlessReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

// serveOnEphemeralPort 让被测 handler 跑在真实 net/http 连接上。刻意不用
// httptest.NewRecorder：它的 Write 写内存恒成功、永不阻塞，测不出写上限；也不用
// httptest.NewServer：那自带一份 Config，我们改字段就绕开了被测的生产装配。
func serveOnEphemeralPort(t *testing.T, h http.Handler) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: h}
	go func() { _ = srv.Serve(ln) }()
	// conn 由调用方 defer Close：它先于 cleanup 触发，客户端断开能让还卡在 Write 里的
	// handler 返回，本 srv.Close() 才不会挂住（顺序反了会把「用例失败」变成「测试挂死」）。
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

// TestWriteResponseBody_ClientNotReading_CutOffAtTotalTimeout 是本 Stage 的核心用例：
// 客户端发完请求就再不读响应，服务端必须在该请求的总时限内收场。
//
// 变异自检验证——去掉 setClientWriteDeadline 里的 SetWriteDeadline 调用（= 回到改动前）：
//   - io.Copy 永久阻塞在 socket 缓冲满的那一刻 → 8s 兜底到点 → 红。
//
// 这是「写上限真的挂在写路径上」的证据：recorder 形态下 io.Copy 恒成功，用例会假绿。
func TestWriteResponseBody_ClientNotReading_CutOffAtTotalTimeout(t *testing.T) {
	const limit = 400 * time.Millisecond

	done := make(chan error, 1)
	router := gin.New()
	router.POST("/probe", func(c *gin.Context) {
		// 与生产同源：总时限挂在请求 ctx 的 deadline 上，写上限从它派生。
		ctx, cancel := context.WithTimeout(c.Request.Context(), limit)
		defer cancel()
		done <- writeResponseBody(ctx, c, string(consts.StyleOpenAI), false, endlessReader{})
	})
	addr := serveOnEphemeralPort(t, router)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 只发请求，之后既不读响应也不断开——真实的「客户端不收结果」。
	if _, err := conn.Write([]byte("POST /probe HTTP/1.1\r\nHost: llmux\r\nContent-Length: 0\r\n\r\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}

	start := time.Now()
	select {
	case writeErr := <-done:
		elapsed := time.Since(start)
		if writeErr == nil {
			t.Fatal("写回返回 nil，want 被写上限掐断的错误")
		}
		var netErr net.Error
		if !errors.As(writeErr, &netErr) || !netErr.Timeout() {
			t.Fatalf("写回错误不是超时：%v（want 写上限到点的超时）", writeErr)
		}
		// 方向标记是提交后失败判定的唯一依据（不清粘性钉定 / 不记供应商失败统计 /
		// 留痕降 Warn 全靠它）。漏打则这类失败会被算到供应商头上——去掉
		// writeResponseBody 里的 MarkResponseWriteError，本行即红。
		if !service.IsClientWriteError(writeErr) {
			t.Fatalf("写回错误缺写侧方向标记：%v（上游读错误才该放过，写错误必须标记）", writeErr)
		}
		// 下限取上限的 1/4：排除「连接被立刻关掉」这种与上限无关的收场；取 1/4 而非
		// 紧贴上限，避开 Windows 单调钟 tick 步进。
		if lower := limit / 4; elapsed < lower {
			t.Fatalf("写回在 %v 就失败，短于下限 %v，want 与写上限（%v）同源", elapsed, lower, limit)
		}
		if upper := 5 * time.Second; elapsed > upper {
			t.Fatalf("写回在 %v 才收场，want 上限 %v 附近", elapsed, limit)
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("写回 8s 内未收场：客户端不读时 io.Copy 永久阻塞，写上限（%v）没生效", limit)
	}
}

// TestWriteResponseBody_KeepAliveConnUsableAfterDeadlineRequest 锁 keep-alive 复用：
// 一次带写上限的请求走完后，同一条连接还能正常服务下一个请求。写上限落在**连接**上，
// 若请求收尾时没人清，同连接的下一个请求会带着一个已过期的写上限发不出去（socket
// deadline 一旦过期不可再延长）。
//
// 这里断言的不是「我们自己清了」——net/http 在每个请求收尾自己清（server.go 的
// `conn.serve`：`w.finishRequest()` 紧跟 `c.rwc.SetWriteDeadline(time.Time{})`），
// 本项目曾为此写了一段手动清理，是多余实现、已删。本用例改为守住**这个行为**：谁哪天
// 换掉 server 实现或改走 HTTP/2 而库不再清，它会红。
//
// 第二次的响应体必须大到能把 socket 发送缓冲填满，否则测不出：socket deadline 只在
// 操作**会阻塞**时才生效（写得进缓冲就立刻成功），2 字节的响应即使上限早已过期也
// 照样发得出去——曾据此写出过一版「第二次请求拿到 200」的假绿用例，变异检验时没咬住。
func TestWriteResponseBody_KeepAliveConnUsableAfterDeadlineRequest(t *testing.T) {
	const limit = 300 * time.Millisecond
	// 16 MiB：必须超过系统可能自动调大的 socket 发送缓冲（Linux tcp_wmem 封顶 4 MiB），
	// 写才会真的阻塞、已过期的上限才会显形。
	bigBody := bytes.Repeat([]byte("x"), 16<<20)

	var callCount atomic.Int32
	router := gin.New()
	router.POST("/probe", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), limit)
		defer cancel()
		src := io.Reader(strings.NewReader("ok"))
		if callCount.Add(1) > 1 {
			src = bytes.NewReader(bigBody) // 第二次：大到填满发送缓冲
		}
		if err := writeResponseBody(ctx, c, string(consts.StyleOpenAI), false, src); err != nil {
			t.Errorf("写回失败: %v", err)
		}
	})
	addr := serveOnEphemeralPort(t, router)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for i := 1; i <= 2; i++ {
		if i == 2 {
			time.Sleep(limit + 200*time.Millisecond) // 跨过第一次的写上限
		}
		req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/probe", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		if err := req.Write(conn); err != nil {
			t.Fatalf("第 %d 次请求发送失败: %v", i, err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		resp, err := http.ReadResponse(reader, req)
		if err != nil {
			t.Fatalf("第 %d 次请求读不到响应（写上限没清干净?）: %v", i, err)
		}
		got, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("第 %d 次响应体读失败（写上限没清干净?）: %v", i, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("第 %d 次请求状态码 = %d, want 200", i, resp.StatusCode)
		}
		wantLen := 2
		if i == 2 {
			wantLen = len(bigBody)
		}
		if len(got) != wantLen {
			t.Fatalf("第 %d 次响应体 = %d 字节, want %d（截断说明写上限没清干净）", i, len(got), wantLen)
		}
	}
}
