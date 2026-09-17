package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qkf688/llmux/providers"
)

// TestRequestUpstream_PassesOn201 锁住 Stage 5 的统一口径：回 201 的兼容网关不得判失败。
// 这里判失败的代价不是「诊断报个错」——handleCheckResult 累计失败到阈值会自动停用
// 模型-供应商关联，线上静默少一条可用线路，且停用原因不入库。
func TestRequestUpstream_PassesOn201(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"cmpl-1","object":"chat.completion","model":"gpt-4.1","choices":[]}`))
	}))
	t.Cleanup(srv.Close)

	client, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-201"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("providers.New: %v", err)
	}

	if err := requestUpstream(context.Background(), client, http.Header{}, "gpt-4.1", chooseTestBody(providers.TypeOpenAI)); err != nil {
		t.Fatalf("requestUpstream(201) = %v, want nil（判失败会导致关联被自动停用）", err)
	}
	if hits != 1 {
		t.Fatalf("上游命中次数 = %d, want 1", hits)
	}
}

// TestRequestUpstream_FailsOnNonPassingStatus 通过侧的对照：口径带上 201 不等于放宽到
// 整个 2xx——204（成功但无正文）、3xx、4xx、5xx 都必须继续判失败并回 HealthCheckError
// （分组级排障与停用决策都靠它）。
// 刻意不测 199：真实 HTTP 服务端回不出 1xx（会被当信息响应吞掉），区间外侧由
// common.TestIsPassingStatus 的纯函数用例覆盖。
func TestRequestUpstream_FailsOnNonPassingStatus(t *testing.T) {
	const upstreamBody = `{"error":{"message":"nope"}}`
	for _, status := range []int{http.StatusNoContent, http.StatusMultipleChoices, http.StatusUnauthorized, http.StatusInternalServerError} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(upstreamBody))
		}))

		client, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-x"}`, srv.URL), "")
		if err != nil {
			srv.Close()
			t.Fatalf("providers.New: %v", err)
		}
		err = requestUpstream(context.Background(), client, http.Header{}, "gpt-4.1", chooseTestBody(providers.TypeOpenAI))
		srv.Close()

		var hcerr *HealthCheckError
		if err == nil {
			t.Fatalf("status %d 应判失败, got nil", status)
		}
		if !errors.As(err, &hcerr) {
			t.Fatalf("status %d 应回 HealthCheckError, got %T: %v", status, err, err)
		}
		if hcerr.StatusCode != status {
			t.Fatalf("HealthCheckError.StatusCode = %d, want %d", hcerr.StatusCode, status)
		}
	}
}

// TestRequestUpstream_FailureKeepsUpstreamBody 失败形状是本轮「只统一判据、不动失败契约」
// 的那一半：HealthCheckError.Body 必须带上游原文，否则逐组失败原因（health_check_logs.error
// 这个排查入口）会回归成光秃状态码。只取有正文的状态码——204/304 的正文被 net/http 吞掉。
func TestRequestUpstream_FailureKeepsUpstreamBody(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusInternalServerError} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"nope"}}`))
		}))

		client, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-x"}`, srv.URL), "")
		if err != nil {
			srv.Close()
			t.Fatalf("providers.New: %v", err)
		}
		err = requestUpstream(context.Background(), client, http.Header{}, "gpt-4.1", chooseTestBody(providers.TypeOpenAI))
		srv.Close()

		var hcerr *HealthCheckError
		if !errors.As(err, &hcerr) {
			t.Fatalf("status %d 应回 HealthCheckError, got %v", status, err)
		}
		if !strings.Contains(hcerr.Body, "nope") {
			t.Fatalf("status %d 的 HealthCheckError.Body = %q, want 带上游原文", status, hcerr.Body)
		}
	}
}
