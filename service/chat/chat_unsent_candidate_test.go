package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/service/channel"
)

// 本文件锁「选路阶段候选被淘汰 → 落一条 unsent 留痕行」这条新行为（2026-10-21 拍板）。
//
// 改动前：淘汰只打 slog.Warn("no channel selection")，chat_logs 里零行——管理端
// 「请求日志」页查不到「这次请求试了哪家、败在哪一层」。chat_logs 的尝试行只在
// executeSingleProviderAttempt 内建（已选出凭据之后），外层淘汰点从未走到那里。
//
// 三个必须同时成立的口径（缺一条这次改动就跑偏）：
//  1. 状态是 unsent 而**不是** error —— 供应商没做错任何事，混进 error 会让
//     handler/associations 的「最近 10 次请求成败」把它画成这家在失败；
//  2. 落名到了 provider / model：不填就查不出「是哪家被淘汰」，改动没意义；
//  3. 组内凭据耗尽那条路径**不补行**（它此前必已产出尝试行），否则同一次上游
//     尝试被计成两行，请求数与模型调用数双双翻倍。

// seedUnsentCandidate 造一个「有供应商但没有任何端点/分组/凭据」的候选：
// Assemble 成功、Select 必失败（无可用端点），是最稳定的外层淘汰形态。
func seedUnsentCandidate(t *testing.T, name string) (models.Provider, models.ModelWithProvider) {
	t.Helper()
	p := models.Provider{Name: name, Type: "openai"}
	if err := models.DB.Create(&p).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	m := models.Model{Name: "m-unsent"}
	if err := models.DB.Create(&m).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	mwp := models.ModelWithProvider{ModelID: m.ID, ProviderID: p.ID, ProviderModel: "pm-unsent"}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	return p, mwp
}

// TestRunProviderRetryLoop_EliminatedCandidate_WritesUnsentRow 锁主行为：
// 候选因选路失败被淘汰时，必须落一条 unsent 行，且带得上供应商与上游模型名。
func TestRunProviderRetryLoop_EliminatedCandidate_WritesUnsentRow(t *testing.T) {
	initChatRecordTestDB(t)

	p, mwp := seedUnsentCandidate(t, "p-unsent")

	// Start 刻意回拨 50ms：ProxyTime 断言因此有确定性下限，不依赖时钟分辨率
	// （写 time.Now() 时，高负载下 time.Since 可能取到 0，用例会变成偶发红）。
	outcome := runProviderRetryLoop(retryLoopInput{
		Ctx:           context.Background(),
		Start:         time.Now().Add(-50 * time.Millisecond),
		Style:         "openai",
		Before:        Before{Model: "m-unsent"},
		RealModelName: "m-unsent",
		ReqMeta:       models.ReqMeta{RequestID: "gw:unsent-test"},
		Pool: CandidatePool{
			ProviderMap:          map[uint]models.Provider{p.ID: p},
			ModelWithProviderMap: map[uint]models.ModelWithProvider{mwp.ID: mwp},
			WeightItems:          map[uint]int{mwp.ID: 10},
			PriorityItems:        map[uint]int{mwp.ID: 1},
		},
		MaxRetry:       1,
		UpstreamWindow: upstreamWindow{Window: time.Second},
	})

	// 池里只有一个候选且选路必失败 → 穷尽（不是致命错误）。
	if outcome.Status != retryLoopExhausted {
		t.Fatalf("status = %d, want %d（选路失败只淘汰候选，不升致命错误）", outcome.Status, retryLoopExhausted)
	}

	var rows []models.ChatLog
	if err := models.DB.Where("provider_name = ?", p.Name).Find(&rows).Error; err != nil {
		t.Fatalf("query unsent rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("unsent 行数 = %d, want 1（淘汰必须留痕，且一家一行）", len(rows))
	}
	got := rows[0]
	if got.Status != models.ChatLogStatusUnsent {
		t.Fatalf("Status = %q, want %q（未发出上游请求 ≠ 上游失败）", got.Status, models.ChatLogStatusUnsent)
	}
	if got.Status == models.ChatLogStatusError {
		t.Fatalf("Status 不得是 error：那会把「这家供应商在失败」的假结论喂给成败统计")
	}
	if got.ProviderModel != "pm-unsent" {
		t.Fatalf("ProviderModel = %q, want pm-unsent（查不出是哪家就失去本次改动意义）", got.ProviderModel)
	}
	if got.Error == "" {
		t.Fatalf("Error 为空：淘汰原因必须落库，否则面板只有一个哑状态")
	}
	if got.RequestID != "gw:unsent-test" {
		t.Fatalf("RequestID = %q, want gw:unsent-test（与同请求其它行可串起来）", got.RequestID)
	}
	// 选路失败真实消耗了请求时间（含探活的一次上游往返），ProxyTime 必须如实反映：
	// 0 会让面板显示得像「瞬间淘汰」，看不出这次请求白等了多久。Start 已回拨 50ms，
	// 故下限是确定性的，不随机器负载/时钟分辨率抖动。
	if got.ProxyTime < 50*time.Millisecond {
		t.Fatalf("ProxyTime = %v, want >= 50ms（淘汰耗时必须如实回填）", got.ProxyTime)
	}
}

// TestRunProviderRetryLoop_EliminatedCandidate_NotCountedAsProviderFailure 锁归因边界：
// unsent 行**不得**被「最近 10 次请求成败」采信。该读点的判据是
// status IN (success, error) 的白名单（handler/associations），故 unsent 天然被排除
// ——本用例把这条依赖钉住：若将来有人把 unsent 并进 error，这里必红。
func TestRunProviderRetryLoop_EliminatedCandidate_NotCountedAsProviderFailure(t *testing.T) {
	initChatRecordTestDB(t)

	p, mwp := seedUnsentCandidate(t, "p-unsent-notfail")

	runProviderRetryLoop(retryLoopInput{
		Ctx:           context.Background(),
		Start:         time.Now(),
		Style:         "openai",
		Before:        Before{Model: "m-unsent"},
		RealModelName: "m-unsent",
		Pool: CandidatePool{
			ProviderMap:          map[uint]models.Provider{p.ID: p},
			ModelWithProviderMap: map[uint]models.ModelWithProvider{mwp.ID: mwp},
			WeightItems:          map[uint]int{mwp.ID: 10},
			PriorityItems:        map[uint]int{mwp.ID: 1},
		},
		MaxRetry:       1,
		UpstreamWindow: upstreamWindow{Window: time.Second},
	})

	// 白名单口径的等价查询：成败读点只看 success/error，unsent 不在此列。
	var outcomeRows int64
	if err := models.DB.Model(&models.ChatLog{}).
		Where("provider_name = ?", p.Name).
		Where("status IN ?", []string{models.ChatLogStatusSuccess, models.ChatLogStatusError}).
		Count(&outcomeRows).Error; err != nil {
		t.Fatalf("count outcome rows: %v", err)
	}
	if outcomeRows != 0 {
		t.Fatalf("成败读点命中 %d 行, want 0（unsent 不得被当成该供应商的失败）", outcomeRows)
	}

	// 反向确认那行确实存在，避免用例因「压根没建行」而假绿。
	var all int64
	if err := models.DB.Model(&models.ChatLog{}).Where("provider_name = ?", p.Name).Count(&all).Error; err != nil {
		t.Fatalf("count all rows: %v", err)
	}
	if all != 1 {
		t.Fatalf("总行数 = %d, want 1（本用例前提是那行确实落了库）", all)
	}
}

// TestRunProviderRetryLoop_CredentialLayerElimination_AttributionAndNoStats 锁
// 「凭据层淘汰」这一最常见淘汰形态的两件事（2026-10-22 code-review 补）：
//
//  1. **归因字段齐全**：凭据层失败时 Select 仍带回 Endpoint/Group（见
//     channel.Select 的凭据层错误返回），故留痕行必须能说出「哪家的哪个分组的
//     哪个端点（协议 + URL）」。此前 EndpointURL 取自 sel.UpstreamURL，而选路失败
//     时它恒为空（只有全成功路径才填）——归因只答到一半，正是本次改动要回答的
//     「败在哪一家的哪一层」被漏掉的那一层。
//  2. **不计入仪表盘统计**：本行是**候选级**事件，一次请求可淘汰 N 个候选、落
//     N 行；若各计一次 reqs/calls，仪表盘的「总请求数」与「真实模型调用排行」
//     就按候选数放大（选路淘汰在本次改动前是零行零计数）。
func TestRunProviderRetryLoop_CredentialLayerElimination_AttributionAndNoStats(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)
	installChatTestSettings(t, credCooldownSettingsReader{
		Cooldown429Sec: 60, CooldownServerSec: 60, AuthFailThreshold: 1, ProbeIntervalSec: 60,
	})

	// 探活路径（GET /models）返回 500：凭据已判停，探活救不回来 → 候选淘汰。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"probe boom"}`))
	}))
	defer srv.Close()

	// 组内唯一凭据已判停（temp_unsched）→ Select 在**凭据层**失败（带回 Endpoint/Group）。
	provider, _, pool, _, _ := seedProbeChannel(t, cipher, "cred-layer-elim", srv.URL, 0, "", 10, 1,
		func(t *testing.T, poolID uint) (uint, uint) {
			t.Helper()
			cred := models.Credential{
				PoolID:         poolID,
				Key:            mustEncrypt(t, cipher, "sk-cold"),
				KeyHash:        cipher.Hash("sk-cold"),
				Status:         models.CredentialStatusTempUnsched,
				CooldownReason: "auth_fail",
				FailCount:      3,
			}
			if err := models.DB.Create(&cred).Error; err != nil {
				t.Fatalf("create credential: %v", err)
			}
			return cred.ID, 0
		})

	outcome := runProbeRetryLoop(pool)
	if outcome.Status != retryLoopExhausted {
		t.Fatalf("status = %d, want %d（选路失败只淘汰候选，不升致命错误）", outcome.Status, retryLoopExhausted)
	}

	var rows []models.ChatLog
	if err := models.DB.Where("provider_name = ?", provider.Name).Find(&rows).Error; err != nil {
		t.Fatalf("query unsent rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("unsent 行数 = %d, want 1（淘汰必须留痕）", len(rows))
	}
	got := rows[0]
	if got.Status != models.ChatLogStatusUnsent {
		t.Fatalf("Status = %q, want %q", got.Status, models.ChatLogStatusUnsent)
	}
	if got.KeyGroupName != "g1" {
		t.Fatalf("KeyGroupName = %q, want g1（凭据层失败时 Select 带回分组，必须留名）", got.KeyGroupName)
	}
	if got.EndpointProtocol != string(consts.ProtocolOpenAI) {
		t.Fatalf("EndpointProtocol = %q, want %q", got.EndpointProtocol, string(consts.ProtocolOpenAI))
	}
	// 端点 URL 为空时按继承链回落到 provider 的 base_url（与 BuildConfig 同一口径）。
	if got.EndpointURL != srv.URL {
		t.Fatalf("EndpointURL = %q, want %q（选路失败时 Select 不带 URL 回来，必须独立解析）", got.EndpointURL, srv.URL)
	}

	// 统计不放大：淘汰行是候选级事件，不得计入请求数与真实模型调用数。
	var totals []models.StatsTotal
	if err := models.DB.Find(&totals).Error; err != nil {
		t.Fatalf("query stats total: %v", err)
	}
	for _, s := range totals {
		if s.Reqs != 0 {
			t.Fatalf("stats reqs = %d, want 0（候选淘汰行不是一次请求，不得计入请求数）", s.Reqs)
		}
	}
	var realModels []models.StatsRealModelTotal
	if err := models.DB.Find(&realModels).Error; err != nil {
		t.Fatalf("query real model stats: %v", err)
	}
	for _, rm := range realModels {
		if rm.Calls != 0 {
			t.Fatalf("真实模型 %q 的 calls = %d, want 0（淘汰行不是一次调用）", rm.Name, rm.Calls)
		}
	}
}

// TestRunProviderRetryLoop_InnerCredentialExhaustion_NoUnsentRow 锁「内层组内凭据
// 耗尽不补 unsent 行」这条防双计约束：那条路径此前**必已产出尝试行**（请求已发出
// 过），再补一行 unsent 会把同一次上游尝试计成两行——chat_logs 两行、请求数与
// 模型调用数双计。
//
// 本用例把两条路径的留痕差异钉死：
//   - 内层耗尽（已发出过请求）→ 只有 1 行尝试行，无 unsent 行；
//   - 外层选路淘汰（从未发出请求）→ 只有 1 行 unsent 留痕行（见上面两条用例）。
//
// 场景复用既有的「401 判停 → 锁组重选 → 组内全判停 → 惰性探活失败 → 组织级处置」。
func TestRunProviderRetryLoop_InnerCredentialExhaustion_NoUnsentRow(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	channel.ResetDefaultSelectorForTest()
	t.Cleanup(channel.ResetDefaultSelectorForTest)
	installChatTestSettings(t, credCooldownSettingsReader{
		Cooldown429Sec: 60, CooldownServerSec: 60, AuthFailThreshold: 1, ProbeIntervalSec: 60,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// 探活路径：500 → 恢复不了 → 组内耗尽 → 组织级处置。
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"probe boom"}`))
			return
		}
		// 聊天路径：401 → 判停 key1 → 锁组重选落 key2（全判停）→ 组内耗尽。
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	provider, _, pool, _, _ := seedProbeChannel(t, cipher, "inner-exhausted", srv.URL, 0, "", 10, 1,
		func(t *testing.T, poolID uint) (uint, uint) {
			t.Helper()
			return seedProbeRecoveryCredentials(t, cipher, poolID)
		})

	outcome := runProbeRetryLoop(pool)
	if outcome.Status != retryLoopExhausted {
		t.Fatalf("status = %d, want %d; err = %v", outcome.Status, retryLoopExhausted, outcome.Err)
	}

	var rows []models.ChatLog
	if err := models.DB.Where("provider_name = ?", provider.Name).Find(&rows).Error; err != nil {
		t.Fatalf("query chat log rows: %v", err)
	}
	// 只有那一次尝试行：组内耗尽路径此前已产出过它，不得再补一行 unsent。
	if len(rows) != 1 {
		var statuses []string
		for _, r := range rows {
			statuses = append(statuses, r.Status)
		}
		t.Fatalf("该 provider 的日志行数 = %d（状态 %v），want 1（内层耗尽不补 unsent 行，否则同一次上游尝试被计成两行）", len(rows), statuses)
	}
	if rows[0].Status == models.ChatLogStatusUnsent {
		t.Fatalf("内层耗尽路径落了一行 unsent：那条路径已发出过请求，状态应是尝试行而非 unsent")
	}
}
