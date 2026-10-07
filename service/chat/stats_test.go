package chat

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
)

func TestDisableAllLogs_StillUpdatesStats(t *testing.T) {
	initChatRecordTestDB(t)

	if err := models.DB.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyDisableAllLogs).
		Update("value", "true").Error; err != nil {
		t.Fatalf("enable disable_all_logs: %v", err)
	}

	logID, err := SaveChatLog(context.Background(), models.ChatLog{
		Name:          "m1",
		ProviderModel: "pm1",
		ProviderName:  "p1",
		Status:        "success",
	})
	if err != nil {
		t.Fatalf("SaveChatLog: %v", err)
	}
	if logID != 0 {
		t.Fatalf("logID = %d, want 0 when disable_all_logs=true", logID)
	}

	var logsCount int64
	if err := models.DB.Model(&models.ChatLog{}).Count(&logsCount).Error; err != nil {
		t.Fatalf("count chat logs: %v", err)
	}
	if logsCount != 0 {
		t.Fatalf("chat logs count = %d, want 0", logsCount)
	}

	today := time.Now().Format("2006-01-02")

	var daily models.StatsDaily
	if err := models.DB.First(&daily, "date = ?", today).Error; err != nil {
		t.Fatalf("load stats daily: %v", err)
	}
	if daily.Reqs != 1 {
		t.Fatalf("daily.reqs = %d, want 1", daily.Reqs)
	}
	if daily.Tokens != 0 {
		t.Fatalf("daily.tokens = %d, want 0 before RecordLog", daily.Tokens)
	}

	var realModelTotal models.StatsRealModelTotal
	if err := models.DB.First(&realModelTotal, "name = ?", "m1").Error; err != nil {
		t.Fatalf("load stats real model total: %v", err)
	}
	if realModelTotal.Calls != 1 {
		t.Fatalf("realModelTotal.calls = %d, want 1", realModelTotal.Calls)
	}

	proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error) {
		_, _ = io.ReadAll(pr)
		return &models.ChatLog{Usage: models.Usage{TotalTokens: 123}}, &models.OutputUnion{OfString: `{"ok":true}`}, nil
	}

	RecordLog(context.Background(), RecordLogInput{
		ReqStart:      time.Now(),
		Reader:        io.NopCloser(strings.NewReader("x")),
		Processer:     proc,
		Before:        Before{Model: "m1", Stream: false, raw: []byte(`{}`)},
		ProviderName:  "p1",
		RealModelName: "m1",
	})

	var afterDaily models.StatsDaily
	if err := models.DB.First(&afterDaily, "date = ?", today).Error; err != nil {
		t.Fatalf("load stats daily after RecordLog: %v", err)
	}
	if afterDaily.Reqs != 1 {
		t.Fatalf("afterDaily.reqs = %d, want 1", afterDaily.Reqs)
	}
	if afterDaily.Tokens != 123 {
		t.Fatalf("afterDaily.tokens = %d, want 123", afterDaily.Tokens)
	}

	// 真实模型维度的 token 与 calls 落同一行：建行时刻记 calls、结算时刻补
	// token，两者必须能对上账。断言点在 LogID==0 早退之后仍成立——
	// 统计不随「关日志」关闭，只关日志会让计量跟着哑掉。
	var afterRealModel models.StatsRealModelTotal
	if err := models.DB.First(&afterRealModel, "name = ?", "m1").Error; err != nil {
		t.Fatalf("load stats real model total after RecordLog: %v", err)
	}
	if afterRealModel.Calls != 1 || afterRealModel.TotalTokens != 123 {
		t.Fatalf("afterRealModel = %d calls / %d tokens, want 1/123",
			afterRealModel.Calls, afterRealModel.TotalTokens)
	}

	var total models.StatsTotal
	if err := models.DB.First(&total, "id = ?", 1).Error; err != nil {
		t.Fatalf("load stats total: %v", err)
	}
	if total.Reqs != 1 {
		t.Fatalf("total.reqs = %d, want 1", total.Reqs)
	}
	if total.Tokens != 123 {
		t.Fatalf("total.tokens = %d, want 123", total.Tokens)
	}

	var ioCount int64
	if err := models.DB.Model(&models.ChatIO{}).Count(&ioCount).Error; err != nil {
		t.Fatalf("count chat io: %v", err)
	}
	if ioCount != 0 {
		t.Fatalf("chat io count = %d, want 0", ioCount)
	}
}

// TestRealModelTokenStats_KeyFallsBackToRequestedModel 钉住统计键的回落口径：
// 直连请求没有「虚拟→真实」的转换，RealModelName 为空，此时必须回落到用户
// 请求的模型名。回落若失效，直连流量会静默不记 token——而且是在真实模型
// 直连占了大部分流量时，看板上只少几行数字，不会有人发现。
func TestRealModelTokenStats_KeyFallsBackToRequestedModel(t *testing.T) {
	cases := []struct {
		name         string
		realModel    string
		requested    string
		wantStatsKey string
	}{
		{name: "虚拟路径用真实命中名", realModel: "real-x", requested: "virtual-y", wantStatsKey: "real-x"},
		{name: "直连回落请求名", realModel: "", requested: "direct-z", wantStatsKey: "direct-z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initChatRecordTestDB(t)

			if _, err := SaveChatLog(context.Background(), models.ChatLog{
				Name:          tc.requested,
				RealModelName: tc.realModel,
				ProviderModel: "pm",
				ProviderName:  "p",
				Status:        "success",
			}); err != nil {
				t.Fatalf("SaveChatLog: %v", err)
			}

			proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time) (*models.ChatLog, *models.OutputUnion, error) {
				_, _ = io.ReadAll(pr)
				return &models.ChatLog{Usage: models.Usage{TotalTokens: 7}}, &models.OutputUnion{}, nil
			}
			RecordLog(context.Background(), RecordLogInput{
				ReqStart:      time.Now(),
				Reader:        io.NopCloser(strings.NewReader("x")),
				Processer:     proc,
				Before:        Before{Model: tc.requested, Stream: false, raw: []byte(`{}`)},
				ProviderName:  "p",
				RealModelName: tc.realModel,
			})

			// calls 与 token 必须落在同一主键上：分家了就说明两处口径没同源。
			var row models.StatsRealModelTotal
			if err := models.DB.First(&row, "name = ?", tc.wantStatsKey).Error; err != nil {
				t.Fatalf("load stats row %q: %v", tc.wantStatsKey, err)
			}
			if row.Calls != 1 || row.TotalTokens != 7 {
				t.Fatalf("row = %d calls / %d tokens, want 1/7", row.Calls, row.TotalTokens)
			}
		})
	}
}
