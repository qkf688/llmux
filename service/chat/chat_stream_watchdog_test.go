package chat

import (
	"errors"
	"io"
	"strings"
	"sync/atomic"
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

// failReadCloser Read 恒失败的 ReadCloser（模拟预读失败路径：首帧从未到达）。
type failReadCloser struct {
	closes atomic.Int32
}

func (f *failReadCloser) Read([]byte) (int, error) { return 0, errors.New("read boom") }

func (f *failReadCloser) Close() error {
	f.closes.Add(1)
	return nil
}

// TestFirstByteWatchdog_CloseDisarmsTimer 预读失败出口（首帧前 Close）必须 disarm 计时器：
// 不 Stop 则 timer 悬挂到超时后 abort 二次 Close 底层（双 Close 只靠 http body「重复
// Close 安全」隐式兜着），超时窗口内还挂着会对已放弃响应误伤的定时器。改前实测红：
// 旧 Close 不触碰 timer，窗口过后 abort 会再 Close 一次（计数=2）。
func TestFirstByteWatchdog_CloseDisarmsTimer(t *testing.T) {
	src := &failReadCloser{}
	wd := newFirstByteWatchdogReader(src, 50*time.Millisecond)

	// 首次 Read 启动计时并以读错误返回（预读失败形态），随后走失败出口的 Close。
	_, _ = wd.Read(make([]byte, 1))
	if err := wd.Close(); err != nil {
		t.Fatalf("Close err = %v", err)
	}

	time.Sleep(150 * time.Millisecond) // 越过窗口：timer 若未 disarm，abort 已二次 Close

	if got := src.closes.Load(); got != 1 {
		t.Fatalf("底层 Close 次数 = %d, want 1（失败出口必须 disarm 计时器）", got)
	}
}

// TestGateStreamCommitment_FailureExitDisarmsWatchdog 真实调用链回归（2026-09-28 审查回批）：
// 提交门预读失败出口必须走 watched.Close()（disarm + 关底层）——只关 capture 碰不到
// timer，上面的单测直调 wd.Close() 锁不住生产路径。改前实测红：旧出口 capture.Close()，
// 超时后 abort 二次 Close（计数=2）。
func TestGateStreamCommitment_FailureExitDisarmsWatchdog(t *testing.T) {
	src := &failReadCloser{}
	if _, err := gateStreamCommitment(src, 50*time.Millisecond); err == nil {
		t.Fatal("gateStreamCommitment err = nil, want 预读失败上抛")
	}

	time.Sleep(150 * time.Millisecond) // 越过窗口：timer 若未 disarm，abort 已二次 Close

	if got := src.closes.Load(); got != 1 {
		t.Fatalf("底层 Close 次数 = %d, want 1（失败出口必须经 watched.Close disarm 计时器）", got)
	}
}
