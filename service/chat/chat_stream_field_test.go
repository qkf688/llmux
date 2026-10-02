package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// 入站 stream 字段的落库契约：executeSingleProviderAttempt 建行时从
// input.Before.Stream 拷贝到 ChatLog.Stream。落点 = service/chat/chat_attempt.go
// logEntry 构造（与 ApiKeyID/ApiKeyName/RequestID 等快照字段同位）。失败出口
// 走 struct Updates 零值跳过语义，建行值自动贯穿全部失败出口——本用例只在
// 建行路径验证一次即可（与 chat_log_attribution_test 的归责写法同形）。
//
// 退化 / 变异：把 chat_attempt.go 的 `Stream: input.Before.Stream` 改成
// `Stream: false` 即红；缺字段（漏写该行）即红。
func TestExecuteSingleProviderAttempt_StreamField_WrittenOnCreate(t *testing.T) {
	initChatRecordTestDB(t)
	providerRow, mwp := createAttributionFixtureProvider(t, "p-stream")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream boom"}`))
	}))
	defer srv.Close()
	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:   context.Background(),
		Style: string(consts.StyleOpenAI),
		Before: Before{
			Model: "m1", Stream: true,
			raw: []byte(`{"model":"m1","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
		},
		RealModelName:     "m1",
		ReqMeta:           models.ReqMeta{Header: http.Header{}},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		Selection:         selectionForEndpoint(string(consts.ProtocolOpenAI)),
		ChatModel:         chatModel,
		Client:            srv.Client(),
	}, make(chan models.ChatLog))

	var got models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p-stream").First(&got).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Stream != true {
		t.Fatalf("Stream = %v, want true（建行须从 Before.Stream 拷贝，缺拷贝即红）", got.Stream)
	}
}

// 非流式入站 → 落库 false，且不与 Go 零值"不可区分"——通过显式 Before.Stream: false
// + 显式断言确认写侧没有默认 true。落点同上。
func TestExecuteSingleProviderAttempt_StreamField_FalseOnCreate(t *testing.T) {
	initChatRecordTestDB(t)
	providerRow, mwp := createAttributionFixtureProvider(t, "p-nonstream")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:   context.Background(),
		Style: string(consts.StyleOpenAI),
		Before: Before{
			Model: "m1", Stream: false,
			raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`),
		},
		RealModelName:     "m1",
		ReqMeta:           models.ReqMeta{Header: http.Header{}},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		Selection:         selectionForEndpoint(string(consts.ProtocolOpenAI)),
		ChatModel:         chatModel,
		Client:            srv.Client(),
	}, make(chan models.ChatLog))

	var got models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p-nonstream").First(&got).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Stream != false {
		t.Fatalf("Stream = %v, want false", got.Stream)
	}
}
