package streaming

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/qkf688/llmux/models"
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
	sc := models.NewTransformSideChannel(true)
	out, err := TransformResponseRealtime(resp, "openai-res", "openai", sc)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	io.Copy(io.Discard, out.Body) // 读完整流，让 goroutine 结束

	got := sc.RawBody()
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

func TestRawAccumulator_PreservesEventName(t *testing.T) {
	// Anthropic 风格 SSE 带 event 行；累积体必须保留 event 行，否则日志里看不出
	// message_stop 这类靠 event 名标识的收尾事件。
	sse := "event: message_start\n" +
		"data: {\"type\":\"message_start\",\"message\":{\"id\":\"m1\",\"model\":\"claude\"}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	resp := newSSEResponse(sse)
	sc := models.NewTransformSideChannel(true)
	out, err := TransformResponseRealtime(resp, "anthropic", "openai", sc)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	io.Copy(io.Discard, out.Body)

	got := sc.RawBody()
	if !strings.Contains(got, "event: message_start") {
		t.Errorf("accumulator missing event line 'message_start': %q", got)
	}
	if !strings.Contains(got, "event: message_stop") {
		t.Errorf("accumulator missing event line 'message_stop': %q", got)
	}
}

func TestRawAccumulator_TruncatesMiddleKeepingHeadAndTail(t *testing.T) {
	// 构造远超预算的流（约 2MB）：首事件与末事件带唯一标记，中间填充。
	// 期望：头尾都在、中间被丢弃并留下截断标记。
	var sseBuilder strings.Builder
	sseBuilder.WriteString("data: {\"type\":\"response.created\",\"response\":{\"id\":\"FIRST_EVENT_MARKER\"}}\n\n")
	for i := 0; i < 2000; i++ {
		padding := strings.Repeat("x", 1000)
		sseBuilder.WriteString("data: {\"type\":\"response.output_text.delta\",\"delta\":\"")
		sseBuilder.WriteString(padding)
		sseBuilder.WriteString("\"}\n\n")
	}
	sseBuilder.WriteString("data: {\"type\":\"response.completed\",\"usage\":{\"total_tokens\":42},\"marker\":\"LAST_EVENT_MARKER\"}\n\n")

	resp := newSSEResponse(sseBuilder.String())
	sc := models.NewTransformSideChannel(true)
	out, err := TransformResponseRealtime(resp, "openai-res", "openai", sc)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	io.Copy(io.Discard, out.Body)

	got := sc.RawBody()

	// 头段保留：起始事件在。
	if !strings.Contains(got, "FIRST_EVENT_MARKER") {
		t.Error("accumulator lost the first event (head section not preserved)")
	}
	// 尾段保留：收尾事件（含 usage）在——这是纯头部截断会丢掉的关键信息。
	if !strings.Contains(got, "LAST_EVENT_MARKER") {
		t.Error("accumulator lost the last event (tail section not preserved)")
	}
	if !strings.Contains(got, "total_tokens") {
		t.Error("accumulator lost trailing usage event")
	}
	// 中间被丢弃时必须留下可识别的截断标记，避免与「上游就这么短」混淆。
	if !strings.Contains(got, "truncated") {
		t.Errorf("accumulator missing truncation marker: tail=%q", got[max(0, len(got)-200):])
	}
	// 总长度受头尾预算约束（标记本身占少量额外字节）。
	if limit := rawAccumulatorHeadSize + rawAccumulatorTailSize + 256; len(got) > limit {
		t.Errorf("accumulator exceeded budget: got %d bytes, limit %d", len(got), limit)
	}
	if len(got) >= sseBuilder.Len() {
		t.Errorf("expected truncation, but accumulator captured full body (%d bytes)", len(got))
	}
}

func TestRawAccumulator_NoMarkerWhenWithinBudget(t *testing.T) {
	// 未超预算时不应出现截断标记，否则用户会误判日志被截断。
	sse := "data: {\"type\":\"response.created\"}\n\n" +
		"data: {\"type\":\"response.completed\"}\n\n"

	resp := newSSEResponse(sse)
	sc := models.NewTransformSideChannel(true)
	out, err := TransformResponseRealtime(resp, "openai-res", "openai", sc)
	if err != nil {
		t.Fatalf("TransformResponseRealtime: %v", err)
	}
	defer out.Body.Close()
	io.Copy(io.Discard, out.Body)

	if got := sc.RawBody(); strings.Contains(got, "truncated") {
		t.Errorf("unexpected truncation marker for small stream: %q", got)
	}
}

// 以下为 rawSSEAccumulator 的单元级边界测试：端到端用例走 SSE 解析，无法精确构造
// 头段恰好写满、丢弃字节数等边界，故直接驱动 write/finalize。

func TestRawSSEAccumulator_NoTruncationWithinHead(t *testing.T) {
	var a rawSSEAccumulator
	a.write("data: a\n\n")
	a.write("data: b\n\n")

	body, dropped := a.finalize()
	if dropped != 0 {
		t.Errorf("dropped = %d, want 0", dropped)
	}
	if want := "data: a\n\ndata: b\n\n"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
	if strings.Contains(body, "truncated") {
		t.Error("unexpected truncation marker")
	}
}

func TestRawSSEAccumulator_HeadSealedWithoutDrop(t *testing.T) {
	// 1KB × 512 正好填满头段；第 513 个事件整块转入尾段但未溢出尾段预算，
	// 内容仍完整，不应出现截断标记。
	chunk := strings.Repeat("x", 1024)
	var a rawSSEAccumulator
	for i := 0; i < rawAccumulatorHeadSize/1024; i++ {
		a.write(chunk)
	}
	if a.headFull {
		t.Fatal("head sealed too early: exact fit should not seal")
	}
	a.write(chunk)
	if !a.headFull {
		t.Fatal("head should be sealed after exceeding budget")
	}

	body, dropped := a.finalize()
	if dropped != 0 {
		t.Errorf("dropped = %d, want 0 (tail did not overflow)", dropped)
	}
	if want := rawAccumulatorHeadSize + 1024; len(body) != want {
		t.Errorf("len(body) = %d, want %d", len(body), want)
	}
	if strings.Contains(body, "truncated") {
		t.Error("unexpected truncation marker when nothing was dropped")
	}
}

func TestRawSSEAccumulator_DroppedByteCountIsExact(t *testing.T) {
	// 整块决策下头段恰好装 512 个 1KB 事件；其余进尾段并滚动，
	// 丢弃量必须精确等于「进尾段总量 - 尾段预算」，不受摊还裁剪时机影响。
	const chunkSize = 1024
	chunk := strings.Repeat("x", chunkSize)
	totalChunks := 2048

	var a rawSSEAccumulator
	for i := 0; i < totalChunks; i++ {
		a.write(chunk)
	}

	body, dropped := a.finalize()

	intoTail := totalChunks*chunkSize - rawAccumulatorHeadSize
	wantDropped := intoTail - rawAccumulatorTailSize
	if dropped != wantDropped {
		t.Errorf("dropped = %d, want %d", dropped, wantDropped)
	}
	if !strings.Contains(body, "truncated") {
		t.Error("missing truncation marker")
	}
	// 头尾各占满预算，多出的只有标记本身。
	if limit := rawAccumulatorHeadSize + rawAccumulatorTailSize + 256; len(body) > limit {
		t.Errorf("len(body) = %d, exceeds limit %d", len(body), limit)
	}

	// finalize 不改状态，可重复调用且结果一致。
	body2, dropped2 := a.finalize()
	if dropped2 != dropped || body2 != body {
		t.Error("finalize is not idempotent")
	}
}

func TestRawSSEAccumulator_SingleChunkLargerThanHead(t *testing.T) {
	// 单个事件就超过头段预算：头段留空、事件整块进尾段，尾段再按预算截尾。
	size := rawAccumulatorHeadSize + 90*1024
	var a rawSSEAccumulator
	a.write(strings.Repeat("y", size))

	if len(a.head) != 0 {
		t.Errorf("len(head) = %d, want 0 (oversized chunk must not be split)", len(a.head))
	}

	body, dropped := a.finalize()
	if want := size - rawAccumulatorTailSize; dropped != want {
		t.Errorf("dropped = %d, want %d", dropped, want)
	}
	if !strings.Contains(body, "truncated") {
		t.Error("missing truncation marker")
	}
}
