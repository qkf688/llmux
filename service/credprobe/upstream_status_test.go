package credprobe

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRunStreamProbe_FailsOn204 通过集边界（Stage 5 审查回批）：204「成功但无正文」不判
// 通过。它曾落在「任意 2xx 即通过」的区间里，而探针要证明的是「这条 key 在这模型上真能
// 跑出回复」，没有正文就没证明任何事。
func TestRunStreamProbe_FailsOn204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 2*time.Second)
	if out.Passed {
		t.Fatalf("204 不应判通过, got %+v", out)
	}
	if out.StatusCode != http.StatusNoContent {
		t.Fatalf("StatusCode = %d, want 204", out.StatusCode)
	}
	if out.Summary != ProbeSummaryUpstreamError {
		t.Fatalf("Summary = %q, want %q", out.Summary, ProbeSummaryUpstreamError)
	}
}

// TestRunStreamProbe_PassesOn201 锁流式探针与全项目通过口径同源（Stage 5，判据单源
// common.IsPassingStatus：仅 200/201）。Content-Type 门刻意排在状态码判定之后，所以这条
// 用例必须同时给 event-stream——否则测的是「上游不支持流式」而不是状态码。
func TestRunStreamProbe_PassesOn201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream") // 必须在 WriteHeader 之前
		w.WriteHeader(http.StatusCreated)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		_, _ = io.WriteString(w, sseBody([]SSEFrame{
			{Data: `{"choices":[{"delta":{"content":"ok"}}]}`},
			{Data: "[DONE]"},
		}, true))
	}))
	t.Cleanup(srv.Close)

	out := runStreamProbe(t, srv.URL, 2*time.Second)
	if !out.Passed {
		t.Fatalf("201 应判通过; got %+v", out)
	}
	if out.StatusCode != http.StatusCreated {
		t.Fatalf("StatusCode = %d, want 201", out.StatusCode)
	}
	if !out.Framed || len(out.Frames) != 2 {
		t.Fatalf("Framed = %v, frames = %d, want true/2（必须真进入分帧）", out.Framed, len(out.Frames))
	}
}
