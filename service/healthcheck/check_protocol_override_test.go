package healthcheck

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/qkf688/llmux/models"
)

// TestDoCheck_FollowsAssociationProtocolOverride 锁 AC-8：关联覆盖主格式后，健康检查
// 必须打这条协议的路径，并且探针体/响应形状按**选中端点协议**派生。
//
// 不跟的代价：测的是供应商默认路径，与这个模型真实走的路无关——一边自动禁用完全可用的
// 关联，一边对真实断掉的链路报健康。两个分支都必须断言路径不同，只测「覆盖态能通」
// 会让「始终走 provider.Type」的实现也全绿。
func TestDoCheck_FollowsAssociationProtocolOverride(t *testing.T) {
	anthropic := "anthropic"

	cases := []struct {
		name     string
		primary  *string
		wantPath string
		// 探针体是否带 `max_tokens`：OpenAI 体没有、Anthropic 体有。用它锁「探针体按选中端点
		// 协议派生」——只断路径的话，把 chooseTestProbe 改回 provider.Type 仍然全绿。
		wantMaxTokens bool
	}{
		{name: "继承供应商主协议", primary: nil, wantPath: "/chat/completions", wantMaxTokens: false},
		{name: "关联主格式 anthropic", primary: &anthropic, wantPath: "/messages", wantMaxTokens: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initHealthCheckTestDB(t)
			cipher := setupHealthCheckCipher(t)

			var path atomic.Value
			var body atomic.Value
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				body.Store(string(raw))
				path.Store(r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/messages") {
					_, _ = w.Write([]byte(`{"id":"msg-1","type":"message","model":"m","role":"assistant",` +
						`"content":[{"type":"text","text":"hi"}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":"cmpl-1","object":"chat.completion","model":"m","choices":[]}`))
			}))
			t.Cleanup(srv.Close)

			provider := models.Provider{
				Name:   "p-mixed",
				Type:   "openai",
				Config: `{"base_url":"https://should-not-use.example"}`,
			}
			if err := models.DB.Create(&provider).Error; err != nil {
				t.Fatalf("create provider: %v", err)
			}
			// 同一 base_url 下两条协议端点，路径由协议决定 → 用路径区分走了哪条。
			seedEndpoint(t, provider.ID, "openai", srv.URL)
			seedEndpoint(t, provider.ID, "anthropic", srv.URL)
			_, _ = seedPoolGroup(t, cipher, provider.ID, "g1", "sk-both")

			mp := &models.ModelWithProvider{ProviderModel: "m", PrimaryProtocol: tc.primary}
			if err := (&HealthChecker{}).doCheck(context.Background(), &provider, mp); err != nil {
				t.Fatalf("doCheck: %v", err)
			}
			if got, _ := path.Load().(string); got != tc.wantPath {
				t.Fatalf("upstream path = %q, want %q", got, tc.wantPath)
			}
			if got, _ := body.Load().(string); strings.Contains(got, `"max_tokens"`) != tc.wantMaxTokens {
				t.Fatalf("probe body max_tokens presence = %v, want %v; body=%s",
					strings.Contains(got, `"max_tokens"`), tc.wantMaxTokens, got)
			}
		})
	}
}
