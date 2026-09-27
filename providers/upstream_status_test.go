package providers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestModels_PassesOn201 锁「拉模型列表」的通过口径与全项目同源（Stage 5）。
// 两家供应商的 Models() 是各自独立的实现、各自一行判定，故表驱动一次钉两处。
// 断言必须落在解析出的列表上：只断 err==nil 会被空表 `{"data":[]}` 蒙混过关。
func TestModels_PassesOn201(t *testing.T) {
	cases := []struct {
		name string
		typ  string
		body string
		want string
	}{
		{
			name: "openai",
			typ:  TypeOpenAI,
			body: `{"object":"list","data":[{"id":"gpt-4.1","object":"model","created":1700000000,"owned_by":"o"}]}`,
			want: "gpt-4.1",
		},
		{
			name: "anthropic",
			typ:  TypeAnthropic,
			body: `{"data":[{"id":"claude-sonnet-4-5","display_name":"S","created_at":"2023-11-01T00:00:00Z","type":"model"}],"has_more":false}`,
			want: "claude-sonnet-4-5",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hits int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			// 刻意不写 custom_models：非空会短路直接返回，根本不打上游。
			p, err := New(tc.typ, fmt.Sprintf(`{"base_url":%q,"api_key":"k","version":"2023-06-01"}`, srv.URL), "")
			if err != nil {
				t.Fatalf("New(%s): %v", tc.typ, err)
			}

			list, err := p.Models(t.Context())
			if err != nil {
				t.Fatalf("Models(201) = %v, want nil", err)
			}
			if len(list) != 1 || list[0].ID != tc.want {
				t.Fatalf("list = %+v, want 1 条 %q", list, tc.want)
			}
			if hits != 1 {
				t.Fatalf("上游命中次数 = %d, want 1", hits)
			}
		})
	}
}

// TestModels_FailsOnNonPassingStatus 通过集边界：只带上 201，不含 204/3xx/4xx/5xx。
// 断言打在错误文本上（必须回显状态码）——放宽后「2xx 但不是清单」的可达面变宽，只回一个
// `EOF` 会让模型同步日志查不出断在哪一段。
func TestModels_FailsOnNonPassingStatus(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusMultipleChoices, http.StatusUnauthorized, http.StatusInternalServerError} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"denied"}}`))
		}))

		p, err := New(TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, srv.URL), "")
		if err != nil {
			srv.Close()
			t.Fatalf("New: %v", err)
		}
		_, err = p.Models(t.Context())
		srv.Close()

		if err == nil {
			t.Fatalf("status %d 应判失败, got nil", status)
		}
		if !strings.Contains(err.Error(), fmt.Sprintf("%d", status)) {
			t.Fatalf("status %d 的错误须回显状态码, got %q", status, err.Error())
		}
	}
}

// TestModels_PassingStatusWithNonListBody 状态码通过但内容不是清单（被反代改写成登录页
// 那一类）：判定不看内容（Stage 5 共识第 4 条，已登记残留待办），但解码错误必须带状态码。
func TestModels_PassingStatusWithNonListBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html>login required</html>`))
	}))
	defer srv.Close()

	p, err := New(TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = p.Models(t.Context())
	if err == nil {
		t.Fatal("HTML 响应体应解码失败, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 200") {
		t.Fatalf("解码错误应带上游状态码, got %q", err.Error())
	}
}

// TestModels_OversizedListBodyRejected 锁 Stage 27 读体上限（此前成功路径无上限整读，
// 上游异常/被篡改时可被灌进任意体量）：200 + 超过 common.ModelsListReadLimitBytes 的
// 清单体必须按解码失败处置，而不是整块读进内存再解析。
func TestModels_OversizedListBodyRejected(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// 写一块合法 JSON 前缀 + 巨量填充，体量超 10 MiB 上限。
		_, _ = w.Write([]byte(`{"data":[{"id":"m1",`))
		padding := strings.Repeat("0", 1<<20)
		for i := 0; i < 12; i++ {
			_, _ = w.Write([]byte(padding))
		}
	}))
	defer srv.Close()

	p, err := New(TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = p.Models(t.Context())
	if err == nil {
		t.Fatal("超限清单体应判失败, got nil")
	}
	if !strings.Contains(err.Error(), "read cap") {
		t.Fatalf("err = %v, want 含 read cap（超限语义，而非普通解码错误）", err)
	}
}
