package providers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

// TestModels_FailsOnNon2xx 对照侧：放宽到 2xx 不含 3xx/4xx/5xx，错误文本仍带状态码与原文。
func TestModels_FailsOnNon2xx(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError} {
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
	}
}
