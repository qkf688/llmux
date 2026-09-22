package chat

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
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

	var readErr error
	for {
		buf := make([]byte, 1024)
		_, readErr = wd.Read(buf)
		if readErr != nil {
			break
		}
	}
	<-done

	if readErr != io.EOF {
		t.Fatalf("read err = %v, want io.EOF（看门狗不应触发）", readErr)
	}
}

// TestFirstByteWatchdog_TimingStartsAtFirstRead 计时起点 = 首次 Read 而非构造：
// 构造后 sleep 超过窗口再 Read，不应超时（等头耗时不计入窗口）。
func TestFirstByteWatchdog_TimingStartsAtFirstRead(t *testing.T) {
	const window = 50 * time.Millisecond
	wd := newFirstByteWatchdogReader(io.NopCloser(strings.NewReader("abc")), window)

	time.Sleep(window * 2) // 模拟"头到达后、预读启动前"的间隙

	buf := make([]byte, 3)
	n, err := wd.Read(buf)
	if err != nil {
		t.Fatalf("Read err = %v, want nil（计时应从首次 Read 起算，constructed->Read 的间隔不计入）", err)
	}
	if n != 3 {
		t.Fatalf("Read n = %d, want 3", n)
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
	case <-time.After(2 * time.Second):
		t.Fatal("Read 未在窗口超时后返回，看门狗未中止阻塞读")
	}
}
