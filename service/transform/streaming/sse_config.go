package streaming

import (
	"os"
	"strconv"
	"strings"
)

const defaultMaxSSEEventSize = 32 * 1024 * 1024

// maxSSEEventSize controls the maximum in-memory size for a single SSE event.
//
// It is used by streaming format conversion (OpenAI Chat / OpenAI Responses / Anthropic).
// Override via `LLMIO_TRANSFORM_MAX_SSE_EVENT_SIZE` (bytes).
var maxSSEEventSize = defaultMaxSSEEventSize

// rawAccumulatorHeadSize / rawAccumulatorTailSize 控制流式响应原始 body 累积器的内存上限（字节）。
//
// 采用「头尾双段保留」而非「纯头部截断」：SSE 流的关键收尾事件（finish_reason / usage /
// [DONE] / message_stop 等）都在末尾，若只保留头部，超长响应恰好会丢掉最有诊断价值的结尾。
// 头段保留请求起始上下文，尾段用滚动缓冲保证结尾一定在；中间超出部分被丢弃并在
// RawResponseBody 里留下一条截断标记（含丢弃字节数），同时打一条 slog.Warn，让
// DB/前端与运维日志都能区分「上游就这么短」和「被截断」。只影响日志字段，不影响流式转发。
//
// 有意不提供环境变量覆盖（与同文件 maxSSEEventSize 不同）：后者决定单事件能否被解析、
// 影响请求成败，属运行时行为；而这里只是诊断日志的缓冲预算，512KB×2 对绝大多数场景足够，
// 暴露成配置只会扩大配置面。这是显式决策，非遗漏。
const (
	rawAccumulatorHeadSize = 512 * 1024
	rawAccumulatorTailSize = 512 * 1024
)

func init() {
	if raw := strings.TrimSpace(os.Getenv("LLMIO_TRANSFORM_MAX_SSE_EVENT_SIZE")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			maxSSEEventSize = v
		}
	}
}
