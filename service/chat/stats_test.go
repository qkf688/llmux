package chat

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/atopos31/llmio/models"
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

	var modelTotal models.StatsModelTotal
	if err := models.DB.First(&modelTotal, "name = ?", "m1").Error; err != nil {
		t.Fatalf("load stats model total: %v", err)
	}
	if modelTotal.Calls != 1 {
		t.Fatalf("modelTotal.calls = %d, want 1", modelTotal.Calls)
	}

	var realModelTotal models.StatsRealModelTotal
	if err := models.DB.First(&realModelTotal, "name = ?", "m1").Error; err != nil {
		t.Fatalf("load stats real model total: %v", err)
	}
	if realModelTotal.Calls != 1 {
		t.Fatalf("realModelTotal.calls = %d, want 1", realModelTotal.Calls)
	}

	proc := func(ctx context.Context, pr io.Reader, stream bool, start time.Time, disablePerformanceTracking bool, disableTokenCounting bool) (*models.ChatLog, *models.OutputUnion, error) {
		_, _ = io.ReadAll(pr)
		return &models.ChatLog{Usage: models.Usage{TotalTokens: 123}}, &models.OutputUnion{OfString: `{"ok":true}`}, nil
	}

	RecordLog(context.Background(), time.Now(), io.NopCloser(strings.NewReader("x")), proc, 0, Before{Stream: false, raw: []byte(`{}`)}, false, "p1")

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

func TestRecordRequestStatsAndTokenStats_UpdatesHourly(t *testing.T) {
	initChatRecordTestDB(t)

	at := time.Date(2026, 1, 2, 15, 4, 5, 0, time.Now().Location())
	date := at.Format("2006-01-02")
	hour := at.Hour()

	if err := recordRequestStats(context.Background(), at, ""); err != nil {
		t.Fatalf("recordRequestStats: %v", err)
	}
	if err := recordRequestStats(context.Background(), at, ""); err != nil {
		t.Fatalf("recordRequestStats again: %v", err)
	}
	if err := recordTokenStats(context.Background(), at, 100); err != nil {
		t.Fatalf("recordTokenStats: %v", err)
	}
	if err := recordTokenStats(context.Background(), at, 23); err != nil {
		t.Fatalf("recordTokenStats again: %v", err)
	}

	if err := recordRealModelRequestStats(context.Background(), at, "rm1"); err != nil {
		t.Fatalf("recordRealModelRequestStats: %v", err)
	}
	if err := recordRealModelRequestStats(context.Background(), at, "rm1"); err != nil {
		t.Fatalf("recordRealModelRequestStats again: %v", err)
	}

	var realModelTotal models.StatsRealModelTotal
	if err := models.DB.First(&realModelTotal, "name = ?", "rm1").Error; err != nil {
		t.Fatalf("load stats real model total: %v", err)
	}
	if realModelTotal.Calls != 2 {
		t.Fatalf("realModelTotal.calls=%d, want 2", realModelTotal.Calls)
	}

	var hourly models.StatsHourly
	if err := models.DB.First(&hourly, "date = ? AND hour = ?", date, hour).Error; err != nil {
		t.Fatalf("load stats hourly: %v", err)
	}
	if hourly.Reqs != 2 {
		t.Fatalf("hourly.reqs=%d, want 2", hourly.Reqs)
	}
	if hourly.Tokens != 123 {
		t.Fatalf("hourly.tokens=%d, want 123", hourly.Tokens)
	}
}
