package chat

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

const gateGoodSSE = "data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n" +
	"data: {\"id\":\"c1\",\"choices\":[{\"delta\":{\"content\":\" more\"}}]}\n\n" +
	"data: [DONE]\n\n"

// 首帧可提交 → 预读字节完整回放：重放体逐字节等于原始流（含后续帧），
// 透传/转换/handler 消费无感知（commit-before-write 的核心不变量）。
func TestGateStreamCommitment_GoodFirstFrame_ReplaysExactly(t *testing.T) {
	src := io.NopCloser(strings.NewReader(gateGoodSSE))
	gated, err := gateStreamCommitment(src, time.Second)
	if err != nil {
		t.Fatalf("gateStreamCommitment err = %v, want 放行", err)
	}
	defer gated.Close()
	got, err := io.ReadAll(gated)
	if err != nil {
		t.Fatalf("read replayed body: %v", err)
	}
	if string(got) != gateGoodSSE {
		t.Fatalf("replayed =\n%q\nwant 原始流逐字节:\n%q", got, gateGoodSSE)
	}
}

// 首帧是错误载荷 → 判不可提交（errUpstreamContentInvalid 分类）。
func TestGateStreamCommitment_ErrorFirstFrame_Rejected(t *testing.T) {
	bad := "data: {\"error\":{\"message\":\"boom\"}}\n\n" + gateGoodSSE
	src := io.NopCloser(strings.NewReader(bad))
	_, err := gateStreamCommitment(src, time.Second)
	if !errors.Is(err, errUpstreamContentInvalid) {
		t.Fatalf("err = %v, want errUpstreamContentInvalid", err)
	}
}

// 非 SSE 内容（合法 JSON 对象整块当 data 行之外的裸行/零帧）→ 判不可提交。
func TestGateStreamCommitment_ZeroFrames_Rejected(t *testing.T) {
	src := io.NopCloser(strings.NewReader(`{"error":{"message":"json, not sse"}}`))
	_, err := gateStreamCommitment(src, time.Second)
	if !errors.Is(err, errUpstreamContentInvalid) {
		t.Fatalf("err = %v, want errUpstreamContentInvalid（零帧）", err)
	}
}

// 上游头后无字节 → 看门狗超时，原样返回 errFirstByteWatchdog（凭据级网络分类依据）。
func TestGateStreamCommitment_FirstByteTimeout_ReturnsWatchdogError(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close() })
	_, err := gateStreamCommitment(pr, 50*time.Millisecond)
	if !errors.Is(err, errFirstByteWatchdog) {
		t.Fatalf("err = %v, want errFirstByteWatchdog", err)
	}
	if errors.Is(err, errUpstreamContentInvalid) {
		t.Fatalf("err = %v, 不应归内容不可提交分类（超时走凭据级网络出口）", err)
	}
}

// 预读字节回放不重不漏：扫描器预读进缓冲的字节也必须完整出现在重放体头部。
func TestCapturingReadCloser_ReplayNoGapNoDuplicate(t *testing.T) {
	raw := bytes.Repeat([]byte("x"), 100_000)
	capture := newCapturingReadCloser(io.NopCloser(bytes.NewReader(raw)))
	buf := make([]byte, 8_000)
	if _, err := io.ReadFull(capture, buf); err != nil {
		t.Fatalf("preflight read: %v", err)
	}
	got, err := io.ReadAll(capture.Replay())
	if err != nil {
		t.Fatalf("read replay: %v", err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("replay len = %d, want %d 且逐字节相等", len(got), len(raw))
	}
}
