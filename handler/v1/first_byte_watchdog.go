package v1

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/consts"
)

// errFirstByteWatchdog 是首字节看门狗触发时包装 reader 返回的哨兵错误。
// handler 用它区分「上游头后无数据」与其它读错误：前者响应头（200/SSE）已提交，
// 只能写协议 error 事件收尾；后者保持既有 InternalServerError 行为。
var errFirstByteWatchdog = errors.New("first byte watchdog: upstream returned no data after response headers")

// firstByteWatchdogReader 包裹流式上游响应体，盯「响应头到达后、首个数据字节前」
// 这个真空期：超过 timeout 仍无数据时关闭底层 body（解除阻塞中的 Read）并以
// errFirstByteWatchdog 中止。212 事故即死于此真空期——上游已回 200+SSE 头却一
// 字不发，网关没有任何超时会触发，客户端无限转圈直到自己断开。
//
// 计时起点 = 首次 Read（handler 中 io.Copy 在 writeHeader 之后才启动，故等价于
// 「响应头已提交给客户端」）；等头耗时不计入（等头归 ResponseHeaderTimeout）。
// 首个字节到达后看门狗失效——只盯首帧，首帧后的停顿不杀（慢输出不算死）。
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
		r.timer = time.AfterFunc(r.timeout, r.abort)
	})

	n, err := r.src.Read(p)
	if n > 0 {
		r.firstOnce.Do(func() {
			r.mu.Lock()
			r.firstByte = true
			r.mu.Unlock()
			r.timer.Stop()
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

func (r *firstByteWatchdogReader) Close() error {
	return r.src.Close()
}

// abort 由 AfterFunc 在超时后调用：标记并关闭底层 body。
// 与 Read 的首帧到达存在竞态，用 r.mu 串行化——首帧先到则放弃中止
// （窗口边界恰好同时到达时优先保数据）。
func (r *firstByteWatchdogReader) abort() {
	r.mu.Lock()
	if r.firstByte {
		r.mu.Unlock()
		return
	}
	r.timedOut = true
	r.mu.Unlock()

	// 关闭底层 body：正在阻塞的 Read 立即返回，io.Copy 随之退出。
	r.src.Close()
}

// TimedOut 报告看门狗是否已触发（供 handler 分流错误处理）。
func (r *firstByteWatchdogReader) TimedOut() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.timedOut
}

// writeStreamErrorEvent 在响应头（200/SSE）已提交后向客户端写出该协议形状的
// SSE error 事件收尾。仅在看门狗触发路径调用——此时状态码已无法改写，
// httpresp.InternalServerError 的 JSON 会不带 SSE 前缀原样污染已提交的流
// （客户端的 SSE 解析器会把它当垃圾数据），协议内错误事件才能被识别为失败。
// 三种入站格式的形状各不相同：Anthropic / Responses 用 event: error 行，
// OpenAI Chat 的流式错误是 data: 行 + error 对象，两者都无终止事件跟随
// （错误即终，客户端解析到 error 即停止）。
func writeStreamErrorEvent(c *gin.Context, style string) {
	message := "upstream returned no data after the response headers (first-byte watchdog)"
	switch consts.Style(style) {
	case consts.StyleAnthropic:
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n",
			`{"type":"error","error":{"type":"overloaded_error","message":"`+message+`"}}`)
	case consts.StyleOpenAI:
		fmt.Fprintf(c.Writer, "data: %s\n\n",
			`{"error":{"message":"`+message+`","type":"server_error","param":null,"code":500}}`)
	case consts.StyleOpenAIRes:
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n",
			`{"type":"error","error":{"code":"server_error","message":"`+message+`"}}`)
	default:
		// 未知 style：无协议可依，仅断流。
	}
	c.Writer.Flush()
}
