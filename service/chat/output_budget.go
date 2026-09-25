package chat

import (
	"strings"

	"github.com/qkf688/llmux/common"
)

// outputBudgetAccumulator 流式输出累积的预算闸（Stage 23）：OfStringArray 此前
// 无上限累积（chunk 数 × chunk 大小都不受限），长流会把内存（转换期）与落库体积
// （ChatIO / ResponseBody 拼接）双双撑到 MiB 级 × 并发流数。
//
// 策略与 rawSSEAccumulator 同构（头段固定 + 尾段滚动）：头段写满后封口，后续
// chunk 进入尾段滚动缓冲，被挤出的字节记入 midDropped。flush() 产出最终 chunk
// 序列：未超限原样逐 chunk；超限产出 [头段合并 chunk, 统一标记, 尾段合并 chunk]
// 三元素——下游 applyResponseBodyToLog 拼整篇后 ≤ 预算，ChatIO 存储同受约束。
//
// 预算字节（含标记预留）由调用方传入（getResponseLogMaxBytes，detached ctx 读取）；
// 只影响日志累积，不影响逐 chunk 转发与 usage 提取。
type outputBudgetAccumulator struct {
	budget     int
	head       strings.Builder
	headFull   bool
	tail       []byte
	tailSize   int
	midDropped int
	chunks     []string // 未超限时原样保留的 chunk 引用
}

func newOutputBudgetAccumulator(budget int) *outputBudgetAccumulator {
	// 标记文案 ~101B（数字位数 ±3），预留 104；两侧对半。
	b := budget - 104
	if b < 2 {
		b = 2
	}
	return &outputBudgetAccumulator{budget: budget, tailSize: b / 2}
}

func (a *outputBudgetAccumulator) write(chunk string) {
	if !a.headFull {
		if a.head.Len()+len(chunk) <= a.tailSize {
			a.head.WriteString(chunk)
			a.chunks = append(a.chunks, chunk)
			return
		}
		a.headFull = true
		a.chunks = nil
		// 头段封口时把已攒内容整体记入头段缓冲，滚动尾从空开始。
		a.tail = a.tail[:0]
	}

	a.tail = append(a.tail, chunk...)
	if len(a.tail) > 2*a.tailSize {
		drop := len(a.tail) - a.tailSize
		a.midDropped += drop
		n := copy(a.tail, a.tail[drop:])
		a.tail = a.tail[:n]
	}
}

// flush 返回最终 chunk 序列与被丢弃的总字节数（0 = 未截断，序列即原样 chunks）。
// 只读不改状态（与 rawSSEAccumulator.finalize 同契约）。
func (a *outputBudgetAccumulator) flush() ([]string, int) {
	if !a.headFull {
		return a.chunks, 0
	}
	// 摊还裁剪允许尾段暂时超预算，最终精确截尾。
	tail := a.tail
	if len(tail) > a.tailSize {
		tail = tail[len(tail)-a.tailSize:]
	}
	headStr := a.head.String()
	tailStr := string(tail)
	marker := common.TruncateHeadTail(len(headStr), len(tailStr), a.midDropped)
	return []string{headStr, marker, tailStr}, a.midDropped
}
