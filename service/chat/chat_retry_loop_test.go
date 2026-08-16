package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
)

func TestRunProviderRetryLoop(t *testing.T) {
	initChatRecordTestDB(t)

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// baseInput 构造「两个候选、供应商 type 均未注册」的候选池：providers.New 必然失败。
	// 各用例只替换要验证的那一项，其余保持基准，避免每个场景复制一整份入参。
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
			// 单个供应商实例化失败只应淘汰该候选，池中其余候选继续尝试，
			// 直到候选耗尽才报穷尽——不得升级为致命错误终止整个请求。
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
