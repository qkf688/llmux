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

// TestRunAnthropicReactTest_PassesOn201 锁 ReAct 能力测试的通过口径与全项目同源（Stage 5）。
// 放宽前回 201 会被拼成「提供商返回错误」的详情文案推给前端，管理员只能看到一个假故障。
// 直调 unexported runner：走完整 handler 需要 seed 整条链路，而工具场景校验随后必然报
// 另一种错，反而断不准状态码这一条。
func TestRunAnthropicReactTest_PassesOn201(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	t.Cleanup(srv.Close)

	client, err := providers.New(providers.TypeAnthropic, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-201","version":"2023-06-01"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("providers.New: %v", err)
	}
	link := &testLink{
		Client:       client,
		Type:         providers.TypeAnthropic,
		ProviderName: "p-anthropic-201",
		Model:        "claude-sonnet-4-5",
		Group:        models.KeyGroup{Name: "g1"},
	}
	c, rec := testsupport.NewTestContext("GET", "/api/test/react/1")

	_, final, err := runAnthropicReactTest(context.Background(), c, srv.Client(), http.Header{}, link, client, reactScenario{Question: "hi"})
	if err != nil {
		t.Fatalf("runAnthropicReactTest(201) = %v, want nil；SSE=%s", err, rec.Body.String())
	}
	if final != "ok" {
		t.Fatalf("final = %q, want %q（必须真解析出回复内容）", final, "ok")
	}
	if strings.Contains(rec.Body.String(), "提供商返回错误") {
		t.Fatalf("SSE 仍在报「提供商返回错误」: %s", rec.Body.String())
	}
	if hits != 1 {
		t.Fatalf("上游命中次数 = %d, want 1", hits)
	}
}
