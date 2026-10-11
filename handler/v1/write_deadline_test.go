package v1

import (
	"bufio"
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
// 变异自检验证——去掉 applyClientWriteDeadline 里的 SetWriteDeadline 调用（= 回到改动前）：
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
		// 方向标记：写上限钉的就是 ctx 的 deadline，故本用例的写失败**必然发生在总时限
		// 到点那一刻**，按 2026-10-04 拍板口径不算客户端侧（预算烧完算供应商的）→ 期望
		// **无**写侧标记。反向的「有标记」由 TestWriteResponseBody_ClientDisconnect_*
		// 锁（总时限未到的真断开）；两条都在才既锁住调用点没漏、又锁住到点那格归供应商。
		if service.IsClientWriteError(writeErr) {
			t.Fatalf("总时限到点的写失败被打上了写侧标记：%v（该格按拍板归供应商）", writeErr)
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

// TestWriteResponseBody_ClientDisconnect_MarkedAsClientSide 端到端锁「打标调用点没漏」：
// 客户端发完请求就断开连接（**总时限远未到**），handler 往客户端写大 body 时写失败，
// 该错误必须带上写侧方向标记——提交后失败判定的清钉 / 供应商失败统计 / 留痕级别三处
// 全靠它，漏打则「客户端跑路」会被记到供应商头上。
//
// 与 TestWriteResponseBody_ClientNotReading_CutOffAtTotalTimeout 互为反向，两条缺一不可：
//   - 那条的写失败发生在**总时限到点**（写上限钉的就是 ctx 的 deadline），按 2026-10-04
//     拍板口径归供应商，断言「无标记」；
//   - 本条的写失败发生在**总时限未到**，是真·客户端断开，断言「有标记」。
//
// 只留前者，把 writeResponseBody 里的 MarkResponseWriteError 整个摘掉也照样绿。
//
// 变异自检验证——去掉 writeResponseBody 里的 MarkResponseWriteError 调用：本行即红。
func TestWriteResponseBody_ClientDisconnect_MarkedAsClientSide(t *testing.T) {
	// 总时限给到 30s：远大于本用例耗时，确保写失败一定来自「客户端断开」而非预算到点。
	const budget = 30 * time.Second

	done := make(chan error, 1)
	router := gin.New()
	router.POST("/probe", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), budget)
		defer cancel()
		done <- writeResponseBody(ctx, c, string(consts.StyleOpenAI), false, endlessReader{})
	})
	addr := serveOnEphemeralPort(t, router)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if _, err := conn.Write([]byte("POST /probe HTTP/1.1\r\nHost: llmux\r\nContent-Length: 0\r\n\r\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}
	// 立刻断开：发完请求就 Close，收发缓冲里还压着未读数据 → 走 RST，
	// 服务端的下一次写立即失败。
	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case writeErr := <-done:
		if writeErr == nil {
			t.Fatal("写回返回 nil，want 客户端断开导致的写错误")
		}
		if !service.IsClientWriteError(writeErr) {
			t.Fatalf("客户端断开（总时限未到）缺写侧方向标记：%v", writeErr)
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("客户端断开后 8s 内写回未收场：%v", budget)
	}
}

// TestWriteResponseBody_KeepAliveConnUsableAfterDeadlineRequest 锁 keep-alive 复用：
// 一次带写上限的请求走完后，同一条连接服务下一个请求时**不带着任何残留上限**。
//
// 关键设计：第二个请求的 handler 刻意不挂总时限（其 ctx 无 deadline → 不设上限），于是
// 它的写能否成功只取决于连接上有没有残留的过期上限——这正是本用例要锁的对象。若哪天
// net/http 的 server 实现不再在请求收尾清上限，残留的过期上限会让第二次的写立即失败
// （实测：过期上限下连 5 字节的写都直接 i/o timeout），本用例即红。
//
// 响应体刻意取**小** body：过期上限对写的作用发生在进入 syscall 之前（netpoll 先判
// deadline，与写多少字节、会不会阻塞无关），不需要把 16 MiB 写满发送缓冲。早先那版把
// 两次请求绑同一个短上限 + 16 MiB body，制造了两条假前提（「小响应测不出」「上限要用大
// body 才显形」）并让**第二次请求自己的上限**在慢环境里到点——CI 上那条偶发
// `unexpected EOF` 与「上限没清干净」无关，是它自己超时。
//
// 变异自检验证（区分度）：把第二个请求的 handler 改成挂一个**已过期**的 deadline
// （等价于连接上带着过期上限），本用例红。
// 边界须知：单独去掉生产侧 `applyClientWriteDeadline` 的 `SetWriteDeadline`，本用例
// **不会**红——此时 server 收尾的清理仍兜着，复用行为没有真坏；它锁的是两侧同时退步。
func TestWriteResponseBody_KeepAliveConnUsableAfterDeadlineRequest(t *testing.T) {
	const limit = 300 * time.Millisecond
	// 第一次的响应体：小到不会阻塞写，只用来构成「一次带上限的真实请求」。
	const firstBody = "ok"
	// 第二次的响应体：中等大小，覆盖 io.Copy 的分块写路径，又远小于任何合理上限。
	secondBody := strings.Repeat("y", 64<<10)

	// 写回错误经通道回测试 goroutine 再断言：testing 规定测试结束后再 Log 会 panic。
	// 写上限真没清干净时 handler 会报错，而此刻下面的主断言往往已先 t.Fatalf 收场，
	// 把 t.Errorf 留在 handler goroutine 里就会 panic 盖掉原本干净的失败信息。
	handlerErrs := make(chan error, 2)
	t.Cleanup(func() {
		for {
			select {
			case err := <-handlerErrs:
				t.Errorf("写回失败: %v", err)
			default:
				return
			}
		}
	})

	var callCount atomic.Int32
	router := gin.New()
	router.POST("/probe", func(c *gin.Context) {
		// 第一次：模拟生产入口（总时限挂在请求 ctx 上）；第二次：刻意不带总时限，
		// 让「连接上是否残留过期上限」成为唯一的成败因素（见用例注释「关键设计」）。
		nth := callCount.Add(1)
		ctx := c.Request.Context()
		cancel := func() {}
		src := io.Reader(strings.NewReader(firstBody))
		if nth > 1 {
			src = strings.NewReader(secondBody)
		} else {
			ctx, cancel = context.WithTimeout(c.Request.Context(), limit)
		}
		defer cancel()
		if err := writeResponseBody(ctx, c, string(consts.StyleOpenAI), false, src); err != nil {
			handlerErrs <- err
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
			t.Fatalf("第 %d 次请求读不到响应（连接上残留了过期写上限?）: %v", i, err)
		}
		got, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("第 %d 次响应体读失败（连接上残留了过期写上限?）: %v", i, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("第 %d 次请求状态码 = %d, want 200", i, resp.StatusCode)
		}
		wantLen := len(firstBody)
		if i == 2 {
			wantLen = len(secondBody)
		}
		if len(got) != wantLen {
			t.Fatalf("第 %d 次响应体 = %d 字节, want %d（截断说明连接上残留了过期写上限）", i, len(got), wantLen)
		}
	}
}
