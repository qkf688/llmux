package credprobe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/providers"
)

// probeRequestCapture 记录最后一次 BuildReq 的入参，供断言「探针注入了什么 body/头」
// （流式探针要证明它把 stream:true 写进了测试体、并带了 Accept 头）。
type probeRequestCapture struct {
	body   []byte
	header http.Header
}

// probeFakeClient 最小 providers.Provider 替身：BuildReq 直接指向假上游 URL，
// headers 原样透传（探针本体不关心凭据装配，那是调用方链路的职责）。
type probeFakeClient struct {
	url         string
	buildReqErr error
	// capture 非 nil 时记录入参；用指针是因为 BuildReq 是值接收者，写自身字段传不出去。
	capture *probeRequestCapture
}

func (f probeFakeClient) BuildReq(ctx context.Context, header http.Header, model string, rawData []byte) (*http.Request, error) {
	if f.buildReqErr != nil {
		return nil, f.buildReqErr
	}
	if f.capture != nil {
		f.capture.body = append([]byte(nil), rawData...)
		f.capture.header = header.Clone()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.url, bytes.NewReader(rawData))
	if err != nil {
		return nil, err
	}
	for key, values := range header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	return req, nil
}

func (f probeFakeClient) Models(context.Context) ([]providers.Model, error) { return nil, nil }
func (f probeFakeClient) GetProxy() string                                  { return "" }

func runProbe(t *testing.T, url string, timeout time.Duration) CompletionProbeResult {
	t.Helper()
	return RunCompletionProbe(CompletionProbeRequest{
		Ctx:     t.Context(),
		Client:  probeFakeClient{url: url},
		Timeout: timeout,
		Model:   "probe-model",
		Body:    []byte(`{"model":"probe-model"}`),
	})
}

// TestRunCompletionProbe_PassedOn200And201 锁「通过集 = 200/201」：兼容网关回 201 不算
// 失败（Stage 5 统一的动因），判据单源 common.IsPassingStatus。
func TestRunCompletionProbe_PassedOn200And201(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"id":"cmpl-1"}`))
		}))
		t.Cleanup(srv.Close)

		out := runProbe(t, srv.URL, 2*time.Second)
		if !out.Passed {
			t.Fatalf("status %d 应判通过; got %+v", status, out)
		}
		if out.StatusCode != status {
			t.Fatalf("StatusCode = %d, want %d（判定放宽后仍须原样上报状态码）", out.StatusCode, status)
		}
		if string(out.Body) != `{"id":"cmpl-1"}` {
			t.Fatalf("Body = %q, want 上游原文", out.Body)
		}
	}
}

// TestRunCompletionProbe_UpstreamErrorClassification 锁未通过判定的归类与统一细节：
// error_type 来自状态码映射，Detail 优先解析 error.message 且带状态码行。
// 204 必须在表里——它曾是「任意 2xx 即通过」的漏网形态（成功但无正文，不该算答复）。
func TestRunCompletionProbe_UpstreamErrorClassification(t *testing.T) {
	cases := []struct {
		status         int
		body           string
		wantErrorType  string
		wantDetailPart string
	}{
		{http.StatusNoContent, ``, common.ErrorTypeUnknown, "HTTP 状态码: 204"},
		{http.StatusUnauthorized, `{"error":{"message":"invalid api key"}}`, common.ErrorTypeAuth, "invalid api key\nHTTP 状态码: 401"},
		{http.StatusTooManyRequests, `{"error":{"message":"rate limited"}}`, common.ErrorTypeValidation, "HTTP 状态码: 429"},
		{http.StatusInternalServerError, `<html>gateway boom</html>`, common.ErrorTypeProvider, "gateway boom"},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, tc.body)
		}))
		t.Cleanup(srv.Close)

		out := runProbe(t, srv.URL, 2*time.Second)
		if out.Passed {
			t.Fatalf("status %d 不应判通过", tc.status)
		}
		if out.StatusCode != tc.status {
			t.Fatalf("StatusCode = %d, want %d", out.StatusCode, tc.status)
		}
		if out.ErrorType != tc.wantErrorType {
			t.Fatalf("ErrorType = %q, want %q", out.ErrorType, tc.wantErrorType)
		}
		if out.Summary != ProbeSummaryUpstreamError {
			t.Fatalf("Summary = %q, want %q", out.Summary, ProbeSummaryUpstreamError)
		}
		if !strings.Contains(out.Detail, tc.wantDetailPart) {
			t.Fatalf("Detail = %q, want 含 %q", out.Detail, tc.wantDetailPart)
		}
		if !bytes.Equal(out.Body, []byte(tc.body)) {
			t.Fatalf("Body = %q, want 完整原文（Detail 截断不牵连 Body）", out.Body)
		}
	}
}

// TestRunCompletionProbe_SlowUpstreamIsTimeoutNotNetwork 锁探针的超时口径：
// ctx deadline 早于 transport 超时 → 慢上游稳定归 timeout（三端点统一的根因项）。
func TestRunCompletionProbe_SlowUpstreamIsTimeoutNotNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	t.Cleanup(srv.Close)

	out := runProbe(t, srv.URL, 100*time.Millisecond)
	if out.Passed {
		t.Fatal("慢上游不应判通过")
	}
	if out.ErrorType != common.ErrorTypeTimeout {
		t.Fatalf("ErrorType = %q, want timeout（误归 network 即口径漂移复现）", out.ErrorType)
	}
	if out.Summary != ProbeSummaryConnect {
		t.Fatalf("Summary = %q, want %q", out.Summary, ProbeSummaryConnect)
	}
}

// TestRunCompletionProbe_SlowBodyDripIsTimeoutNotNetwork headers 秒回、body 滴流时
// deadline 落在读体分支——transport 的 ResponseHeaderTimeout 管不到这里，读体错误同样
// 要按 DeadlineExceeded 归 timeout（否则统一契约从读体路径漏回 network）。
func TestRunCompletionProbe_SlowBodyDripIsTimeoutNotNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				if _, err := w.Write([]byte{0x20}); err != nil {
					return
				}
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		}
	}))
	t.Cleanup(srv.Close)

	out := runProbe(t, srv.URL, 100*time.Millisecond)
	if out.Passed {
		t.Fatal("body 滴流超期不应判通过")
	}
	if out.ErrorType != common.ErrorTypeTimeout {
		t.Fatalf("ErrorType = %q, want timeout（误归 network 即读体路径漏判）", out.ErrorType)
	}
	if out.Summary != ProbeSummaryReadResponse {
		t.Fatalf("Summary = %q, want %q", out.Summary, ProbeSummaryReadResponse)
	}
}

// TestRunCompletionProbe_UnreachableUpstreamIsNetwork 连不上（非超时）→ network。
func TestRunCompletionProbe_UnreachableUpstreamIsNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // 立刻关闭 → 连接被拒

	out := runProbe(t, url, 2*time.Second)
	if out.Passed {
		t.Fatal("连不上不应判通过")
	}
	if out.ErrorType != common.ErrorTypeNetwork || out.Summary != ProbeSummaryConnect {
		t.Fatalf("归类 = %s/%s, want network/%s", out.ErrorType, out.Summary, ProbeSummaryConnect)
	}
}

// TestRunCompletionProbe_BuildReqFailureShortCircuits BuildReq 失败零上游请求、归 network。
func TestRunCompletionProbe_BuildReqFailureShortCircuits(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)

	out := RunCompletionProbe(CompletionProbeRequest{
		Ctx:     t.Context(),
		Client:  probeFakeClient{url: srv.URL, buildReqErr: errors.New("boom")},
		Timeout: time.Second,
	})
	if out.Passed {
		t.Fatal("BuildReq 失败不应判通过")
	}
	if out.ErrorType != common.ErrorTypeNetwork || out.Summary != ProbeSummaryBuildRequest || out.Detail != "boom" {
		t.Fatalf("结果 = %+v, want network/%s/boom", out, ProbeSummaryBuildRequest)
	}
	if hits != 0 {
		t.Fatalf("upstream hits = %d, want 0", hits)
	}
}
