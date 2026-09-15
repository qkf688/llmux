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
	"github.com/qkf688/llmux/service/virtualmodel"
)

// 多 api-key 方案 §4 归因快照的落库契约：/v1 请求产生的每条 chat_logs 行携带
// 调用 key 的**写入时**快照（api_key_id + api_key_name），key 改名/删除不回写历史行。
// 四类建行点（尝试、虚拟跳过、虚拟选模失败、模型不存在）在此各钉一处；
// handler→ReqMeta 的翻译在 handler/v1 侧测（newReqMeta），出口 DTO 在 handler/logs 侧测。

func createAttributionFixtureProvider(t *testing.T, name string) (models.Provider, models.ModelWithProvider) {
	t.Helper()
	providerRow := models.Provider{Name: name, Type: providers.TypeOpenAI}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	mwp := models.ModelWithProvider{ProviderID: providerRow.ID, ProviderModel: "pm1"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create model-provider: %v", err)
	}
	return providerRow, mwp
}

func TestExecuteSingleProviderAttempt_CallerKeySnapshot_WrittenOnCreate(t *testing.T) {
	initChatRecordTestDB(t)
	providerRow, mwp := createAttributionFixtureProvider(t, "p-attr")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream boom"}`))
	}))
	defer srv.Close()
	chatModel, err := providers.New(providers.TypeOpenAI, fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, srv.URL), "")
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	callerID := uint(7)
	executeSingleProviderAttempt(singleProviderAttemptInput{
		Ctx:   context.Background(),
		Style: string(consts.StyleOpenAI),
		Before: Before{
			Model: "m1", Stream: false,
			raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`),
		},
		RealModelName: "m1",
		ReqMeta: models.ReqMeta{
			Header:     http.Header{},
			ApiKeyID:   &callerID,
			ApiKeyName: "codex-key",
		},
		Provider:          providerRow,
		ModelWithProvider: mwp,
		Selection:         selectionForEndpoint(string(consts.ProtocolOpenAI)),
		ChatModel:         chatModel,
		Client:            srv.Client(),
	}, make(chan models.ChatLog))

	var got models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p-attr").First(&got).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.ApiKeyID == nil || *got.ApiKeyID != callerID {
		t.Fatalf("ApiKeyID = %v, want %d（归因快照须随建行落库）", got.ApiKeyID, callerID)
	}
	if got.ApiKeyName != "codex-key" {
		t.Fatalf("ApiKeyName = %q, want %q", got.ApiKeyName, "codex-key")
	}
}

// 无 CallerKey 的建行（内部路径/测试构造）是合法空态：id 必须 NULL 而非 0。
func TestExecuteSingleProviderAttempt_NoCallerKey_AttributionStaysNull(t *testing.T) {
	initChatRecordTestDB(t)
	providerRow, mwp := createAttributionFixtureProvider(t, "p-none")

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
	if err := models.DB.Where("provider_name = ?", "p-none").First(&got).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.ApiKeyID != nil {
		t.Fatalf("ApiKeyID = %d, want NULL（无归因不得写 0 指向不存在的 key）", *got.ApiKeyID)
	}
	if got.ApiKeyName != "" {
		t.Fatalf("ApiKeyName = %q, want 空串", got.ApiKeyName)
	}
}

// 快照语义：日志行与 api_keys 表零耦合——改名、硬删都不回写历史行。
// 若将来有人给两列加 FK/JOIN/级联，本用例即红。
func TestChatLog_AttributionSnapshot_ImmuneToKeyRenameAndDelete(t *testing.T) {
	initChatRecordTestDB(t)

	key := models.ApiKey{OwnerID: 1, Name: "original", Key: "cipher-placeholder", KeyHash: "hash-1", Enabled: true}
	if err := models.DB.Create(&key).Error; err != nil {
		t.Fatalf("create api key: %v", err)
	}
	keyID := key.ID
	callerID := uint(keyID)
	logID, err := SaveChatLog(context.Background(), models.ChatLog{
		Name:       "m1",
		Status:     "success",
		ApiKeyID:   &callerID,
		ApiKeyName: "original",
	})
	if err != nil {
		t.Fatalf("save log: %v", err)
	}

	if err := models.DB.Model(&key).Update("name", "renamed").Error; err != nil {
		t.Fatalf("rename key: %v", err)
	}
	if err := models.DB.Unscoped().Delete(&models.ApiKey{}, keyID).Error; err != nil {
		t.Fatalf("delete key: %v", err)
	}

	var got models.ChatLog
	if err := models.DB.First(&got, logID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if got.ApiKeyName != "original" {
		t.Fatalf("ApiKeyName = %q, want %q（名称是写入时快照，改名不回写）", got.ApiKeyName, "original")
	}
	if got.ApiKeyID == nil || *got.ApiKeyID != keyID {
		t.Fatalf("ApiKeyID = %v, want %d（删 key 不回写日志行）", got.ApiKeyID, keyID)
	}
}

func TestSaveVirtualSkipLog_CarriesCallerKeySnapshot(t *testing.T) {
	initChatRecordTestDB(t)
	callerID := uint(3)
	saveVirtualSkipLog(context.Background(), models.ReqMeta{ApiKeyID: &callerID, ApiKeyName: "virt-key"},
		"vm1", "real-x", "openai", "virtual model skip: test")

	var got models.ChatLog
	if err := models.DB.Where("name = ? AND real_model_name = ?", "vm1", "real-x").First(&got).Error; err != nil {
		t.Fatalf("reload skip log: %v", err)
	}
	if got.ApiKeyID == nil || *got.ApiKeyID != callerID {
		t.Fatalf("ApiKeyID = %v, want %d（虚拟跳过行也须带归因）", got.ApiKeyID, callerID)
	}
	if got.ApiKeyName != "virt-key" {
		t.Fatalf("ApiKeyName = %q, want %q", got.ApiKeyName, "virt-key")
	}
}

// 「模型不存在」是错误行归因价值最高的路径（拿 key 探测不存在模型必须可追）。
func TestProvidersWithMetaBymodelsName_ModelNotFound_LogCarriesCallerKey(t *testing.T) {
	initChatRecordTestDB(t)
	callerID := uint(11)
	_, err := ProvidersWithMetaBymodelsName(context.Background(), "openai", Before{Model: "ghost-model"},
		models.ReqMeta{ApiKeyID: &callerID, ApiKeyName: "probe-key"})
	if err == nil {
		t.Fatalf("expected error for missing model")
	}

	var got models.ChatLog
	if err := models.DB.Where("name = ?", "ghost-model").First(&got).Error; err != nil {
		t.Fatalf("reload model-not-found log: %v", err)
	}
	if got.ApiKeyID == nil || *got.ApiKeyID != callerID {
		t.Fatalf("ApiKeyID = %v, want %d", got.ApiKeyID, callerID)
	}
	if got.ApiKeyName != "probe-key" {
		t.Fatalf("ApiKeyName = %q, want %q", got.ApiKeyName, "probe-key")
	}
}

// 虚拟选模失败建行点同样须带归因（审查 W1：plan 承诺两 error 路径各钉一例）。
// 构造：mapping 指向不存在的真实模型 → loadRealModels 报 errNoRealModels。
func TestProvidersWithMetaBymodelsName_VirtualSelectionFailed_LogCarriesCallerKey(t *testing.T) {
	virtualmodel.ResetSingletonForTest()
	initChatRecordTestDB(t)

	enabled := true
	virtual := models.VirtualModel{Name: "vm-broken", Strategy: "priority", Enabled: &enabled}
	if err := models.DB.Create(&virtual).Error; err != nil {
		t.Fatalf("create virtual model: %v", err)
	}
	if err := models.DB.Create(&models.VirtualModelMapping{
		VirtualModelID: virtual.ID,
		RealModelID:    99999,
		Enabled:        &enabled,
	}).Error; err != nil {
		t.Fatalf("create dangling mapping: %v", err)
	}

	callerID := uint(13)
	_, err := ProvidersWithMetaBymodelsName(context.Background(), "openai", Before{Model: virtual.Name},
		models.ReqMeta{ApiKeyID: &callerID, ApiKeyName: "vm-probe-key"})
	if err == nil {
		t.Fatalf("expected virtual selection error for dangling mapping")
	}

	var got models.ChatLog
	if err := models.DB.Where("name = ?", virtual.Name).First(&got).Error; err != nil {
		t.Fatalf("reload virtual-selection-failed log: %v", err)
	}
	if got.ApiKeyID == nil || *got.ApiKeyID != callerID {
		t.Fatalf("ApiKeyID = %v, want %d", got.ApiKeyID, callerID)
	}
	if got.ApiKeyName != "vm-probe-key" {
		t.Fatalf("ApiKeyName = %q, want %q", got.ApiKeyName, "vm-probe-key")
	}
}
