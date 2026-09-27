package chat

import (
	"errors"
	"io"
	"sync"
	"time"
)

// errFirstByteWatchdog 是首字节看门狗触发时包装 reader 返回的哨兵错误。
// 提交门用它区分「上游头后无数据」与其它读错误：两者都判本次尝试失败，
// 但超时要归「凭据级网络」出口（冷却 + 组内换 key），分类依据即 errors.Is。
var errFirstByteWatchdog = errors.New("first byte watchdog: upstream returned no data after response headers")

// firstByteWatchdogReader 包裹流式上游响应体，盯「响应头到达后、首个数据字节前」
// 这个真空期：超过 timeout 仍无数据时关闭底层 body（解除阻塞中的 Read）并以
// errFirstByteWatchdog 中止。212 事故即死于此真空期——上游已回 200+SSE 头却一
// 字不发，网关没有任何超时会触发，客户端无限转圈直到自己断开。
//
// 计时起点 = 首次 Read（提交门在拿到上游响应后立即预读首帧，故等价于「响应头已
// 到达」）；等头耗时不计入（等头归 ResponseHeaderTimeout）。首个字节到达后看门狗
// 失效——只盯首帧，首帧后的停顿不杀（慢输出不算死）。
//
// 历史：本类型原在 handler/v1（响应头提交后才开始计时）；commit-before-write 落地后
// 首帧验证前移到 service/chat 提交门，「头后无数据」从「写完 200 才发现」提前到
// 「写客户端之前」，触发即换下一家重试而非仅报错收尾，故随之迁入本包。
type firstByteWatchdogReader struct {
	src     io.ReadCloser
	timeout time.Duration
	timer   *time.Timer // 首次 Read 时创建（AfterFunc），首帧到达或超时时停用

	startOnce sync.Once // 首次 Read 启动计时
	firstOnce sync.Once // 首帧到达后失效

	mu        sync.Mutex
	firstByte bool // 首帧已到达（abort 与 Read 竞态串行化用）
	timedOut  bool // 看门狗已触发
}

func newFirstByteWatchdogReader(src io.ReadCloser, timeout time.Duration) *firstByteWatchdogReader {
	return &firstByteWatchdogReader{src: src, timeout: timeout}
}

func (r *firstByteWatchdogReader) Read(p []byte) (int, error) {
	r.startOnce.Do(func() {
		t := time.AfterFunc(r.timeout, r.abort)
		r.mu.Lock()
		r.timer = t
		r.mu.Unlock()
	})

	n, err := r.src.Read(p)
	if n > 0 {
		r.firstOnce.Do(func() {
			// 首帧到达：置位 + disarm，与 abort 共用 mu 串行化。
			r.mu.Lock()
			r.firstByte = true
			if r.timer != nil {
				r.timer.Stop()
			}
			r.mu.Unlock()
		})
	}

	r.mu.Lock()
	timedOut := r.timedOut
	r.mu.Unlock()
	if timedOut {
		// 看门狗已触发：中止原因统一为哨兵错误（底层 src.Read 返回的是
		// Close 后的杂错，不一刀切会漏进其它错误分支）。
		return 0, errFirstByteWatchdog
	}
	return n, err
}

// Close 关闭底层 body 并 disarm 计时器：提交门预读失败出口（gateStreamCommitment
// 各失败分支的 watched.Close）都经由本方法收尾，timer 不 Stop 会悬挂到超时再 abort
// 一次底层 Close（双 Close 靠 http body 的「重复 Close 安全」隐式兜着），且整个超时
// 窗口内挂着一个会对已放弃的响应误伤的定时器。timer 字段读写全在 mu 下。
func (r *firstByteWatchdogReader) Close() error {
	r.mu.Lock()
	if r.timer != nil {
		r.timer.Stop()
	}
	r.mu.Unlock()
	return r.src.Close()
}

// abort 由 AfterFunc 在超时后调用：标记并关闭底层 body。
// 与 Read 的首帧到达存在竞态，用 r.mu 串行化——首帧已置位则放弃中止。竞态窗口里
// Read 已带回 n>0 但 firstOnce 尚未拿锁时 abort 先拿锁，已读字节会被丢弃：
// 方向安全（宁失败不提交），不承诺「窗口边界同时到达时保数据」。
func (r *firstByteWatchdogReader) abort() {
	r.mu.Lock()
	if r.firstByte {
		r.mu.Unlock()
		return
	}
	r.timedOut = true
	r.mu.Unlock()

	// 关闭底层 body：正在阻塞的 Read 立即返回，扫描随之退出。
	r.src.Close()
}
