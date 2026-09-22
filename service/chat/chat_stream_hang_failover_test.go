package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/channel"
)

// gateHangSettingsReader 测试用 settings.Reader stub：只把首字节看门狗窗口压到
// getter 下限 1 秒（超时最小粒度秒），其余键回退默认值。
type gateHangSettingsReader struct{}

func (gateHangSettingsReader) Bool(_ context.Context, _ string, def bool) bool { return def }

func (gateHangSettingsReader) Int(_ context.Context, key string, def, _ int) int {
	if key == models.SettingKeyStreamFirstByteTimeout {
		return 1
	}
	return def
}

func (gateHangSettingsReader) String(_ context.Context, _ string, def string) string { return def }

// AC-4（#24-2 验收承诺的「挂死 → 自动换下家」集成测试）：上游回 200 + SSE 响应头后
// 一字不发（212 事故形态）→ 首字节看门狗在**提交前**判失败并自动换下家，客户端
// 只收到好候选字节、零错误字节。改造前该形态要等 handler 写完 200 响应头才发现，
// 客户端已收到头无法回收，只能带病收尾。
func TestBalanceChat_Stream_HangAfterHeaders_FailsOver(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	SetSettingsReader(gateHangSettingsReader{})
	t.Cleanup(func() { SetSettingsReader(nil) })
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)

	const goodSSE = "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"}}]}\n\n" +
		"data: [DONE]\n\n"

	var badHits, goodHits atomic.Int32
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			// 惰性探活等非聊天请求快速失败，不占用挂死窗口
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		badHits.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush() // 响应头已送达，正文一字不发
		}
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second): // 兜底防 Server.Close 等待悬挂 handler
		}
	}))
	t.Cleanup(bad.Close)
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		goodHits.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(goodSSE))
	}))
	t.Cleanup(good.Close)

	model := models.Model{Name: "gate-hang-m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	badMwp := setupGateFailoverProvider(t, cipher, model.ID, "bad", bad.URL, 2)
	goodMwp := setupGateFailoverProvider(t, cipher, model.ID, "good", good.URL, 1)

	got, err := runBalanceOverPool(t, true, "gate-hang-m1",
		[]byte(`{"model":"gate-hang-m1","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
		[]models.ModelWithProvider{badMwp, goodMwp})
	if err != nil {
		t.Fatalf("BalanceChat err = %v, want 看门狗判失败后换下家成功", err)
	}
	if string(got) != goodSSE {
		t.Fatalf("client stream =\n%q\nwant 好候选字节原样:\n%q", got, goodSSE)
	}
	if badHits.Load() != 1 {
		t.Fatalf("bad hits = %d, want 1（挂死候选只试一次）", badHits.Load())
	}
	if goodHits.Load() != 1 {
		t.Fatalf("good hits = %d, want 1", goodHits.Load())
	}
}
