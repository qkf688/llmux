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

// maxRawAccumulatorSize 控制流式响应原始 body 累积器的内存上限（字节）。
// 达到上限后停止累积并记一条 warning，不影响流式转发，只影响日志里的 RawResponseBody 被截断。
// 1MB 足够覆盖常见流式响应；超长对话的原始 SSE 超出部分不记录。
const maxRawAccumulatorSize = 1 * 1024 * 1024

func init() {
	if raw := strings.TrimSpace(os.Getenv("LLMIO_TRANSFORM_MAX_SSE_EVENT_SIZE")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			maxSSEEventSize = v
		}
	}
}
