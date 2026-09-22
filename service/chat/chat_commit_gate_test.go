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

// 预算超限（注释洪水撑爆预读预算）→ 按不可提交处置（组织级换下家、不冷却凭据）。
// 垃圾洪水是端点/供应商级行为，归凭据级网络出口会让组内 key 逐条陪烧、
// 重试循环反复撞回同一坏端点（审查回批 2026-09-23 修正分类）。
func TestGateStreamCommitment_CommentFlood_RejectedAsContentInvalid(t *testing.T) {
	flood := strings.Repeat(": "+strings.Repeat("x", 78)+"\n", 60_000) // ~4.7 MiB > 4 MiB 预算
	src := io.NopCloser(strings.NewReader(flood))
	_, err := gateStreamCommitment(src, time.Second)
	if !errors.Is(err, errUpstreamContentInvalid) {
		t.Fatalf("err = %v, want errUpstreamContentInvalid（预算超限=不可提交）", err)
	}
}

// 畸形巨行（单行超 1 MiB）同归不可提交：上游吐的是坏流，不是凭据问题。
func TestGateStreamCommitment_TooLongLine_RejectedAsContentInvalid(t *testing.T) {
	bad := "data: " + strings.Repeat("x", 2<<20) + "\n\n"
	src := io.NopCloser(strings.NewReader(bad))
	_, err := gateStreamCommitment(src, time.Second)
	if !errors.Is(err, errUpstreamContentInvalid) {
		t.Fatalf("err = %v, want errUpstreamContentInvalid（单行超限）", err)
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
