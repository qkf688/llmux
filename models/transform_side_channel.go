package models

import (
	"strings"
	"sync"
)

// TransformSideChannel 是协议转换过程的旁路产物容器。
//
// 为什么需要它：转换后发给客户端的流受目标协议表达能力限制（Anthropic 无
// reasoning token 字段、message_start 必须在拿到真实 prompt_tokens 之前就发出），
// 从那条流反解 usage 必然有损。故由转换层在解析**上游原始响应**时把 usage 旁路
// 交出来，落库侧优先采用它，不再依赖下游流。
//
// 并发契约：写入只发生在转换 goroutine 内，读取由调用方在流结束（pipe 关闭、
// processer 读到 EOF）之后进行。读时理论上已无写入者，但流式多行覆盖写仍需
// 互斥，故所有访问一律加锁。
//
// 所有方法对 nil 接收者安全：未启用旁路时调用方可直接传 nil，转换层无需判空。
type TransformSideChannel struct {
	mu            sync.Mutex
	rawBody       *strings.Builder
	upstreamUsage *Usage
}

// NewTransformSideChannel 创建侧信道。captureRawBody 为 true 时才分配原始体累积器
// （非日志场景避免为大流累积内存）；usage 捕获无条件启用，因为它服务计费与统计。
func NewTransformSideChannel(captureRawBody bool) *TransformSideChannel {
	c := &TransformSideChannel{}
	if captureRawBody {
		c.rawBody = &strings.Builder{}
	}
	return c
}

// WantsRawBody 表示是否需要累积上游原始体。转换层据此决定要不要付出累积开销。
func (c *TransformSideChannel) WantsRawBody() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rawBody != nil
}

// AppendRawBody 追加上游原始体片段。未启用原始体捕获时为 no-op。
func (c *TransformSideChannel) AppendRawBody(s string) {
	if c == nil || s == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rawBody == nil {
		return
	}
	c.rawBody.WriteString(s)
}

// RawBody 返回已累积的上游原始体；未启用或未写入时返回空串。
func (c *TransformSideChannel) RawBody() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rawBody == nil {
		return ""
	}
	return c.rawBody.String()
}

// SetUpstreamUsage 记录上游原始 usage。
//
// 允许被多次调用（Anthropic 的 usage 就拆在 message_start 与 message_delta 两处），
// 语义是**最后一次有效观测胜出**。约束在调用方：后写的快照必须是先写快照的超集，
// 否则会把已观测到的字段覆盖成 0。之所以不做逐字段合并——合并需要区分「上游报了 0」
// 与「上游没报」，而 map 解析后两者不可分；把超集责任交给最了解协议时序的转换器更可靠。
//
// token 全为 0 的快照会被丢弃（见 Usage.HasTokens）：上游只要回包里带 usage 对象，
// 各协议适配器都会产出非 nil 的全零 Usage。若照收，落库侧会把它当成最可信来源并
// 跳过 missing 告警——而 UsageSource 存在的意义正是区分「上游没给」与「归集链路丢了」。
// 在此收口，避免各捕获点各写一遍零值判断，也保证全零快照不会抹掉先前的有效观测。
func (c *TransformSideChannel) SetUpstreamUsage(u Usage) {
	if c == nil || !u.HasTokens() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	snapshot := u
	c.upstreamUsage = &snapshot
}

// UpstreamUsage 返回上游 usage 及其是否被捕获过。
// ok 为 false 表示转换层没有交出 usage（未走转换路径，或上游确实没给）。
func (c *TransformSideChannel) UpstreamUsage() (Usage, bool) {
	if c == nil {
		return Usage{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.upstreamUsage == nil {
		return Usage{}, false
	}
	return *c.upstreamUsage, true
}
