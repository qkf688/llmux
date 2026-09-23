package healthcheck

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/consts"
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

	testBody, format := chooseTestProbe(providers.TypeOpenAI)
	if err := requestUpstream(context.Background(), client, http.Header{}, "gpt-4.1", testBody, format); err != nil {
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
		testBody, format := chooseTestProbe(providers.TypeOpenAI)
		err = requestUpstream(context.Background(), client, http.Header{}, "gpt-4.1", testBody, format)
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
		testBody, format := chooseTestProbe(providers.TypeOpenAI)
		err = requestUpstream(context.Background(), client, http.Header{}, "gpt-4.1", testBody, format)
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

// TestRequestUpstream_ShapeGate 锁 Stage 19 内容形状门：状态码通过（见 common.IsPassingStatus）
// 之后，内容还须是所测协议的合法响应形状。动机：上游被反代/网关改写到登录页时回
// 200 + HTML，此前判健康——健康检查绿、真实聊天每条报错，坏线路留在轮询里
// （Stage 5 盲区的镜像）。Content-Type 故意乱写：形状门判内容不判头。
func TestRequestUpstream_ShapeGate(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr string // 空 = 判过
	}{
		{name: "空回复判过只挡形状不挡语义", body: ""},
		{name: "合法0-content判过", body: `{"id":"cmpl-1","object":"chat.completion","choices":[]}`},
		{name: "html登录页判失败并带状态码", body: `<html>login required</html>`, wantErr: "HTTP 200"},
		{name: "错误形状判失败", body: `{"error":{"message":"quota exceeded"}}`, wantErr: "error payload"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)

			client, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-shape"}`, srv.URL), "")
			if err != nil {
				t.Fatalf("providers.New: %v", err)
			}
			testBody, format := chooseTestProbe(providers.TypeOpenAI)
			err = requestUpstream(context.Background(), client, http.Header{}, "gpt-4.1", testBody, format)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("requestUpstream(%q) = %v, want nil", tc.body, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("requestUpstream(%q) = nil, want error containing %q", tc.body, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("requestUpstream(%q) err = %v, want containing %q", tc.body, err, tc.wantErr)
			}
		})
	}
}

// TestRequestUpstream_OversizeBodyUnverifiable 锁超限口径：响应体超出诊断读体上限后
// 形状不可验证，判失败且文案明示原因——「无法验证却判健康」会重新打开假健康面；
// 健康检查体是短补全，正常响应远低于 1 MiB，超限本身即异常。
func TestRequestUpstream_OversizeBodyUnverifiable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("a"), common.UpstreamDiagnosticReadLimitBytes+64))
	}))
	t.Cleanup(srv.Close)

	client, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-big"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("providers.New: %v", err)
	}
	testBody, format := chooseTestProbe(providers.TypeOpenAI)
	err = requestUpstream(context.Background(), client, http.Header{}, "gpt-4.1", testBody, format)
	if err == nil || !strings.Contains(err.Error(), "read cap") {
		t.Fatalf("requestUpstream(oversize) err = %v, want 含 read cap 的形状不可验错误", err)
	}
}

// TestChooseTestProbe 锁「请求形状与响应形状同源」：健康检查体与响应解析口径必须来自
// 同一个 provider type 的元数据，发 A 形状的请求却按 B 形状判响应会让形状门变随机误判源。
func TestChooseTestProbe(t *testing.T) {
	cases := []struct {
		providerType string
		wantFormat   consts.WireFormat
	}{
		{providers.TypeOpenAI, consts.FormatOpenAIChat},
		{providers.TypeOpenAIRes, consts.FormatOpenAIResponses},
		{providers.TypeAnthropic, consts.FormatAnthropic},
		{"unknown-type", consts.FormatOpenAIChat}, // 未知 type 与 chooseTestBody 的 OpenAI 回退同口径
	}
	for _, tc := range cases {
		body, format := chooseTestProbe(tc.providerType)
		if format != tc.wantFormat {
			t.Fatalf("chooseTestProbe(%q) format = %s, want %s", tc.providerType, format, tc.wantFormat)
		}
		if len(body) == 0 {
			t.Fatalf("chooseTestProbe(%q) body 为空, want 非空探针体", tc.providerType)
		}
	}
}

// TestRequestUpstream_ReadErrorFails 锁「读不到 ≠ 空回复」：上游回 200 后 body 读中断
// （Content-Length 大于实写字节 → unexpected EOF）不得被「空回复判过」误判健康——
// 吞掉读错误会让断连的上游在健康检查里显示绿（Stage 19 审查回批 Major 1）。
func TestRequestUpstream_ReadErrorFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "5")
		w.WriteHeader(http.StatusOK)
		// 不写 body 直接返回：客户端读到 unexpected EOF、0 字节。
	}))
	t.Cleanup(srv.Close)

	client, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-read"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("providers.New: %v", err)
	}
	testBody, format := chooseTestProbe(providers.TypeOpenAI)
	err = requestUpstream(context.Background(), client, http.Header{}, "gpt-4.1", testBody, format)
	if err == nil || !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("requestUpstream(读中断) err = %v, want 含 read failed 的读体失败错误", err)
	}
}

// TestRequestUpstream_EmptyBodyPassesOn201 组合边界：201 + 空体两个口径叠在一起时仍判过
// （通过集含 201 × 空回复判健康），此前两口径各测一次、组合态没锁。
func TestRequestUpstream_EmptyBodyPassesOn201(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(srv.Close)

	client, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-201e"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("providers.New: %v", err)
	}
	testBody, format := chooseTestProbe(providers.TypeOpenAI)
	if err := requestUpstream(context.Background(), client, http.Header{}, "gpt-4.1", testBody, format); err != nil {
		t.Fatalf("requestUpstream(201+空体) = %v, want nil", err)
	}
}
