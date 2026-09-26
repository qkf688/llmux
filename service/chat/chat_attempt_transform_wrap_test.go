package chat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// TestWrapTransformError_PreservesChain（Stage 25 回归）：errors.Is 必须穿透包装——
// 历史写法 %v 断链，转换失败路径的断连分类（降 Warn 分流）静默失效。此测试直接钉
// 包装函数（实现侧 chat_attempt_error.go）的 unwrap 契约；实现回退成断链包装即红。
func TestWrapTransformError_PreservesChain(t *testing.T) {
	sentinel := errors.New("upstream read interrupted")
	wrapped := wrapTransformError("response", sentinel)
	if !errors.Is(wrapped, sentinel) {
		t.Fatal("errors.Is 穿透 wrapTransformError 失败——错误链断了（回退成断链包装）")
	}
	if got := wrapped.Error(); got != "transform response error: upstream read interrupted" {
		t.Fatalf("文案形状变化: %q（落库 Error 前缀契约，勿无声改动）", got)
	}
	// 判据自测（防假绿）：断链写法（旧实现形态）必须被本测试判红——
	// 用 v 格式动词复刻旧实现，errors.Is 必须不命中，否则测试没有区分度。
	broken := fmt.Errorf("transform response error: %v", sentinel)
	if errors.Is(broken, sentinel) {
		t.Fatal("判据失效：断链包装竟通过了 errors.Is（测试无区分度）")
	}
}

// TestExecuteSingleProviderAttempt_TransformResponseError_ErrorTextPrefix 端到端锁
// 转换失败出口落库的 Error 文本前缀（与 BackfillsProxyTime 用例同夹具），防包装单源
// 被绕开（就近 fmt.Sprintf 复发时前缀漂移）。
func TestExecuteSingleProviderAttempt_TransformResponseError_ErrorTextPrefix(t *testing.T) {
	initChatRecordTestDB(t)

	providerRow := models.Provider{Name: "p1", Type: providers.TypeOpenAI}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model-provider: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"usage":"boom"}`))
	}))
	defer srv.Close()

	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:               context.Background(),
		Start:             time.Now(),
		Style:             string(consts.StyleAnthropic),
		Before:            Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName:     "m1",
		ReqMeta:           models.ReqMeta{Header: http.Header{}},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		Selection:         selectionForEndpoint(string(consts.ProtocolOpenAI)),
		ChatModel:         chatModel,
		Client:            srv.Client(),
	}, make(chan models.ChatLog))

	var got models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p1").First(&got).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.Status != "error" {
		t.Fatalf("Status = %q, want error", got.Status)
	}
	if !strings.HasPrefix(got.Error, "transform response error: ") {
		t.Fatalf("Error = %q, want 前缀 transform response error:（包装单源被绕开）", got.Error[:min(64, len(got.Error))])
	}
}
