package streaming

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func newSSEResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestRawAccumulator_CapturesOriginalSSE(t *testing.T) {
	// 上游 openai-res 格式的 SSE 流，转成 openai；累积器应记录原始 openai-res 事件。
	sse := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"r1\"}}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hi\"}\n\n" +
		"data: {\"type\":\"response.completed\"}\n\n"

	resp := newSSEResponse(sse)
	var acc strings.Builder
	out, err := TransformResponseRealtime(resp, "openai-res", "openai", &acc)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	io.Copy(io.Discard, out.Body) // 读完整流，让 goroutine 结束

	got := acc.String()
	// 累积的是原始 SSE（含 data: 前缀和 \n\n），不是转换后的。
	if !strings.Contains(got, "response.created") {
		t.Errorf("accumulator missing original event 'response.created': %q", got)
	}
	if !strings.Contains(got, "response.output_text.delta") {
		t.Errorf("accumulator missing original event 'response.output_text.delta': %q", got)
	}
	if !strings.Contains(got, "response.completed") {
		t.Errorf("accumulator missing original event 'response.completed': %q", got)
	}
	// 每个 event 应有 data: 前缀
	if strings.Count(got, "data: ") != 3 {
		t.Errorf("expected 3 'data: ' lines, got %d in %q", strings.Count(got, "data: "), got)
	}
}

func TestRawAccumulator_NilDoesNotAccumulate(t *testing.T) {
	sse := "data: {\"type\":\"response.created\"}\n\n"
	resp := newSSEResponse(sse)
	out, err := TransformResponseRealtime(resp, "openai-res", "openai", nil)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	io.Copy(io.Discard, out.Body)
	// nil 累积器不应 panic，流应正常转换。
}

func TestRawAccumulator_SizeLimitTruncates(t *testing.T) {
	// 构造超过 maxRawAccumulatorSize 的流：每个 event 1KB，发 2000 个（总 2MB > 1MB）。
	var sseBuilder strings.Builder
	for i := 0; i < 2000; i++ {
		// 每个 data 约 1KB
		padding := strings.Repeat("x", 1000)
		sseBuilder.WriteString("data: {\"type\":\"response.output_text.delta\",\"delta\":\"")
		sseBuilder.WriteString(padding)
		sseBuilder.WriteString("\"}\n\n")
	}

	resp := newSSEResponse(sseBuilder.String())
	var acc strings.Builder
	out, err := TransformResponseRealtime(resp, "openai-res", "openai", &acc)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	io.Copy(io.Discard, out.Body)

	got := acc.String()
	if len(got) > maxRawAccumulatorSize {
		t.Errorf("accumulator exceeded limit: got %d bytes, limit %d", len(got), maxRawAccumulatorSize)
	}
	// 应被截断（不是全量 2MB）
	if len(got) == sseBuilder.Len() {
		t.Errorf("expected truncation, but accumulator captured full body (%d bytes)", len(got))
	}
}
