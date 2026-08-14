package streaming

import (
	"fmt"
	"strings"
)

// rawSSEAccumulator 以「头段固定 + 尾段滚动」的方式累积语义等价的 SSE 事件序列，
// 供日志 RawResponseBody 使用。头段写满后封口，后续事件进入尾段滚动缓冲（只保留
// 最后 rawAccumulatorTailSize 字节），被挤出的中间部分记入 midDropped。
//
// 为什么保留头尾而非只保留头部：SSE 流的关键收尾事件（finish_reason / usage /
// [DONE] / message_stop）都在末尾，纯头部截断会恰好丢掉排查时最需要的信息。
// finalize() 在头尾之间插入截断标记，使 DB / 前端可区分「上游就这么短」与「被截断」。
//
// 注意累积结果不是字节级保真：多行 data 已在上游被合成单行（既有行为），
// 因此不要用于与上游原始字节逐字节比对，只作诊断用。
//
// 本类型不做 IO（不打日志、不落库），截断信号由调用方在 finalize 边界发出。
// 仅在单个转换 goroutine 内写入，写入结束后（pipe Close）才由调用方读取，无并发。
type rawSSEAccumulator struct {
	head       []byte
	headFull   bool
	tail       []byte
	midDropped int
}

// write 追加一个已格式化的 SSE 事件（含 event/data 行与结尾空行）。
//
// 头段采用整块决策：装不下就整个事件转入尾段，让头段停在完整事件边界、可被独立
// 解析，代价是头段末尾最多空出一个事件的长度。
func (a *rawSSEAccumulator) write(chunk string) {
	if !a.headFull {
		if len(a.head)+len(chunk) <= rawAccumulatorHeadSize {
			a.head = append(a.head, chunk...)
			return
		}
		a.headFull = true
	}

	a.tail = append(a.tail, chunk...)
	// 摊还裁剪：攒到 2 倍预算才搬移一次。若每次写入都精确裁剪，稳态下每个小 chunk
	// 都要触发一次 tailSize 量级的 memmove，长流上是 O(流长 × tailSize) 的开销，
	// 且发生在转换 goroutine 内会拖慢转发。精确截尾留给 finalize()。
	if len(a.tail) > 2*rawAccumulatorTailSize {
		drop := len(a.tail) - rawAccumulatorTailSize
		a.midDropped += drop
		// 向前搬移保留最后 rawAccumulatorTailSize 字节，复用底层数组。
		n := copy(a.tail, a.tail[drop:])
		a.tail = a.tail[:n]
	}
}

// finalize 返回最终 RawResponseBody 文本与被丢弃的总字节数（0 表示未截断）。
// 不修改累积器状态，可重复调用。
func (a *rawSSEAccumulator) finalize() (string, int) {
	// 头段未封口 → 全部内容都在头段，未发生截断。
	if !a.headFull {
		return string(a.head), 0
	}

	tail := a.tail
	dropped := a.midDropped
	// 摊还裁剪允许尾段暂时超出预算，在此做最终精确截尾。
	if len(tail) > rawAccumulatorTailSize {
		d := len(tail) - rawAccumulatorTailSize
		dropped += d
		tail = tail[d:]
	}

	if dropped == 0 {
		// 头段封口但中间无丢弃：内容仍完整，不加标记以免用户误判被截断。
		return string(a.head) + string(tail), 0
	}

	var b strings.Builder
	b.Grow(len(a.head) + len(tail) + 96)
	b.Write(a.head)
	fmt.Fprintf(&b, "\n...[llmux truncated %d bytes of raw SSE middle section, head/tail preserved]...\n\n", dropped)
	b.Write(tail)
	return b.String(), dropped
}

// accumulateRawSSE 把当前 SSE event 追加到累积器。state.rawAcc 为 nil 时不累积
// （非日志场景避免内存开销）。event 名取自 state.currentEvent，保证 Anthropic 等
// 依赖 event 行的协议在日志中保留完整结构。
func accumulateRawSSE(state *realtimeStreamState, data string) {
	if state.rawAcc == nil {
		return
	}
	var b strings.Builder
	b.Grow(len(data) + 16)
	if state.currentEvent != "" {
		b.WriteString("event: ")
		b.WriteString(state.currentEvent)
		b.WriteByte('\n')
	}
	b.WriteString("data: ")
	b.WriteString(data)
	b.WriteString("\n\n")
	state.rawAcc.write(b.String())
}
