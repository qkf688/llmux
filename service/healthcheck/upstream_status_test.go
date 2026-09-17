package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// TestRequestUpstream_FailsOnNon2xx 通过侧的对照：口径放宽到 2xx 不等于放宽到全部，
// 3xx/4xx/5xx 仍必须回 HealthCheckError 并带上游原文（停用决策与排障都靠它）。
// 刻意不测 199：真实 HTTP 服务端回不出 1xx（会被当信息响应吞掉），区间下界由
// common.TestIsPassingStatus 的纯函数用例覆盖。
func TestRequestUpstream_FailsOnNon2xx(t *testing.T) {
	for _, status := range []int{http.StatusMultipleChoices, http.StatusUnauthorized, http.StatusInternalServerError} {
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
		if err == nil {
			t.Fatalf("status %d 应判失败, got nil", status)
		}
		if !errors.As(err, &hcerr) {
			t.Fatalf("status %d 应回 HealthCheckError（带状态码与原文）, got %T: %v", status, err, err)
		}
		if hcerr.StatusCode != status {
			t.Fatalf("HealthCheckError.StatusCode = %d, want %d", hcerr.StatusCode, status)
		}
	}
}
