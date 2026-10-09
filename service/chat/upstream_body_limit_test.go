package chat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// buildAttemptInputAgainstURL 建一次 attempt 所需的供应商/关联/模型夹具并组装入参。
// 与 runAttemptAgainstURL 拆开，是因为「上游阻塞期间行停在 processing」类用例要在
// goroutine 里跑 attempt、而 t.Fatalf 只能出现在测试 goroutine——夹具在这里先用 t 建好。
func buildAttemptInputAgainstURL(t *testing.T, providerName, url string, client *http.Client, ctx context.Context, reqRaw string) singleProviderAttemptInput {
	t.Helper()
	providerRow := models.Provider{Name: providerName, Type: providers.TypeOpenAI}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	enabled := true
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1", Status: &enabled}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-limit"}`, url), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	return singleProviderAttemptInput{
		Ctx:           ctx,
		Start:         time.Now(),
		Style:         string(consts.StyleOpenAI),
		Before:        Before{Model: "m1", raw: []byte(reqRaw)},
		RealModelName: "m1",
		// rid 快照（Stage 17）：测试常量值，断言建行点把 ReqMeta.RequestID 定格进日志行。
		ReqMeta:           models.ReqMeta{Header: http.Header{}, RequestID: consts.RequestIDPrefixGenerated + "attempt-test"},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		Selection:         selectionForEndpoint(string(consts.ProtocolOpenAI)),
		ChatModel:         chatModel,
		Client:            client,
	}
}

// runAttemptAgainstURL 以 OpenAI 协议直通形态对指定上游跑一次 attempt（Stage 2-C/D 测试共用）。
// reqRaw 为入站原始请求体（原始采集截断测试需要超大 raw）。
func runAttemptAgainstURL(t *testing.T, providerName, url string, client *http.Client, ctx context.Context, reqRaw string) singleProviderAttemptResult {
	t.Helper()
	return executeSingleProviderAttempt(buildAttemptInputAgainstURL(t, providerName, url, client, ctx, reqRaw), make(chan models.ChatLog, 8))
}

const smallReqRaw = `{"model":"m1","messages":[{"role":"user","content":"hi"}]}`

// Stage 2-C 超限出口：33 MiB > 默认 32 MiB → **组织级**淘汰（换下一家）、不标凭据级、
// 不写冷却；错误日志带超限原因。锁死共识里「超大 body 是端点级行为，与 key 无关」。
func TestLimitUpstreamResponseBody_OverflowFailsOverWithoutCooldown(t *testing.T) {
	initChatRecordTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("a", 33<<20)))
	}))
	t.Cleanup(srv.Close)

	result := runAttemptAgainstURL(t, "p-ovf", srv.URL, srv.Client(), context.Background(), smallReqRaw)
	if result.Success {
		t.Fatalf("超限响应不得判成功")
	}
	if result.CredentialFailure {
		t.Fatalf("超限是端点级异常, 不得标凭据级失败: %+v", result)
	}
	if !result.RemoveWeight || !result.RemovePriority {
		t.Fatalf("超限应组织级淘汰候选: %+v", result)
	}

	var row models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p-ovf").First(&row).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if row.Status != "error" || !strings.Contains(row.Error, "exceeds configured limit") {
		t.Fatalf("log = %q/%q, want error + 超限原因", row.Status, row.Error)
	}
}

// Stage 2-C 正常出口：闸内响应必须原样可重读（回填不吞尾字节、不改内容）——
// 采集/转换/直通 io.Copy 都从这份内存字节再读一遍，闸不能成为数据完整性回归点。
func TestLimitUpstreamResponseBody_UnderLimitBodySurvivesIntact(t *testing.T) {
	initChatRecordTestDB(t)

	payload := `{"id":"chatcmpl-l","object":"chat.completion","created":0,"model":"pm1","choices":[{"index":0,"message":{"role":"assistant","content":"` + strings.Repeat("字", 5000) + `"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, payload)
	}))
	t.Cleanup(srv.Close)

	result := runAttemptAgainstURL(t, "p-intact", srv.URL, srv.Client(), context.Background(), smallReqRaw)
	if !result.Success {
		t.Fatalf("闸内响应应判成功: %+v", result)
	}
	defer result.Response.Body.Close()
	got, err := io.ReadAll(result.Response.Body)
	if err != nil {
		t.Fatalf("read restored body: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("回填后 body 不完整: got %d bytes, want %d", len(got), len(payload))
	}
}

// Stage 2-C 读体超时出口：headers 先到、body 滴灌中被 ctx deadline 打断 →
// 必须与 Do 分支成对归类为**凭据级网络**失败（RULES「Go HTTP 超时：body 是懒流」——
// 分类只挂 Do 侧则此路径静默漏走）。
func TestLimitUpstreamResponseBody_ReadDeadlineIsNetworkClass(t *testing.T) {
	initChatRecordTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush() // headers 先到达，把失败点逼进读体阶段
		time.Sleep(800 * time.Millisecond)
		_, _ = io.WriteString(w, `{"id":"late"}`)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	result := runAttemptAgainstURL(t, "p-deadline", srv.URL, srv.Client(), ctx, smallReqRaw)
	if result.Success {
		t.Fatalf("读体超时不得判成功")
	}
	if !result.CredentialFailure {
		t.Fatalf("读体超时应与 Do 失败同归凭据级网络: %+v", result)
	}

	// Stage 17 回归（AC-6/AC-5）：超时场景最需要留痕——失败出口回写不得陪葬请求
	// ctx，行终态必须是 error 且带错误原因（改前该行停在建行占位的 success，
	// 日志页/指标把超时计为成功）；rid 快照须在建行时定格进日志行。
	var row models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p-deadline").First(&row).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if row.Status != "error" || row.Error == "" {
		t.Fatalf("超时行终态 = %q/%q, want error + 非空原因（回写不得陪葬请求 ctx）", row.Status, row.Error)
	}
	if row.RequestID != consts.RequestIDPrefixGenerated+"attempt-test" {
		t.Fatalf("日志行 request_id = %q, want 建行时定格的 ReqMeta 快照", row.RequestID)
	}
}
