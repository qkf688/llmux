package chat

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
	"github.com/qkf688/llmux/service/channel"
)

// 提交门（commit-before-write）行为锁：上游回 200 但内容不可提交（首帧错误载荷 /
// 零 SSE 帧 / 非法 JSON / 错误对象）时，该尝试必须判失败并自动换下一家，客户端
// 最终只见到好候选的字节——「错误不下沉」的端到端语义。改前实测红：现状拿到 200
// 即判成功，坏内容原样回给客户端。

// setupGateFailoverProvider 建独立 provider（端点/组/凭据/关联），挂到共享 modelID 上。
// priority 控制选路次序（高者恒先），保证「坏候选先上、失败后换好候选」可断言。
func setupGateFailoverProvider(t *testing.T, cipher *credentialcrypto.Cipher, modelID uint, name, epURL string, priority int) models.ModelWithProvider {
	t.Helper()
	provider := models.Provider{Name: "gate-" + name, Type: providers.TypeOpenAI, Config: `{"base_url":"http://127.0.0.1:1/never"}`}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider %s: %v", name, err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), URL: epURL, Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint %s: %v", name, err)
	}
	grp := models.KeyGroup{ProviderID: provider.ID, Name: "g-" + name, Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group %s: %v", name, err)
	}
	plain := "sk-gate-" + name
	cred := models.Credential{GroupID: &grp.ID, Key: mustEncrypt(t, cipher, plain), KeyHash: cipher.Hash(plain)}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential %s: %v", name, err)
	}
	mwp := models.ModelWithProvider{ModelID: modelID, ProviderID: provider.ID, ProviderModel: "pm-" + name, Weight: 10, Priority: priority}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association %s: %v", name, err)
	}
	return mwp
}

// runBalanceOverPool 以给定候选池跑 BalanceChat，返回 (客户端应收到的字节, 错误)。
func runBalanceOverPool(t *testing.T, stream bool, modelName string, raw []byte, mwps []models.ModelWithProvider) ([]byte, error) {
	t.Helper()
	pool, err := buildCandidatePool(context.Background(), mwps)
	if err != nil {
		t.Fatalf("build candidate pool: %v", err)
	}
	result, err := BalanceChat(context.Background(), BalanceInput{
		Start:  time.Now(),
		Style:  string(consts.StyleOpenAI),
		Before: Before{Model: modelName, Stream: stream, raw: raw},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: modelName},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err != nil {
		return nil, err
	}
	defer result.Response.Body.Close()
	body, err := io.ReadAll(result.Response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return body, nil
}

// AC-1：流式首帧是错误载荷 → 坏候选判失败自动换下家，客户端只收到好候选的字节。
func TestBalanceChat_Stream_ErrorFirstFrame_FailsOver(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)

	const badSSE = "data: {\"error\":{\"message\":\"upstream boom\",\"type\":\"server_error\"}}\n\n"
	const goodSSE = "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"}}]}\n\n" +
		"data: [DONE]\n\n"

	var badHits, goodHits atomic.Int32
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		badHits.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(badSSE))
	}))
	t.Cleanup(bad.Close)
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodHits.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(goodSSE))
	}))
	t.Cleanup(good.Close)

	model := models.Model{Name: "gate-m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	badMwp := setupGateFailoverProvider(t, cipher, model.ID, "bad", bad.URL, 2)
	goodMwp := setupGateFailoverProvider(t, cipher, model.ID, "good", good.URL, 1)

	got, err := runBalanceOverPool(t, true, "gate-m1",
		[]byte(`{"model":"gate-m1","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
		[]models.ModelWithProvider{badMwp, goodMwp})
	if err != nil {
		t.Fatalf("BalanceChat err = %v, want 换下家成功", err)
	}
	if string(got) != goodSSE {
		t.Fatalf("client stream =\n%q\nwant 好候选字节原样:\n%q", got, goodSSE)
	}
	if badHits.Load() != 1 {
		t.Fatalf("bad hits = %d, want 1", badHits.Load())
	}
	if goodHits.Load() != 1 {
		t.Fatalf("good hits = %d, want 1", goodHits.Load())
	}
}

// AC-2：流式请求但上游回 200 + 非 SSE（JSON 错误体）→ 零帧不可提交；
// 全候选如此时 BalanceChat 必须报错（handler 端回 500，客户端零业务字节）。
func TestBalanceChat_Stream_ZeroFrames_AllFail(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":{"message":"gateway returned json, not sse"}}`))
	}))
	t.Cleanup(srv.Close)

	model := models.Model{Name: "gate-m2"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	mwp := setupGateFailoverProvider(t, cipher, model.ID, "json", srv.URL, 1)

	got, err := runBalanceOverPool(t, true, "gate-m2",
		[]byte(`{"model":"gate-m2","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
		[]models.ModelWithProvider{mwp})
	if err == nil {
		t.Fatalf("BalanceChat err = nil (body=%q), want 全候选不可提交时整体失败", got)
	}
}

// AC-3：非流式直通上游回 200 但内容不可提交（HTML / 错误对象 / 空体）→
// 判失败换下家，客户端只收到好候选的合法 JSON。
func TestBalanceChat_NonStream_PassthroughInvalidContent_FailsOver(t *testing.T) {
	const goodBody = `{"id":"chatcmpl-1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`
	cases := []struct {
		name string
		body string
	}{
		{"HTML登录页", `<html><body>login</body></html>`},
		{"错误对象", `{"error":{"message":"quota exhausted","type":"insufficient_quota"}}`},
		{"空体", ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initChatRecordTestDB(t)
			cipher := setupChatChannelCipher(t)
			channel.ResetDefaultSelectorForTest()
			t.Cleanup(channel.ResetDefaultSelectorForTest)

			var badHits, goodHits atomic.Int32
			bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				badHits.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(bad.Close)
			good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				goodHits.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(goodBody))
			}))
			t.Cleanup(good.Close)

			model := models.Model{Name: "gate-m3-" + tc.name}
			if err := models.DB.Create(&model).Error; err != nil {
				t.Fatalf("create model: %v", err)
			}
			badMwp := setupGateFailoverProvider(t, cipher, model.ID, "bad-"+tc.name, bad.URL, 2)
			goodMwp := setupGateFailoverProvider(t, cipher, model.ID, "good-"+tc.name, good.URL, 1)

			got, err := runBalanceOverPool(t, false, model.Name,
				[]byte(`{"model":"`+model.Name+`","messages":[{"role":"user","content":"hi"}]}`),
				[]models.ModelWithProvider{badMwp, goodMwp})
			if err != nil {
				t.Fatalf("BalanceChat err = %v, want 换下家成功", err)
			}
			if string(got) != goodBody {
				t.Fatalf("client body = %q, want 好候选 JSON 原样: %q", got, goodBody)
			}
			if badHits.Load() != 1 || goodHits.Load() != 1 {
				t.Fatalf("hits bad=%d good=%d, want 各 1", badHits.Load(), goodHits.Load())
			}
		})
	}
}
