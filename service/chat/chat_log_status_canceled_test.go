package chat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/internal/testutil"
	"github.com/qkf688/llmux/models"
)

// TestRecordLog_CanceledStatusMatrix 锁提交后失败出口（RecordLog 的 processer 错误分支）
// 的状态判定：只有调用方**主动撤走**才记 canceled，其余失败形态一律 error。
//
// 判据单源 failureLogStatus（errors.Is(err, context.Canceled)）：写上限掐断与坏管道同属
// 客户端侧、却不是「主动取消」，混进来会让该状态失去语义，故逐条列为反例。
//
// 本文件与 chat_settlement_test.go 的断连用例有交叠（同一 fixture、同一链路），此处仍
// 独立成例是有意的：那边的主张是「调用方撤走不结算」，状态只是顺带断言；这边是状态判定
// 的专属落点，判据坏掉时不必先读完结算用例才定位。后两条分别断不同失败出口——
// Do 失败与读体失败——共处一个判定函数但走不同调用点，缺一条则该出口退回硬编码时无人发现。
func TestRecordLog_CanceledStatusMatrix(t *testing.T) {
	cases := []struct {
		name       string
		procErr    error
		wantStatus string
		why        string
	}{
		{
			name:       "调用方撤走→canceled",
			procErr:    fmt.Errorf("write response body failed: %w", context.Canceled),
			wantStatus: models.ChatLogStatusCanceled,
			why:        "请求 ctx 被取消即客户端断开/点停止，行必须如实记 canceled，不能混进供应商故障",
		},
		{
			name:       "上游截断→error",
			procErr:    fmt.Errorf("write response body failed: %w", io.ErrUnexpectedEOF),
			wantStatus: models.ChatLogStatusError,
			why:        "上游腰斩是供应商故障，调用方还在等——判据只看 Canceled，不看同属客户端侧的族",
		},
		{
			name:       "写回被写上限掐断→error",
			procErr:    fmt.Errorf("write response body failed: %w", MarkResponseWriteError(context.Background(), newWriteCutoffErr())),
			wantStatus: models.ChatLogStatusError,
			why:        "客户端不收结果≠客户端主动取消（Stage 39 已定案），供应商已完整产出，不记 canceled",
		},
		{
			name:       "总超时到点→error",
			procErr:    fmt.Errorf("read upstream: %w", context.DeadlineExceeded),
			wantStatus: models.ChatLogStatusError,
			why:        "超时与取消语义不同：超时是上游不吭声，照旧归供应商（Stage 33/24 口径）",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initChatRecordTestDB(t)

			logRow := testutil.NewRow(t, models.DB, models.ChatLog{
				Name:          "m",
				ProviderModel: "pm",
				ProviderName:  "p",
				Status:        models.ChatLogStatusProcessing,
			})

			RecordLog(context.Background(), RecordLogInput{
				ReqStart:     time.Now(),
				Reader:       io.NopCloser(strings.NewReader("x")),
				Processer:    failingProcesser(tc.procErr),
				LogID:        logRow.ID,
				Before:       Before{Stream: false, raw: []byte(`{}`)},
				ProviderName: "p",
			})

			var got models.ChatLog
			if err := models.DB.First(&got, logRow.ID).Error; err != nil {
				t.Fatalf("reload log: %v", err)
			}
			if got.Status != tc.wantStatus {
				t.Fatalf("状态 = %q, want %q（%s）", got.Status, tc.wantStatus, tc.why)
			}
		})
	}
}

// TestExecuteSingleProviderAttempt_CanceledDuringDo 锁提交前失败出口里最常见的一条：
// 请求已发出、响应头还没到时客户端断开 → Do 返回带 context.Canceled 的错误，该行落
// canceled（而非 error）。走真实 attempt 链路，不直调失败出口。
func TestExecuteSingleProviderAttempt_CanceledDuringDo(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release // 拖住上游：本次请求永远拿不到响应
	}))

	input := makeInput("canceled-during-do")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input.Ctx = ctx

	runAttemptUntilCallerAbandons(t, input, cancel, started, release)

	var row models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p1").First(&row).Error; err != nil {
		t.Fatalf("reload chat log: %v", err)
	}
	if row.Status != models.ChatLogStatusCanceled {
		t.Fatalf("状态 = %q, want canceled（调用方在等响应头时撤走，行须如实记已取消）", row.Status)
	}
}

// TestExecuteSingleProviderAttempt_CanceledDuringBodyRead 覆盖同族第二个出口：响应头已到、
// body 滴灌中断开 → 读体失败的 cause 带 context.Canceled，经**同一个**判定点落 canceled。
// 单独断这条是为钉住「判定点在共享失败拼装里」——只在 Do 出口断言过，共享处的回归
// （例如有人把判定挪回各出口、或给 cause 断链）不会被发现。
func TestExecuteSingleProviderAttempt_CanceledDuringBodyRead(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	_, makeInput := newStatsFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush() // 头先到：把失败点逼进读体阶段
		close(started)
		<-release
	}))

	input := makeInput("canceled-during-read")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input.Ctx = ctx

	runAttemptUntilCallerAbandons(t, input, cancel, started, release)

	var row models.ChatLog
	if err := models.DB.Where("provider_name = ?", "p1").First(&row).Error; err != nil {
		t.Fatalf("reload chat log: %v", err)
	}
	if row.Status != models.ChatLogStatusCanceled {
		t.Fatalf("状态 = %q, want canceled（读体途中调用方撤走，与 Do 失败同判）", row.Status)
	}
}
