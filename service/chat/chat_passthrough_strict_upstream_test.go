package chat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/tidwall/gjson"
)

// strictAnthropicViolation 复刻严格 Anthropic 上游的校验口径：messages[].content[] 的块 type
// 必须在规范集合内（DashScope 的 /apps/anthropic/v1 即如此），越界即 400
// `InvalidParameter "Request body format invalid"`。判定口径由 2026-09-30 的最小复现钉死：
// 同位置的纯文本 system 消息被接受，换成 tool_addition 块即被拒。
func strictAnthropicViolation(body string) string {
	allowed := map[string]bool{
		"text": true, "image": true, "tool_use": true,
		"tool_result": true, "thinking": true, "redacted_thinking": true,
	}
	for _, msg := range gjson.Get(body, "messages").Array() {
		content := msg.Get("content")
		if !content.IsArray() {
			continue
		}
		for _, block := range content.Array() {
			if t := block.Get("type").String(); t != "" && !allowed[t] {
				return t
			}
		}
	}
	return ""
}

// 假上游校验器的自检：事故 body 必须被判越界，否则上面那条端到端测试会因「假上游永远放行」
// 退化成空跑（回不去红的测试不算回归网）。
func TestStrictAnthropicViolation_FlagsIncidentBody(t *testing.T) {
	if got := strictAnthropicViolation(incidentRawBody); got != "tool_addition" {
		t.Fatalf("violation = %q, want tool_addition（假上游必须能拒收事故形状）", got)
	}
	canonical := `{"model":"m","max_tokens":64,"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`
	if got := strictAnthropicViolation(canonical); got != "" {
		t.Fatalf("canonical body 被误判越界: %q", got)
	}
}

// 端到端回归（事故 2026-09-30 / 日志 11517）：客户端风格 anthropic、命中 anthropic 端点 →
// 直通 → 私有扩展块必须已经被收掉，严格上游才会接受。本测试钉的是**到达上游的字节**：
// 单元测试只断言 body 构造，这里断言整条链路（BalanceChat → 构建 → POST 上游）。
func TestBalanceChat_AnthropicPassthrough_StrictUpstreamAcceptsAfterNormalization(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	var mu sync.Mutex
	var bodies []string
	strictSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()

		if offending := strictAnthropicViolation(body); offending != "" {
			// 真实上游的报文形状：400 + SSE error 体（llmux 落库的 error 字段就是这个形状）。
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("event:error\ndata:{\"code\":\"InvalidParameter\",\"message\":\"Request body format invalid\"}\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"pm1",` +
			`"content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn",` +
			`"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer strictSrv.Close()

	model := models.Model{Name: "deepseek-v4.1-flash", SupportsThinking: true}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	provider := models.Provider{Name: "ali-strict", Type: providers.TypeAnthropic}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolAnthropic), URL: strictSrv.URL, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	_, _, mwp := seedGroupedChannel(t, cipher, provider.ID, "g1", model.ID, 10, 1, []string{"sk-any"}, nil)

	pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{mwp})
	if err != nil {
		t.Fatalf("build candidate pool: %v", err)
	}
	result, err := BalanceChat(context.Background(), BalanceInput{
		Start:  time.Now(),
		Style:  string(consts.StyleAnthropic),
		Before: Before{Model: "deepseek-v4.1-flash", Stream: false, raw: []byte(incidentRawBody)},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &model,
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err != nil {
		t.Fatalf("BalanceChat err = %v（严格上游拒收了到达的 body）", err)
	}
	if result == nil || result.ProviderName != "ali-strict" {
		t.Fatalf("result = %+v, want provider ali-strict", result)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) == 0 {
		t.Fatal("上游一次都没被打到")
	}
	sent := bodies[0]
	if offending := strictAnthropicViolation(sent); offending != "" {
		t.Fatalf("到达上游的 body 仍含越界块 %q:\n%s", offending, sent)
	}
	if strings.Contains(sent, "tool_addition") || strings.Contains(sent, "tool_reference") {
		t.Fatalf("到达上游的 body 仍含私有扩展:\n%s", sent)
	}
	if !strings.Contains(sent, "Tool added: read_page") {
		t.Fatalf("工具变更语义丢了（应已渲染成文本）:\n%s", sent)
	}
}
