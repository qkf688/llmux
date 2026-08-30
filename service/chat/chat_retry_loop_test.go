package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
)

// testChatCredHexKey 与 service/channel 测试同值的 32 字节 hex 加密密钥
// （凭据选择需要 decrypt 明文 key）。
const testChatCredHexKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestRunProviderRetryLoop(t *testing.T) {
	initChatRecordTestDB(t)

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// baseInput 构造「两个候选、DB 均无端点/分组/凭据」的候选池：选路（channel Select）
	// 必然失败。各用例只替换要验证的那一项，其余保持基准，避免每个场景复制一整份入参。
	baseInput := func() retryLoopInput {
		return retryLoopInput{
			Ctx:    context.Background(),
			Start:  time.Now(),
			Style:  "openai",
			Before: Before{Model: "m"},
			Pool: CandidatePool{
				ProviderMap: map[uint]models.Provider{
					11: {Type: "no-such-provider-type", Name: "bad-1"},
					12: {Type: "no-such-provider-type", Name: "bad-2"},
				},
				// 候选身份由 map key 标识（循环经 key 查找并淘汰），无需填充嵌入的 gorm.Model.ID。
				ModelWithProviderMap: map[uint]models.ModelWithProvider{
					1: {ProviderID: 11, ProviderModel: "m-1"},
					2: {ProviderID: 12, ProviderModel: "m-2"},
				},
				WeightItems:   map[uint]int{1: 10, 2: 10},
				PriorityItems: map[uint]int{1: 1, 2: 1},
			},
			RealModelName: "m",
			MaxRetry:      5,
			ClientTimeout: time.Second,
			// 不触发的超时通道：本组用例只关心候选淘汰与取消路径。
			Deadline:    make(chan time.Time),
			DeadlineErr: errors.New("unused"),
		}
	}

	tests := []struct {
		name       string
		mutate     func(*retryLoopInput)
		wantStatus retryLoopStatus
		wantErr    error
	}{
		{
			// 候选无选路数据（端点/分组/凭据全缺）时只淘汰该候选，池中其余候选
			// 继续尝试，直到候选耗尽才报穷尽——不得升级为致命错误终止整个请求。
			name:       "unusable providers skipped until pool exhausted",
			wantStatus: retryLoopExhausted,
		},
		{
			// ctx 取消须原样上抛 ctx.Err()，不得降级为「候选池穷尽」（后者会让虚拟路径继续换模型）。
			name:       "canceled context aborts",
			mutate:     func(in *retryLoopInput) { in.Ctx = canceledCtx },
			wantStatus: retryLoopAborted,
			wantErr:    context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput()
			if tt.mutate != nil {
				tt.mutate(&in)
			}

			outcome := runProviderRetryLoop(in)

			if outcome.Status != tt.wantStatus {
				t.Fatalf("status = %d, want %d; err = %v", outcome.Status, tt.wantStatus, outcome.Err)
			}
			if tt.wantErr != nil && !errors.Is(outcome.Err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", outcome.Err, tt.wantErr)
			}
		})
	}
}

// TestRunProviderRetryLoop_SkipsUnselectableProviderThenSucceeds：
// 池中候选「无端点/分组/凭据数据（选路必失败）」时只淘汰该候选继续尝试，
// 不得升级为 FatalErr；剩余正常候选仍应完成请求——选路失败即组织级淘汰
// （#13 才做凭据级组内转移，此处不区分 sentinel 层级）。
func TestRunProviderRetryLoop_SkipsUnselectableProviderThenSucceeds(t *testing.T) {
	initChatRecordTestDB(t)
	cipher := setupChatChannelCipher(t)

	var upstreamHit atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit.Add(1)
		// bad 候选若发出请求（旧行为：绕过选路直连），必带 model=pm-bad → 502 淘汰；
		// good 命中的请求带 pm-good → 200。计数 1 次 = 只有 good 被放行。
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "pm-bad") {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"upstream boom"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","model":"pm-good","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	// good 供应商：1 端点（openai、URL 空继承 base_url）+ 1 分组 + 1 内联凭据——
	// 老 Provider 迁移后的最小等价形态，选路必须成功。
	good := models.Provider{
		Name:   "good",
		Type:   "openai",
		Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL),
	}
	if err := models.DB.Create(&good).Error; err != nil {
		t.Fatalf("create good provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: good.ID, Protocol: string(consts.ProtocolOpenAI), Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	grp := models.KeyGroup{ProviderID: good.ID, Name: "g1", Weight: 1}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group: %v", err)
	}
	plainKey := "sk-good-key"
	encKey, err := cipher.Encrypt(plainKey)
	if err != nil {
		t.Fatalf("encrypt credential: %v", err)
	}
	cred := models.Credential{GroupID: &grp.ID, Key: encKey, KeyHash: cipher.Hash(plainKey)}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}
	goodMWP := models.ModelWithProvider{ProviderID: 12, ProviderModel: "pm-good"}
	if err := models.DB.Create(&goodMWP).Error; err != nil {
		t.Fatalf("create good assoc: %v", err)
	}
	// bad 的 MWP 也落库：Adjustment 按 MWP.ID 自增/复位连续失败（re-read 需要行存在），
	// 不落库只会留下 record not found 噪音——ProviderID 11 无 DB 行（候选身份由 map 标识）。
	badMWP := models.ModelWithProvider{ProviderID: 11, ProviderModel: "pm-bad"}
	if err := models.DB.Create(&badMWP).Error; err != nil {
		t.Fatalf("create bad assoc: %v", err)
	}

	// bad 候选：仅存在于候选池 map，DB 无其端点/分组/凭据 → 选路必失败被淘汰。
	outcome := runProviderRetryLoop(retryLoopInput{
		Ctx:           context.Background(),
		Start:         time.Now(),
		Style:         string(consts.StyleOpenAI),
		Before:        Before{Model: "m", Stream: false, raw: []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName: "m",
		Pool: CandidatePool{
			ProviderMap: map[uint]models.Provider{
				// bad 候选给 base_url 指向 srv：若绕过选路（旧行为）必打中 502 分支被淘汰。
				11: {Name: "bad-no-data", Type: "openai", Config: fmt.Sprintf(`{"base_url":%q}`, srv.URL)},
				12: good,
			},
			ModelWithProviderMap: map[uint]models.ModelWithProvider{
				badMWP.ID:  badMWP,
				goodMWP.ID: goodMWP,
			},
			WeightItems:   map[uint]int{badMWP.ID: 10, goodMWP.ID: 10},
			PriorityItems: map[uint]int{badMWP.ID: 1, goodMWP.ID: 1},
		},
		MaxRetry:      5,
		ClientTimeout: 2 * time.Second,
		Deadline:      make(chan time.Time),
		DeadlineErr:   errors.New("unused"),
	})

	if outcome.Status != retryLoopSucceeded {
		t.Fatalf("status = %d, want %d; err = %v", outcome.Status, retryLoopSucceeded, outcome.Err)
	}
	if got := upstreamHit.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1（bad 候选必须被选路提前拦截，只有 good 发出请求）", got)
	}
}
