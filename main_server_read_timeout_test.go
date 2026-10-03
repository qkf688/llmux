package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// 读方向上限的装配值锁（Stage 39，R4/R5）：http.Server 的两个读超时零值即「永不超时」，
// 一旦漏配（或被改成 0），客户端连上不吭声就能一直占住连接。这半条锁**字面值**而非
// 「等于具名常量」——后者会与实现同源：把常量一并改成 0 时断言仍成立，扣不出这类退化。
// 单测里不能真等 5s/60s，故只断产出值。
func TestNewHTTPServer_SetsReadDirectionLimits(t *testing.T) {
	srv := newHTTPServer("127.0.0.1:0", http.NotFoundHandler())

	if want := 5 * time.Second; srv.ReadHeaderTimeout != want {
		t.Fatalf("ReadHeaderTimeout = %v, want %v（读头上限未装配或被置 0）", srv.ReadHeaderTimeout, want)
	}
	if want := 60 * time.Second; srv.ReadTimeout != want {
		t.Fatalf("ReadTimeout = %v, want %v（整个请求读完的上限未装配或被置 0）", srv.ReadTimeout, want)
	}
}

// serveOnEphemeralPort 让生产构造函数产出的 *http.Server 真监听一个随机端口。
// 不用 httptest.NewServer：它自带一份 Config，我们改字段就绕开了生产装配路径。
func serveOnEphemeralPort(t *testing.T, srv *http.Server) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

// assertServerClosedStalledConn 断言「连接被服务端在上限内收场」：
//   - 读回错误（连接已关），不是仍在等；
//   - 没收到任何 HTTP 响应（收到就说明是被 400 之类拒了，不是超时收场——同形但不同因）；
//   - 耗时落在 [下限, 上限带余量] 区间，排除「立刻关」和「根本没触发」。
func assertServerClosedStalledConn(t *testing.T, conn net.Conn, sentAt time.Time, limit time.Duration) {
	t.Helper()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 512)
	n, readErr := conn.Read(buf)
	elapsed := time.Since(sentAt)

	if readErr == nil {
		t.Fatalf("连接仍未被收场，收到 %d 字节：%q", n, buf[:n])
	}
	if bytes.HasPrefix(buf[:n], []byte("HTTP/")) {
		t.Fatalf("服务端先回了 HTTP 响应（%q），want 静默收场", buf[:n])
	}
	// 下限取上限的 1/4：排除「连接被立刻关掉」这种与上限无关的收场。取 1/4 而非
	// 紧贴上限，避开 Windows 单调钟 tick 步进导致的边界抖动。
	if lower := limit / 4; elapsed < lower {
		t.Fatalf("连接在 %v 就被收场，短于下限 %v，want 与读超时上限（%v）同源", elapsed, lower, limit)
	}
	// 余量给 4s 级别：CI/Windows 上 net/http 收连接调度可能滞后，只卡「不得无限等」。
	if upper := 4 * time.Second; elapsed > upper {
		t.Fatalf("连接在 %v 才被收场，超过余量上限 %v，want 上限 %v 附近", elapsed, upper, limit)
	}
}

// 读头（连上来先说的那几行）发一半就停住：handler 根本没被调用，服务端到点静默收场。
func TestHTTPServer_ReadHeaderTimeout_ClosesStalledHeader(t *testing.T) {
	const testLimit = 300 * time.Millisecond

	srv := newHTTPServer("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("请求头都没读完，handler 不该被调用")
	}))
	srv.ReadHeaderTimeout = testLimit // 测试内缩短，不改生产常量
	addr := serveOnEphemeralPort(t, srv)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 残缺头：没有结束空行，Content-Length 也不给 → 服务端继续等。
	if _, err := conn.Write([]byte("POST /v1/chat/completions HTTP/1.1\r\nHost: llmux\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	assertServerClosedStalledConn(t, conn, time.Now(), testLimit)
}

// 头写完整、请求体只发一半就停住：handler 被正常调用（net/http 不预读请求体，头读完
// 就进 handler），但它读请求体的那次 Read 到点被掐断——生产里 chatHandler 正是这样
// 整读请求体（io.ReadAll(c.Request.Body)），所以这条上限兜住的是「handler 卡在读体」。
func TestHTTPServer_ReadTimeout_BoundsHandlerBodyRead(t *testing.T) {
	const testLimit = 400 * time.Millisecond

	bodyRead := make(chan error, 1)
	srv := newHTTPServer("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		bodyRead <- err
		w.WriteHeader(http.StatusBadRequest)
	}))
	srv.ReadTimeout = testLimit // 读头用生产值（请求头毫秒级写完，够）
	addr := serveOnEphemeralPort(t, srv)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 声明一个大 body 却只发开头：handler 会一直等剩下的字节。
	start := time.Now()
	if _, err := conn.Write([]byte("POST /v1/chat/completions HTTP/1.1\r\nHost: llmux\r\n" +
		"Content-Type: application/json\r\nContent-Length: 1048576\r\n\r\n" +
		`{"model":"gpt-4o-mini","messages":[`)); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case readErr := <-bodyRead:
		if readErr == nil {
			t.Fatal("handler 的读体返回 nil，want 到点被掐断的错误")
		}
		elapsed := time.Since(start)
		if lower := testLimit / 4; elapsed < lower {
			t.Fatalf("读体在 %v 就失败了，短于下限 %v，want 与读超时上限（%v）同源", elapsed, lower, testLimit)
		}
		if upper := 4 * time.Second; elapsed > upper {
			t.Fatalf("读体在 %v 才失败，超过余量上限 %v，want 上限 %v 附近", elapsed, upper, testLimit)
		}
	case <-time.After(6 * time.Second):
		t.Fatalf("handler 的读体 6s 内仍未返回，读请求体的上限（want %v）没生效", testLimit)
	}
}
