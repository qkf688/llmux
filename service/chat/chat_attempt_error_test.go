package chat

import (
	"context"
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

// Stage 2-B：上游 500 错误体（病态/被劫持上游可回超大 body）不得整段进内存再拼进
// 日志——读取处即收口到 common.ReadCappedBody 的 1 MiB 诊断单源上限；同时锁住既有
// 判定不回归：500 仍是凭据级失败（CredentialFailure）。
func TestExecuteSingleProviderAttempt_NonOKErrorBodyCapped(t *testing.T) {
	initChatRecordTestDB(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("x", 4<<20))) // 4 MiB，超 1 MiB 上限
	}))
	t.Cleanup(srv.Close)

	providerRow := models.Provider{Name: "p-errcap", Type: providers.TypeOpenAI}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	enabled := true
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1", Status: &enabled}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"sk-errcap"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	result := executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:               context.Background(),
		Start:             time.Now(),
		Style:             string(consts.StyleOpenAI),
		Before:            Before{Model: "m1", raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName:     "m1",
		ReqMeta:           models.ReqMeta{Header: http.Header{}},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		Selection:         selectionForEndpoint(string(consts.ProtocolOpenAI)),
		ChatModel:         chatModel,
		Client:            srv.Client(),
	}, make(chan models.ChatLog, 8))

	if !result.CredentialFailure {
		t.Fatalf("500 应仍判凭据级失败, result = %+v", result)
	}

	var row models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p-errcap").First(&row).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if row.Status != "error" {
		t.Fatalf("Status = %q, want error", row.Status)
	}
	// Error = "status: %d, body: %s"：读侧 1 MiB 封顶后，Stage 2-D 再把落库文本截到
	// 采集上限（默认 256 KiB）——整串必须 ≤ 256KiB + 前缀/标记余量。
	if len(row.Error) > (256<<10)+128 {
		t.Fatalf("Error 长度 %d, want ≤ 256KiB+128（采集截断未生效）", len(row.Error))
	}
	if !strings.HasPrefix(row.Error, "status: 500") {
		t.Fatalf("Error = %q, want 前缀 status: 500", row.Error[:min(64, len(row.Error))])
	}
}
