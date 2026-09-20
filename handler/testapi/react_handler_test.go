package testapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/qkf688/llmux/handler/testsupport"
)

func TestSelectScenarioIndex(t *testing.T) {
	t.Run("stable default", func(t *testing.T) {
		idx, err := selectScenarioIndex("model-123", "", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx < 0 || idx >= 3 {
			t.Fatalf("index out of range: %d", idx)
		}
		idx2, err := selectScenarioIndex("model-123", "", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx2 != idx {
			t.Fatalf("expected stable index, got %d then %d", idx, idx2)
		}
	})

	t.Run("explicit index", func(t *testing.T) {
		idx, err := selectScenarioIndex("model-123", "1", 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if idx != 1 {
			t.Fatalf("idx = %d, want %d", idx, 1)
		}
	})

	t.Run("invalid index", func(t *testing.T) {
		if _, err := selectScenarioIndex("model-123", "not-a-number", 3); err == nil {
			t.Fatal("expected error")
		}
		if _, err := selectScenarioIndex("model-123", "9", 3); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestNormalizeCity(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "南京", want: "南京"},
		{raw: "南京市", want: "南京"},
		{raw: "Nanjing", want: "南京"},
		{raw: "Beijing, China", want: "北京"},
		{raw: "广州", want: "广州"},
		{raw: "unknown-city", want: "unknown-city"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.raw, func(t *testing.T) {
			if got := normalizeCity(tc.raw); got != tc.want {
				t.Fatalf("normalizeCity(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestValidateReactToolCalls(t *testing.T) {
	tests := []struct {
		name         string
		toolCities   []string
		expected     []string
		expectErrSub string
	}{
		{
			name:       "ok any order",
			toolCities: []string{"北京", "南京"},
			expected:   []string{"南京", "北京"},
		},
		{
			name:       "ok extra calls",
			toolCities: []string{"南京", "北京", "南京"},
			expected:   []string{"南京", "北京"},
		},
		{
			name:         "missing city",
			toolCities:   []string{"南京"},
			expected:     []string{"南京", "北京"},
			expectErrSub: "次数过少",
		},
		{
			name:         "missing city with enough calls",
			toolCities:   []string{"南京", "南京"},
			expected:     []string{"南京", "北京"},
			expectErrSub: "缺少城市",
		},
		{
			name:         "unexpected city",
			toolCities:   []string{"南京", "杭州"},
			expected:     []string{"南京", "北京"},
			expectErrSub: "非预期城市",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateReactToolCalls(tc.toolCities, tc.expected)
			if tc.expectErrSub == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expectErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.expectErrSub)
				}
				if !strings.Contains(err.Error(), tc.expectErrSub) {
					t.Fatalf("error = %q, want contains %q", err.Error(), tc.expectErrSub)
				}
			}
		})
	}
}

func TestValidateReactFinalResponse(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		expected     []string
		expectErrSub string
	}{
		{
			name:     "ok chinese",
			text:     "南京天气晴转多云，温度 18℃；北京天气大雨转小雨，温度 15℃。",
			expected: []string{"南京", "北京"},
		},
		{
			name:     "ok english",
			text:     "Beijing weather is rainy; Nanjing weather is cloudy. Temperature details above.",
			expected: []string{"南京", "北京"},
		},
		{
			name:         "missing city",
			text:         "南京天气不错，温度 18℃。",
			expected:     []string{"南京", "北京"},
			expectErrSub: "缺少城市",
		},
		{
			name:         "missing weather keyword",
			text:         "南京和北京我都看过了。",
			expected:     []string{"南京", "北京"},
			expectErrSub: "缺少天气",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateReactFinalResponse(tc.text, tc.expected)
			if tc.expectErrSub == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.expectErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.expectErrSub)
				}
				if !strings.Contains(err.Error(), tc.expectErrSub) {
					t.Fatalf("error = %q, want contains %q", err.Error(), tc.expectErrSub)
				}
			}
		})
	}
}

func TestBuildWeatherStub(t *testing.T) {
	got, ok := buildWeatherStub("北京", "fahrenheit")
	if !ok {
		t.Fatal("expected ok")
	}
	if !strings.Contains(got, "°F") {
		t.Fatalf("expected fahrenheit output, got: %s", got)
	}
}

func TestWeatherFromArguments(t *testing.T) {
	city, res := weatherFromArguments(`{"location":"南京市","unit":"celsius"}`)
	if city != "南京" {
		t.Fatalf("city = %q, want %q", city, "南京")
	}
	if !strings.Contains(res, "南京") || !strings.Contains(res, "℃") {
		t.Fatalf("unexpected response: %s", res)
	}
}

// TestTestReactHandler_NonSSEUpstreamEmitsErrorEvent 上游对流式请求回非 SSE 的 JSON 体
// （choices 为空）时，第三方库 nsxno/react 内部对空切片取下标 panic（agent.go）。该 panic
// 属上游不符合流式约定触发的库缺陷，非本地逻辑错误；runOpenAIReactTest 的 recover 兜底
// 把它转成 checkError，管理端经 SSE error 事件看到可读失败，而不是流中途被硬切断后
// 「卡住后失败」。本用例锁兜底契约：不 panic + 回 error 事件 + 不误报 success。
func TestTestReactHandler_NonSSEUpstreamEmitsErrorEvent(t *testing.T) {
	testsupport.InitTestDB(t)
	cipher := setupTestAPICipher(t)

	var hits atomic.Int64
	// 与 completionHandlerByAuth 的关键差异：无论请求体是否 "stream":true，
	// 一律回非 SSE 的 JSON（choices 空）——复现库内取下标 panic 的触发条件。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-1","object":"chat.completion","model":"m","choices":[]}`))
	}))
	t.Cleanup(srv.Close)

	provider := seedProviderRow(t, "p-react-nonsse", "openai")
	seedEndpointRow(t, provider.ID, "openai", srv.URL)
	seedPoolGroup(t, cipher, provider.ID, "react 池组", "pool-react-nonsse", "sk-react-nonsse")
	mwp := seedAssociation(t, provider.ID, 1, "gpt-4.1")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/test/react/"+strconv.FormatUint(uint64(mwp.ID), 10), nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(mwp.ID), 10)}}

	// 兜底缺失时库内 panic 从这里直穿上抛，测试进程挂 = 用例红（改前实测）。
	TestReactHandler(c)

	if hits.Load() == 0 {
		t.Fatalf("未打到上游（装配阶段就失败）；SSE=%s", w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "event:error") {
		t.Fatalf("SSE 未回 error 事件；body=%s", body)
	}
	if strings.Contains(body, "event:success") {
		t.Fatalf("非 SSE 上游不应报 success；body=%s", body)
	}
}
