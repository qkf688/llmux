package transform

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

func init() {
	if raw := strings.TrimSpace(os.Getenv("LLMIO_TRANSFORM_MAX_SSE_EVENT_SIZE")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			maxSSEEventSize = v
		}
	}
}
