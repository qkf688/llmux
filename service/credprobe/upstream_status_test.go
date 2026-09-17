package credprobe

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRunStreamProbe_PassesOn201 锁流式探针的通过口径与 common.IsPassingStatus 同源
// （Stage 5：非流式探针早已认 2xx，本次把健康检查/真实链路/模型列表都并到这一条口径）。
// Content-Type 门刻意留在 2xx 判定之后，所以这条用例必须同时给 event-stream，
// 否则测的是「上游不支持流式」而不是状态码。
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
