package chat

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// failReader 在首次 Read 即返回指定错误（模拟上游读侧失败形态，不产出字节）。
type failReader struct{ err error }

func (r failReader) Read([]byte) (int, error) { return 0, r.err }

// TestMarkUpstreamReader_EOFNotMarked 锁「终止 EOF 不是失败」。
//
// 判据必须打在 io.Copy 的返回值上而不是 errors.Is 上：io.Copy 判流结束用的是**身份比较**
// （`er != io.EOF`），`upstreamReadErr` 由 Unwrap 让 errors.Is(err, io.EOF) 恒真——只断言
// errors.Is 会让「正常收尾被当成读失败」这个缺陷隐身。
//
// 真机后果（2026-10-05 定位）：handler/v1 交给 writeResponseBody 的 body 是
// io.TeeReader(MarkUpstreamReader(resBody), pw)，终止 EOF 被套上上游标记后，**每一轮正常
// 完成的流**都会被判成失败——日志落 `processer error: write response body failed: EOF`
// （缺终止符的流），客户端还会在 [DONE] 之后收到一条凭空多出的协议 error 事件。
func TestMarkUpstreamReader_EOFNotMarked(t *testing.T) {
	const payload = "data: {\"x\":1}\n\n"

	n, err := io.Copy(io.Discard, MarkUpstreamReader(strings.NewReader(payload)))
	if err != nil {
		t.Fatalf("io.Copy err = %v, want nil（正常收尾不得被判成读失败）", err)
	}
	if n != int64(len(payload)) {
		t.Fatalf("io.Copy n = %d, want %d", n, len(payload))
	}

	r := MarkUpstreamReader(strings.NewReader("ab"))
	buf := make([]byte, 8)
	if _, err := r.Read(buf); err != nil {
		t.Fatalf("首次 Read err = %v, want nil", err)
	}
	// 身份比较：与 io.Copy 的终止判据同源，errors.Is 判不出这个断层。
	if _, err := r.Read(buf); err != io.EOF {
		t.Fatalf("末次 Read err = %v, want io.EOF（身份）", err)
	}
}

// TestMarkUpstreamReader_ReadErrorsStillMarked 锁方向标记能力不退化（C2）：
// 非 EOF 的读错仍必须带 upstreamReadErr——「上游 RST」是非超时 net.Error，判定层的类型
// 反推会把它误判成客户端撤走（该清钉的没清），方向事实只能由错误产生处承载。
func TestMarkUpstreamReader_ReadErrorsStillMarked(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"chunked 截断", io.ErrUnexpectedEOF},
		{"上游 RST（非超时 net.Error）", rstLikeError()},
		{"普通读错", errors.New("read boom")},
		// 被包装过的 EOF（非身份）：消费者（io.Copy 的身份比较 / bufio.Scanner.Err）照样
		// 判失败，方向仍归上游。这条把「身份判据」与「errors.Is 判据」区分开——换成
		// `!errors.Is(err, io.EOF)` 的实现时只有本行变红（其余行不受影响）。
		{"被包装过的 EOF", fmt.Errorf("upstream body: %w", io.EOF)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := MarkUpstreamReader(failReader{err: tc.err}).Read(make([]byte, 8))
			if n != 0 {
				t.Fatalf("n = %d, want 0", n)
			}
			if !IsUpstreamReadErr(err) {
				t.Fatalf("err = %v, want 带上游读侧标记", err)
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want errors.Is 触达内层（诊断能力不得被标记层吞掉）", err)
			}
		})
	}
}
