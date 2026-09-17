package testapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qkf688/llmux/handler/testsupport"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// newAnthropicUpstream 起一个按指定状态码回 Anthropic 形状回复的假上游，返回 provider 实例、
// 命中计数与请求出口。回复无 tool_calls，故 ReAct loop 一步即结束（hits 恒 1）。
func newAnthropicUpstream(t *testing.T, status int) (providers.Provider, *http.Client, *int, *testLink) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		if status != http.StatusNoContent {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		if status != http.StatusNoContent {
			_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`))
		}
	}))
	t.Cleanup(srv.Close)

	client, err := providers.New(providers.TypeAnthropic, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-1","version":"2023-06-01"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("providers.New: %v", err)
	}
	link := &testLink{
		Client:       client,
		Type:         providers.TypeAnthropic,
		ProviderName: "p-anthropic",
		Model:        "claude-sonnet-4-5",
		Group:        models.KeyGroup{Name: "g1"},
	}
	return client, srv.Client(), &hits, link
}

// runReact 直调 unexported runner（走完整 handler 需要 seed 整条链路，而工具场景校验随后
// 必然报另一种错，反而断不准状态码这一条）。
func runReact(t *testing.T, client providers.Provider, httpClient *http.Client, link *testLink) (string, string, error) {
	t.Helper()
	c, rec := testsupport.NewTestContext("GET", "/api/test/react/1")
	_, final, err := runAnthropicReactTest(context.Background(), c, httpClient, http.Header{}, link, client, reactScenario{Question: "hi"})
	return final, rec.Body.String(), err
}

// TestRunAnthropicReactTest_PassesOn201 锁 ReAct 能力测试与全项目通过口径同源（Stage 5）。
// 统一前回 201 会被拼成「提供商返回错误」推给前端，管理员只能看到一个假故障。
func TestRunAnthropicReactTest_PassesOn201(t *testing.T) {
	client, httpClient, hits, link := newAnthropicUpstream(t, http.StatusCreated)

	final, sse, err := runReact(t, client, httpClient, link)
	if err != nil {
		t.Fatalf("201 应判通过, got err=%v；SSE=%s", err, sse)
	}
	if final != "ok" {
		t.Fatalf("final = %q, want %q（必须真解析出回复内容）", final, "ok")
	}
	// 成功路径会把回复作为 message 事件推给前端；只断 err==nil 测不到「内容真的到了用户」。
	if !strings.Contains(sse, "message") || !strings.Contains(sse, "ok") {
		t.Fatalf("SSE 未推送回复内容: %q", sse)
	}
	if *hits != 1 {
		t.Fatalf("上游命中次数 = %d, want 1", *hits)
	}
}

// TestRunAnthropicReactTest_FailsOn204 通过集边界：204「成功但无正文」不判通过，且错误
// 里必须带上状态码——放宽后这类上游可达面变宽，只报「解析失败」查不出断在哪一段。
func TestRunAnthropicReactTest_FailsOn204(t *testing.T) {
	client, httpClient, _, link := newAnthropicUpstream(t, http.StatusNoContent)

	_, sse, err := runReact(t, client, httpClient, link)
	if err == nil {
		t.Fatal("204 应判失败, got nil")
	}
	if !strings.Contains(err.Error(), "204") {
		t.Fatalf("失败结论须带上游状态码, got %q；SSE=%s", err.Error(), sse)
	}
}
