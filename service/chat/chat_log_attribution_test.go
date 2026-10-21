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
	"github.com/qkf688/llmux/service/virtualmodel"
)

// 多 api-key 方案 §4 归因快照的落库契约：/v1 请求产生的每条 chat_logs 行携带
// 调用 key 的**写入时**快照（api_key_id + api_key_name），key 改名/删除不回写历史行。
// 五类建行点（尝试、虚拟跳过、虚拟选模失败、模型不存在、真实模型无启用供应商）
// 在此各钉一处；handler→ReqMeta 的翻译在 handler/v1 侧测（newReqMeta），
// 出口 DTO 在 handler/logs 侧测。

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

// 「真实模型存在、但一个启用供应商都没有」此前是审计盲区：请求失败却零行。
// 本用例钉两件事：① 该失败必须建行且带调用 key 快照；② 失败原因要点明
// 「严格能力匹配开启时关联可能被能力条件筛掉」——否则「关联页明明启用了」
// 只能被当成系统 bug 排查。
func TestProvidersWithMetaBymodelsName_NoEnabledProvider_WritesAttributedLog(t *testing.T) {
	initChatRecordTestDB(t)

	amin := models.Model{Name: "m-no-provider"}
	if err := models.DB.Create(&amin).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	callerID := uint(17)
	_, err := ProvidersWithMetaBymodelsName(context.Background(), "openai", Before{Model: amin.Name},
		models.ReqMeta{ApiKeyID: &callerID, ApiKeyName: "cfg-probe-key"})
	if err == nil {
		t.Fatalf("expected error for model without enabled provider")
	}

	var got models.ChatLog
	if err := models.DB.Where("name = ?", amin.Name).First(&got).Error; err != nil {
		t.Fatalf("reload no-provider log: %v", err)
	}
	// 未发出上游请求即失败 → unsent（不是 error），与「上游失败」区分开。
	if got.Status != models.ChatLogStatusUnsent {
		t.Fatalf("Status = %q, want %q", got.Status, models.ChatLogStatusUnsent)
	}
	if got.RealModelName != amin.Name {
		t.Fatalf("RealModelName = %q, want %q", got.RealModelName, amin.Name)
	}
	// ProviderName/ProviderModel 恒空 = 「未选到供应商」的可判别特征（列表页靠此辨认）。
	if got.ProviderName != "" || got.ProviderModel != "" {
		t.Fatalf("ProviderName/ProviderModel = %q/%q, want 空", got.ProviderName, got.ProviderModel)
	}
	if !strings.Contains(got.Error, "strict capability match is on") {
		t.Fatalf("Error = %q, want 含严格能力匹配提示（默认开启）", got.Error)
	}
	if got.ApiKeyID == nil || *got.ApiKeyID != callerID {
		t.Fatalf("ApiKeyID = %v, want %d（无供应商失败也必须可归因）", got.ApiKeyID, callerID)
	}
	if got.ApiKeyName != "cfg-probe-key" {
		t.Fatalf("ApiKeyName = %q, want %q", got.ApiKeyName, "cfg-probe-key")
	}
}

// 严格能力匹配关闭时不该出现那条提示：否则管理员会去查一个根本没参与的开关。
func TestProvidersWithMetaBymodelsName_NoEnabledProvider_OmitsCapabilityHintWhenOff(t *testing.T) {
	initChatRecordTestDB(t)

	// setting_schema 已在建库时种入默认值（true），此处只改值——再 Insert 会撞唯一索引。
	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyStrictCapabilityMatch).
		Update("value", "false").Error; err != nil {
		t.Fatalf("disable strict capability match: %v", err)
	}

	amin := models.Model{Name: "m-no-provider-off"}
	if err := models.DB.Create(&amin).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	if _, err := ProvidersWithMetaBymodelsName(context.Background(), "openai", Before{Model: amin.Name},
		models.ReqMeta{}); err == nil {
		t.Fatalf("expected error for model without enabled provider")
	}

	var got models.ChatLog
	if err := models.DB.Where("name = ?", amin.Name).First(&got).Error; err != nil {
		t.Fatalf("reload no-provider log: %v", err)
	}
	if strings.Contains(got.Error, "strict capability match") {
		t.Fatalf("Error = %q, 关档时不得出现能力匹配提示", got.Error)
	}
}

// 本出口是「日志与统计双双零记录」变「两处都有」的修复，故除了 chat_logs 行，
// 还必须钉住仪表盘统计同步累加——只查日志行会让「统计漏记」这一半静默回归。
func TestProvidersWithMetaBymodelsName_NoEnabledProvider_CountsRequestStats(t *testing.T) {
	initChatRecordTestDB(t)

	amin := models.Model{Name: "m-no-provider-stats"}
	if err := models.DB.Create(&amin).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	if _, err := ProvidersWithMetaBymodelsName(context.Background(), "openai", Before{Model: amin.Name},
		models.ReqMeta{}); err == nil {
		t.Fatalf("expected error for model without enabled provider")
	}

	var daily models.StatsDaily
	if err := models.DB.Where("date = ?", time.Now().Format("2006-01-02")).First(&daily).Error; err != nil {
		t.Fatalf("reload stats daily: %v", err)
	}
	if daily.Reqs != 1 {
		t.Fatalf("StatsDaily.Reqs = %d, want 1（失败请求同样计入请求数）", daily.Reqs)
	}

	var realTotal models.StatsRealModelTotal
	if err := models.DB.Where("name = ?", amin.Name).First(&realTotal).Error; err != nil {
		t.Fatalf("reload real model stats: %v", err)
	}
	if realTotal.Calls != 1 {
		t.Fatalf("StatsRealModelTotal.Calls = %d, want 1（真实模型维度按 RealModelName 归集）", realTotal.Calls)
	}
}

// 写日志失败必须保持调用方的原错误（旧实现会把「写日志失败」原样上抛，把
// 「模型不存在」偷换成数据库错误）。做法：只删日志表——模型查询仍正常，
// 走到建行那一步才失败。
func TestProvidersWithMetaBymodelsName_LogWriteFailure_KeepsOriginalError(t *testing.T) {
	initChatRecordTestDB(t)

	amin := models.Model{Name: "m-log-write-fails"}
	if err := models.DB.Create(&amin).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := models.DB.Migrator().DropTable(&models.ChatLog{}); err != nil {
		t.Fatalf("drop chat_logs: %v", err)
	}

	_, gotErr := ProvidersWithMetaBymodelsName(context.Background(), "openai", Before{Model: amin.Name},
		models.ReqMeta{})
	if gotErr == nil {
		t.Fatalf("expected error")
	}
	// 文案随 404 收口改为 noUsableProviderReason 原文（no enabled provider...），
	// 断言跟着对齐：锁「保留配置态原因」而非具体措辞。
	if !strings.Contains(gotErr.Error(), "no enabled provider for model") {
		t.Fatalf("err = %q, want 保留「无启用供应商」原错误而非写库错误", gotErr)
	}
	if strings.Contains(gotErr.Error(), "no such table") {
		t.Fatalf("err = %q, 不得把写库失败偷换成调用方看到的失败原因", gotErr)
	}
}

// 严格能力匹配开启（默认）时，「关联存在但被能力需求筛掉」才是这条日志要解释的
// 真实场景：只造零关联的 fixture 测不出筛选行为，故这里造一条 tool_call 能力不满足
// 的已启用关联，请求带工具调用 —— 结果必须是「无可用供应商 + 带能力匹配提示」。
func TestProvidersWithMetaBymodelsName_CapabilityFilteredProvider_HintedInLog(t *testing.T) {
	initChatRecordTestDB(t)

	enabled := true
	providerRow := models.Provider{Name: "p-no-toolcall", Type: providers.TypeOpenAI}
	if err := models.DB.Create(&providerRow).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	amin := models.Model{Name: "m-cap-filtered"}
	if err := models.DB.Create(&amin).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	noToolCall := false
	if err := models.DB.Create(&models.ModelWithProvider{
		ModelID:       amin.ID,
		ProviderID:    providerRow.ID,
		ProviderModel: "pm-cap",
		Status:        &enabled,
		ToolCall:      &noToolCall,
	}).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}

	raw := []byte(`{"model":"` + amin.Name + `","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"f","parameters":{}}}]}`)
	before, err := BeforerOpenAI(raw)
	if err != nil {
		t.Fatalf("parse before: %v", err)
	}
	if !before.toolCall {
		t.Fatalf("fixture 未标记 toolCall，用例失去意义")
	}

	if _, err := ProvidersWithMetaBymodelsName(context.Background(), "openai", *before,
		models.ReqMeta{}); err == nil {
		t.Fatalf("expected error: 关联存在但工具调用能力不满足，应被筛掉")
	}

	var got models.ChatLog
	if err := models.DB.Where("name = ?", amin.Name).First(&got).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if !strings.Contains(got.Error, "strict capability match is on") {
		t.Fatalf("Error = %q, want 含能力匹配提示——这正是该提示要解释的场景", got.Error)
	}
}
