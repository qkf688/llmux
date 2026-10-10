package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// timeoutSettingsReader 测试用 settings.Reader stub：只覆写请求参数的四个键（字段为 0
// 表示不覆写、回退 defaultValue），其余键一律回退 defaultValue。超时最小粒度秒。
//
// 参数化而非写死值（Stage 38）：窗口分流用例与「窗口到点后候选 2 仍有完整预算」用例
// 需要**可区分**的窗口值，写死值的桩在第二处会被复制成近似副本——桩只留这一份。
//
// 总时长上限不在 BalanceChat 内读取（Stage 33 起经 WithRequestTotalTimeout 挂进
// 请求 ctx 的 deadline），该键供派生 deadline 的用例使用。
type timeoutSettingsReader struct {
	streamWindowSec    int // 流式等头窗口
	nonStreamWindowSec int // 非流式整段预算窗口
	totalTimeoutSec    int // 总时长上限（供 WithRequestTotalTimeout 派生 deadline）
	maxRetry           int // 单候选池尝试次数
	// fallbackToSchemaDefault 模拟「读失败 / 键缺失」：所有键都回退 defaultValue
	//（生产语义 = models 读不到该键时用调用方传的 default）。
	fallbackToSchemaDefault bool
}

func (timeoutSettingsReader) Bool(_ context.Context, _ string, def bool) bool { return def }

func (r timeoutSettingsReader) Int(_ context.Context, key string, def, _ int) int {
	// 零值桩 = 忘了填字段：四个键都会回退生产默认值，用例静默失去判据
	//（TestWithRequestTotalTimeout_DerivesDeadline 在参数化后曾因此拿到生产默认 600 而变红）。
	// 把「漏填」变成响亮的失败，而不是让断言悄悄测到生产默认值。
	if r == (timeoutSettingsReader{}) {
		panic("timeoutSettingsReader 需显式给字段：零值会让所有键回退生产默认值，用例失去判据")
	}
	if r.fallbackToSchemaDefault {
		return def
	}
	switch key {
	case models.SettingKeyRequestHeaderTimeout:
		if r.streamWindowSec > 0 {
			return r.streamWindowSec
		}
	case models.SettingKeyNonStreamResponseTimeout:
		if r.nonStreamWindowSec > 0 {
			return r.nonStreamWindowSec
		}
	case models.SettingKeyRequestTotalTimeout:
		if r.totalTimeoutSec > 0 {
			return r.totalTimeoutSec
		}
	case models.SettingKeyRequestMaxRetry:
		if r.maxRetry > 0 {
			return r.maxRetry
		}
	}
	return def
}

func (timeoutSettingsReader) String(_ context.Context, _ string, def string) string { return def }

func (r timeoutSettingsReader) BoolE(ctx context.Context, key string, def bool) (bool, error) {
	return r.Bool(ctx, key, def), nil
}

func (r timeoutSettingsReader) IntE(ctx context.Context, key string, def, min int) (int, error) {
	return r.Int(ctx, key, def, min), nil
}

// TestBalanceChat_HeaderTimeout_CandidateHasFullWindow 锁定「单次尝试窗口与总时长上限
// 解耦」的行为（AC-8）：候选 1 窗口超时后，候选 2 仍有完整窗口可继续尝试。
//
// 构造：单次尝试窗口=1s、总时长上限=3s（Stage 33 起以请求 ctx deadline 表达，同 handler
// 挂法）、MaxRetry=2；候选 1 上游 1.5s 不回响应头（窗口超时），候选 2 立即成功。若总上限
// 与窗口同一值（旧双角色语义），候选 1 耗满 1s 后总上限同步耗尽，候选 2 必然 aborted、
// 断言变红——本用例能区分新旧语义。
func TestBalanceChat_HeaderTimeout_CandidateHasFullWindow(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)
	// 两键同为 1s：本用例入站为非流式，分流后取「整段预算」键（Stage 38）——
	// 两键都给 1s 才能继续表达「窗口到点后候选 2 仍有完整预算」。
	installChatTestSettings(t, timeoutSettingsReader{streamWindowSec: 1, nonStreamWindowSec: 1, totalTimeoutSec: 3, maxRetry: 2})

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1500 * time.Millisecond) // 不回响应头：超过 1s 的尝试窗口
	}))
	defer slow.Close()

	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":0,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer fast.Close()

	// 真实路径常规构造：model + provider + 关联建行后经 buildCandidatePool 装配，
	// 避免内存构造 mwp 时 adjustment 的失败递增回查数据库产生 record not found 噪音。
	model := models.Model{Name: "m1"}
	if err := models.DB.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	providerSlow := models.Provider{Name: "slow-provider", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, slow.URL)}
	providerFast := models.Provider{Name: "fast-provider", Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q,"api_key":"k"}`, fast.URL)}
	for _, p := range []*models.Provider{&providerSlow, &providerFast} {
		if err := models.DB.Create(p).Error; err != nil {
			t.Fatalf("create provider: %v", err)
		}
		// S3-2 起凭据走加密 credentials 表：补老 Provider 等价形态（端点/分组/内联凭据）。
		addLegacyChannelRows(t, cipher, *p)
	}
	mwpSlow := models.ModelWithProvider{ModelID: model.ID, ProviderID: providerSlow.ID, ProviderModel: "slow-upstream", Weight: 10, Priority: 10}
	mwpFast := models.ModelWithProvider{ModelID: model.ID, ProviderID: providerFast.ID, ProviderModel: "fast-upstream", Weight: 10, Priority: 5}
	for _, m := range []*models.ModelWithProvider{&mwpSlow, &mwpFast} {
		if err := models.DB.Create(m).Error; err != nil {
			t.Fatalf("create association: %v", err)
		}
	}

	pool, err := buildCandidatePool(context.Background(), []models.ModelWithProvider{mwpSlow, mwpFast})
	if err != nil {
		t.Fatalf("build candidate pool: %v", err)
	}

	// 真实路径 CandidatePool 经 buildCandidatePool 装配。总时长上限以请求 ctx deadline
	// 表达（3s，同 handler 的挂法），候选 1 等头耗掉 1s 后仍有完整预算给候选 2。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := BalanceChat(ctx, BalanceInput{
		Start:  time.Now(),
		Style:  "openai",
		Before: Before{Model: "m1", Stream: false, raw: []byte(`{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)},
		ProvidersWithMeta: ProvidersWithMeta{
			CandidatePool: pool,
			Model:         &models.Model{Name: "m1"},
		},
		ReqMeta: models.ReqMeta{Header: http.Header{}},
	})
	if err != nil {
		t.Fatalf("BalanceChat err = %v, want success（候选 1 等头超时后候选 2 应仍有完整窗口）", err)
	}
	if result.ProviderName != "fast-provider" {
		t.Fatalf("ProviderName = %q, want %q（应故障转移到候选 2）", result.ProviderName, "fast-provider")
	}
}

// TestRequestParamReaders_FallbackMatchesSchemaDefault 锁「读取器兜底值 == schema 默认值」：
// 两者是同一语义的两处手写（reader 的 default 参数 vs SettingSchemas 的 Default），漂移会让
// 「新建库种入的值」与「读失败时回退的值」不一致——Stage 38 的 getNonStreamResponseTimeout
// 就曾把 90 写成 30（生产靠 seed 落库掩盖，只有读失败路径才暴露）。
func TestRequestParamReaders_FallbackMatchesSchemaDefault(t *testing.T) {
	installChatTestSettings(t, timeoutSettingsReader{fallbackToSchemaDefault: true})

	schemas := models.SettingSchemas()
	ctx := context.Background()
	cases := []struct {
		key string
		got int
	}{
		{models.SettingKeyRequestHeaderTimeout, getRequestHeaderTimeout(ctx)},
		{models.SettingKeyNonStreamResponseTimeout, getNonStreamResponseTimeout(ctx)},
		{models.SettingKeyRequestTotalTimeout, getRequestTotalTimeout(ctx)},
		{models.SettingKeyStreamFirstByteTimeout, getStreamFirstByteTimeout(ctx)},
		{models.SettingKeyRequestMaxRetry, getRequestMaxRetry(ctx)},
	}
	for _, tc := range cases {
		schema, ok := schemas[tc.key]
		if !ok {
			t.Fatalf("schema 未注册 %s", tc.key)
		}
		want, ok := schema.Default.(int)
		if !ok {
			t.Fatalf("%s 的 schema 默认值不是 int：%#v", tc.key, schema.Default)
		}
		if tc.got != want {
			t.Errorf("%s：读取器兜底 %d ≠ schema 默认 %d（改默认值时两处必须同步）", tc.key, tc.got, want)
		}
	}
}
